package admin

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sadegh/gows/internal/apps"
	"github.com/sadegh/gows/internal/config"
	"github.com/sadegh/gows/internal/stats"
)

//go:embed templates/*.html
var adminTemplateFS embed.FS

var adminTemplates = template.Must(template.ParseFS(adminTemplateFS, "templates/*.html"))

type DashboardData struct {
	Mode                      string
	StartedAt                 string
	Uptime                    string
	Apps                      []DashboardApp
	TotalConnections          int
	TotalUsers                int
	TotalChannels             int
	TotalBroadcastRequests    uint64
	TotalBroadcastSent        uint64
	TotalBroadcastFailed      uint64
	WSAttempts                uint64
	WSAccepted                uint64
	WSRejected                uint64
	BroadcastRequests         uint64
	BroadcastUnauthorized     uint64
	BroadcastSent             uint64
	BroadcastFailed           uint64
	ValidationRequests        uint64
	ValidationFailures        uint64
	AdminUnauthorizedViews    uint64
	IsAdminPasswordConfigured bool
}

type DashboardApp struct {
	Name        string
	AppID       string
	TicketURL   string
	Users       int
	Connections int
	Channels    int
	Broadcasts  apps.AppStatsSnapshot
}

type AppFormData struct {
	Mode                      string
	Title                     string
	IsEdit                    bool
	App                       config.AppConfig
	Message                   string
	Error                     string
	IsAdminPasswordConfigured bool
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdminAuth(w, r) {
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTemplates.ExecuteTemplate(w, "admin.html", dashboardSnapshot()); err != nil {
		log.Printf("Could not render admin dashboard: %v", err)
	}
}

func isAdminAuthorized(r *http.Request) bool {
	if config.AdminPassword() == "" {
		return false
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(config.AdminUsername())) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(config.AdminPassword())) == 1
	return userOK && passOK
}

func requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	if !isAdminAuthorized(r) {
		stats.Server.AdminUnauthorizedViews.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="GoWS Admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func AppNewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireAdminAuth(w, r) {
		return
	}

	data := AppFormData{
		Mode:                      config.Mode(),
		Title:                     "Add app",
		IsEdit:                    false,
		IsAdminPasswordConfigured: config.AdminPasswordConfigured(),
	}

	if r.Method == http.MethodGet {
		renderAppForm(w, data)
		return
	}

	if err := r.ParseForm(); err != nil {
		data.Error = "Could not read form data."
		renderAppForm(w, data)
		return
	}

	secret, err := apps.GenerateSecret()
	if err != nil {
		data.Error = "Could not generate secret."
		renderAppForm(w, data)
		return
	}

	app := config.AppConfig{
		Name:      r.FormValue("name"),
		AppID:     r.FormValue("app_id"),
		TicketURL: r.FormValue("ticket_url"),
		Secret:    secret,
	}

	if err := apps.Add(app); err != nil {
		data.Error = err.Error()
		data.App = app
		renderAppForm(w, data)
		return
	}

	if err := config.Persist(apps.ListConfigs()); err != nil {
		_ = apps.Delete(app.AppID)
		data.Error = "Failed to save config: " + err.Error()
		data.App = app
		renderAppForm(w, data)
		return
	}

	http.Redirect(w, r, "/admin/apps/"+template.URLQueryEscaper(app.AppID)+"/edit?created=1", http.StatusSeeOther)
}

func AppHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdminAuth(w, r) {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/apps/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	appID := parts[0]
	action := parts[1]
	if !apps.IsValidAppID(appID) {
		http.Error(w, "Invalid app id", http.StatusBadRequest)
		return
	}

	switch action {
	case "edit":
		adminAppEditHandler(w, r, appID)
	case "delete":
		adminAppDeleteHandler(w, r, appID)
	default:
		http.NotFound(w, r)
	}
}

func adminAppEditHandler(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app, _, ok := apps.GetByID(appID)
	if !ok || app == nil {
		http.NotFound(w, r)
		return
	}

	data := AppFormData{
		Mode:                      config.Mode(),
		Title:                     "Edit app",
		IsEdit:                    true,
		App:                       *app,
		IsAdminPasswordConfigured: config.AdminPasswordConfigured(),
	}

	if r.Method == http.MethodGet {
		if r.URL.Query().Get("created") == "1" {
			data.Message = "App created. Copy the secret now."
		}
		if r.URL.Query().Get("updated") == "1" {
			data.Message = "App updated."
		}
		renderAppForm(w, data)
		return
	}

	if err := r.ParseForm(); err != nil {
		data.Error = "Could not read form data."
		renderAppForm(w, data)
		return
	}

	name := r.FormValue("name")
	ticketURL := r.FormValue("ticket_url")
	previous := data.App

	if err := apps.Update(appID, name, ticketURL); err != nil {
		data.Error = err.Error()
		data.App.Name = name
		data.App.TicketURL = ticketURL
		renderAppForm(w, data)
		return
	}

	if err := config.Persist(apps.ListConfigs()); err != nil {
		_ = apps.Update(previous.AppID, previous.Name, previous.TicketURL)
		data.Error = "Failed to save config: " + err.Error()
		renderAppForm(w, data)
		return
	}

	http.Redirect(w, r, "/admin/apps/"+template.URLQueryEscaper(appID)+"/edit?updated=1", http.StatusSeeOther)
}

func adminAppDeleteHandler(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app, _, ok := apps.GetByID(appID)
	if !ok || app == nil {
		http.NotFound(w, r)
		return
	}
	appCopy := *app

	if err := apps.Delete(appID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := config.Persist(apps.ListConfigs()); err != nil {
		_ = apps.Add(appCopy)
		_ = config.Persist(apps.ListConfigs())
		http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func renderAppForm(w http.ResponseWriter, data AppFormData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTemplates.ExecuteTemplate(w, "app_form.html", data); err != nil {
		log.Printf("Could not render app form: %v", err)
	}
}

func dashboardSnapshot() DashboardData {
	data := DashboardData{
		Mode:                      config.Mode(),
		StartedAt:                 stats.Server.StartedAt.Format(time.RFC3339),
		Uptime:                    time.Since(stats.Server.StartedAt).Round(time.Second).String(),
		WSAttempts:                stats.Server.WSAttempts.Load(),
		WSAccepted:                stats.Server.WSAccepted.Load(),
		WSRejected:                stats.Server.WSRejected.Load(),
		BroadcastRequests:         stats.Server.BroadcastRequests.Load(),
		BroadcastUnauthorized:     stats.Server.BroadcastUnauthorized.Load(),
		BroadcastSent:             stats.Server.BroadcastSent.Load(),
		BroadcastFailed:           stats.Server.BroadcastFailed.Load(),
		ValidationRequests:        stats.Server.ValidationRequests.Load(),
		ValidationFailures:        stats.Server.ValidationFailures.Load(),
		AdminUnauthorizedViews:    stats.Server.AdminUnauthorizedViews.Load(),
		IsAdminPasswordConfigured: config.AdminPasswordConfigured(),
	}

	for _, snapshot := range apps.SnapshotAll() {
		data.Apps = append(data.Apps, DashboardApp{
			Name:        snapshot.App.Name,
			AppID:       snapshot.App.AppID,
			TicketURL:   snapshot.App.TicketURL,
			Users:       snapshot.Users,
			Connections: snapshot.Connections,
			Channels:    snapshot.Channels,
			Broadcasts:  snapshot.Broadcasts,
		})
		data.TotalUsers += snapshot.Users
		data.TotalConnections += snapshot.Connections
		data.TotalChannels += snapshot.Channels
		data.TotalBroadcastRequests += snapshot.Broadcasts.BroadcastRequests
		data.TotalBroadcastSent += snapshot.Broadcasts.BroadcastSent
		data.TotalBroadcastFailed += snapshot.Broadcasts.BroadcastFailed
	}

	return data
}
