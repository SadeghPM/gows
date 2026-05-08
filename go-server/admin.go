package main

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"log"
	"net/http"
	"time"
)

//go:embed templates/admin.html
var adminTemplateFS embed.FS

var dashboardTemplate = template.Must(template.ParseFS(adminTemplateFS, "templates/admin.html"))

type DashboardData struct {
	Mode                      string
	StartedAt                 string
	Uptime                    string
	Apps                      []DashboardApp
	TotalConnections          int
	TotalUsers                int
	TotalChannels             int
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
	AppID       string
	Users       int
	Connections int
	Channels    int
}

func adminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isAdminAuthorized(r) {
		stats.adminUnauthorizedViews.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="GoWS Admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(w, dashboardSnapshot()); err != nil {
		log.Printf("Could not render admin dashboard: %v", err)
	}
}

func isAdminAuthorized(r *http.Request) bool {
	if adminPassword == "" {
		return false
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(adminUsername)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(adminPassword)) == 1
	return userOK && passOK
}

func dashboardSnapshot() DashboardData {
	data := DashboardData{
		Mode:                      serverMode,
		StartedAt:                 stats.startedAt.Format(time.RFC3339),
		Uptime:                    time.Since(stats.startedAt).Round(time.Second).String(),
		WSAttempts:                stats.wsAttempts.Load(),
		WSAccepted:                stats.wsAccepted.Load(),
		WSRejected:                stats.wsRejected.Load(),
		BroadcastRequests:         stats.broadcastRequests.Load(),
		BroadcastUnauthorized:     stats.broadcastUnauthorized.Load(),
		BroadcastSent:             stats.broadcastSent.Load(),
		BroadcastFailed:           stats.broadcastFailed.Load(),
		ValidationRequests:        stats.validationRequests.Load(),
		ValidationFailures:        stats.validationFailures.Load(),
		AdminUnauthorizedViews:    stats.adminUnauthorizedViews.Load(),
		IsAdminPasswordConfigured: adminPassword != "",
	}

	for appID, hub := range hubs {
		snapshot := hub.Snapshot()
		data.Apps = append(data.Apps, DashboardApp{
			AppID:       appID,
			Users:       snapshot.Users,
			Connections: snapshot.Connections,
			Channels:    snapshot.Channels,
		})
		data.TotalUsers += snapshot.Users
		data.TotalConnections += snapshot.Connections
		data.TotalChannels += snapshot.Channels
	}

	return data
}
