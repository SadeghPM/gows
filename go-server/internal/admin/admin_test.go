package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sadegh/gows/internal/apps"
	"github.com/sadegh/gows/internal/config"
	"github.com/sadegh/gows/internal/stats"
)

func resetAdminState() {
	apps.ResetForTest()
	stats.Server = stats.ServerStats{}
	config.ResetForTest()
}

func TestDashboard_RequiresAuth(t *testing.T) {
	resetAdminState()
	config.SetAdminCredentials("admin", "secret")

	req, err := http.NewRequest("GET", "/admin", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	DashboardHandler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", rr.Code)
	}
}

func TestDashboard_Authorized(t *testing.T) {
	resetAdminState()
	config.SetAdminCredentials("admin", "secret")
	apps.LoadFromConfig([]config.AppConfig{
		{
			AppID:     "default",
			Name:      "Default",
			TicketURL: "http://example.com/api/internal/ws/validate-ticket",
			Secret:    "secret",
		},
	})

	req, err := http.NewRequest("GET", "/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("admin", "secret")

	rr := httptest.NewRecorder()
	DashboardHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok status, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("GoWS Admin")) {
		t.Fatal("expected dashboard HTML")
	}
}
