package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func resetTestState() {
	apps = make(map[string]*AppConfig)
	hubs = make(map[string]*Hub)
	serverPort = "8080"
	serverMode = "production"
	adminUsername = "admin"
	adminPassword = ""
	stats = ServerStats{}
}

func TestBroadcastHandler_Unauthorized(t *testing.T) {
	resetTestState()
	req, err := http.NewRequest("POST", "/api/internal/broadcast", bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(broadcastHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestBroadcastHandler_Authorized(t *testing.T) {
	resetTestState()
	// Setup test apps and hubs
	apps["test-app"] = &AppConfig{
		AppID:  "test-app",
		Secret: "test-secret",
	}
	hubs["test-app"] = NewHub()

	payload := []byte(`{"channel": "news", "event": "test", "payload": {}}`)
	req, err := http.NewRequest("POST", "/api/internal/broadcast", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-secret")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(broadcastHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestValidateTicketWithLaravel_Success(t *testing.T) {
	resetTestState()
	// Mock Laravel server
	laravelMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user_id": "123"}`))
	}))
	defer laravelMock.Close()

	app := &AppConfig{
		AppID:     "test-app",
		TicketURL: laravelMock.URL,
		Secret:    "test-secret",
	}

	userID, err := validateTicketWithLaravel("valid-ticket", app)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if userID != "123" {
		t.Errorf("Expected userID 123, got %s", userID)
	}
}

func TestValidateTicketWithLaravel_InvalidTicket(t *testing.T) {
	resetTestState()
	// Mock Laravel server
	laravelMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer laravelMock.Close()

	app := &AppConfig{
		AppID:     "test-app",
		TicketURL: laravelMock.URL,
		Secret:    "test-secret",
	}

	userID, err := validateTicketWithLaravel("invalid", app)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if userID != "" {
		t.Errorf("Expected empty userID, got %s", userID)
	}
}

func TestApplyServerSettings_ModeAndAdmin(t *testing.T) {
	resetTestState()

	applyServerSettings(ServerConfig{
		Mode:          "debug",
		AdminUsername: "root",
		AdminPassword: "secret",
	})

	if serverMode != "debug" {
		t.Fatalf("expected debug mode, got %s", serverMode)
	}
	if adminUsername != "root" {
		t.Fatalf("expected admin username root, got %s", adminUsername)
	}
	if adminPassword != "secret" {
		t.Fatal("expected admin password to be set")
	}
}

func TestAdminDashboard_RequiresAuth(t *testing.T) {
	resetTestState()
	adminPassword = "secret"

	req, err := http.NewRequest("GET", "/admin", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	adminDashboardHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", rr.Code)
	}
}

func TestAdminDashboard_Authorized(t *testing.T) {
	resetTestState()
	adminPassword = "secret"
	hubs["default"] = NewHub()

	req, err := http.NewRequest("GET", "/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("admin", "secret")

	rr := httptest.NewRecorder()
	adminDashboardHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok status, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("GoWS Admin")) {
		t.Fatal("expected dashboard HTML")
	}
}
