package broadcast

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/sadegh/gows/internal/apps"
	"github.com/sadegh/gows/internal/logging"
	"github.com/sadegh/gows/internal/stats"
)

type Payload struct {
	UserID  string          `json:"user_id,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	stats.Server.BroadcastRequests.Add(1)
	if r.Method != http.MethodPost {
		stats.Server.BroadcastFailed.Add(1)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	secret := strings.TrimPrefix(authHeader, "Bearer ")

	matchedApp, matchedHub, matchedStats := apps.FindBySecret(secret)
	if matchedApp == nil || matchedHub == nil {
		stats.Server.BroadcastUnauthorized.Add(1)
		stats.Server.BroadcastFailed.Add(1)
		logging.Info("Unauthorized broadcast request from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if matchedStats == nil {
		matchedStats = apps.EnsureStats(matchedApp.AppID)
	}
	matchedStats.BroadcastRequests.Add(1)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		stats.Server.BroadcastFailed.Add(1)
		matchedStats.BroadcastFailed.Add(1)
		logging.Info("Could not read broadcast body: %v", err)
		http.Error(w, "Could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		stats.Server.BroadcastFailed.Add(1)
		matchedStats.BroadcastFailed.Add(1)
		logging.Info("Invalid broadcast JSON: %v", err)
		logging.Debug("Invalid broadcast body: %s", string(body))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Channel != "" {
		logging.Info("[App: %s] Broadcasting event '%s' to channel: %s", matchedApp.AppID, payload.Event, payload.Channel)
		matchedHub.BroadcastToChannel(payload.Channel, body)
	} else if payload.UserID != "" {
		logging.Info("[App: %s] Broadcasting event '%s' to user: %s", matchedApp.AppID, payload.Event, payload.UserID)
		matchedHub.BroadcastToUser(payload.UserID, body)
	} else {
		stats.Server.BroadcastFailed.Add(1)
		matchedStats.BroadcastFailed.Add(1)
		logging.Info("[App: %s] Failed broadcast: Missing user_id or channel", matchedApp.AppID)
		http.Error(w, "Missing user_id or channel", http.StatusBadRequest)
		return
	}

	stats.Server.BroadcastSent.Add(1)
	matchedStats.BroadcastSent.Add(1)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"sent"}`))
}
