package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sadegh/gows/internal/config"
	"github.com/sadegh/gows/internal/stats"
)

func resetWSState() {
	stats.Server = stats.ServerStats{}
}

func TestValidateTicketWithLaravel_Success(t *testing.T) {
	resetWSState()
	laravelMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user_id": "123"}`))
	}))
	defer laravelMock.Close()

	app := &config.AppConfig{
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
	resetWSState()
	laravelMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer laravelMock.Close()

	app := &config.AppConfig{
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
