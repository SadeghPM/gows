package broadcast

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sadegh/gows/internal/apps"
	"github.com/sadegh/gows/internal/config"
	"github.com/sadegh/gows/internal/stats"
)

func resetBroadcastState() {
	apps.ResetForTest()
	stats.Server = stats.ServerStats{}
	config.ResetForTest()
}

func TestHandler_Unauthorized(t *testing.T) {
	resetBroadcastState()
	req, err := http.NewRequest("POST", "/api/internal/broadcast", bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	Handler(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestHandler_Authorized(t *testing.T) {
	resetBroadcastState()
	err := apps.Add(config.AppConfig{
		AppID:     "test-app",
		Name:      "Test App",
		TicketURL: "http://example.com/api/internal/ws/validate-ticket",
		Secret:    "test-secret",
	})
	if err != nil {
		t.Fatalf("failed to add app: %v", err)
	}

	payload := []byte(`{"channel": "news", "event": "test", "payload": {}}`)
	req, err := http.NewRequest("POST", "/api/internal/broadcast", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-secret")

	rr := httptest.NewRecorder()
	Handler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
