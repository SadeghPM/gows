package main

import (
	"log"
	"net/http"

	"github.com/sadegh/gows/internal/admin"
	"github.com/sadegh/gows/internal/apps"
	"github.com/sadegh/gows/internal/broadcast"
	"github.com/sadegh/gows/internal/config"
	"github.com/sadegh/gows/internal/stats"
	"github.com/sadegh/gows/internal/ws"
)

func main() {
	stats.Init()
	appConfigs := config.Load()
	apps.LoadFromConfig(appConfigs)

	http.HandleFunc("/ws", ws.Handler)
	http.HandleFunc("/api/internal/broadcast", broadcast.Handler)
	http.HandleFunc("/up", healthHandler)
	http.HandleFunc("/admin", admin.DashboardHandler)
	http.HandleFunc("/admin/apps/new", admin.AppNewHandler)
	http.HandleFunc("/admin/apps/", admin.AppHandler)

	port := ":" + config.Port()
	log.Printf("Go WebSocket server running on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
