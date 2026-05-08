package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

var apps = make(map[string]*AppConfig)
var hubs = make(map[string]*Hub)

const (
	writeWait    = 10 * time.Second
	pongWait     = 60 * time.Second
	pingInterval = (pongWait * 9) / 10 // 54 seconds
)

func main() {
	initConfig()

	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/api/internal/broadcast", broadcastHandler)
	http.HandleFunc("/up", healthHandler)
	http.HandleFunc("/admin", adminDashboardHandler)
	http.HandleFunc("/admin/apps/new", adminAppNewHandler)
	http.HandleFunc("/admin/apps/", adminAppHandler)

	port := ":" + serverPort
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

type ClientMessage struct {
	Action  string `json:"action"`
	Channel string `json:"channel"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	stats.wsAttempts.Add(1)
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		appID = "default"
	}

	debugLog("[WS] Connection attempt - app_id: %s", appID)

	app, hub, ok := getAppByID(appID)
	if !ok || app == nil || hub == nil {
		stats.wsRejected.Add(1)
		infoLog("[WS] App not found: %s", appID)
		debugLog("[WS] Available apps: %v", getAppIDs())
		http.Error(w, "Invalid app_id", http.StatusUnauthorized)
		return
	}

	debugLog("[WS] App config - ticket_url: %s", app.TicketURL)

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		stats.wsRejected.Add(1)
		infoLog("[WS] Missing ticket for app_id: %s", appID)
		http.Error(w, "Missing ticket", http.StatusUnauthorized)
		return
	}

	debugLog("[WS] Validating ticket: %s", ticket)
	userID, err := validateTicketWithLaravel(ticket, app)
	if err != nil || userID == "" {
		stats.wsRejected.Add(1)
		infoLog("[WS] Validation failed for app_id: %s", appID)
		debugLog("[WS] Validation failure details - err: %v, userID: %s", err, userID)
		http.Error(w, "Invalid ticket", http.StatusUnauthorized)
		return
	}

	stats.wsAccepted.Add(1)
	debugLog("[WS] Validation success - user_id: %s", userID)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	hub.Register(userID, conn)
	defer hub.Unregister(userID, conn)

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	// Goroutine to send periodic pings
	go func() {
		for range ticker.C {
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}()

	// Keep alive & Client Message pump
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				debugLog("Unexpected close error: %v", err)
			}
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err == nil {
			if clientMsg.Action == "subscribe" && clientMsg.Channel != "" {
				hub.Subscribe(conn, clientMsg.Channel)
			} else if clientMsg.Action == "unsubscribe" && clientMsg.Channel != "" {
				hub.Unsubscribe(conn, clientMsg.Channel)
			}
		}
	}
}

func validateTicketWithLaravel(ticket string, app *AppConfig) (string, error) {
	stats.validationRequests.Add(1)
	reqBody, _ := json.Marshal(map[string]string{"ticket": ticket})
	req, err := http.NewRequest("POST", app.TicketURL, bytes.NewBuffer(reqBody))
	if err != nil {
		stats.validationFailures.Add(1)
		infoLog("[VALIDATION] Request creation error: %v", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+app.Secret)

	debugLog("[VALIDATION] Requesting: %s with ticket: %s", app.TicketURL, ticket)
	debugLog("[VALIDATION] Secret being used: %s", app.Secret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		stats.validationFailures.Add(1)
		infoLog("[VALIDATION] Request error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	debugLog("[VALIDATION] Response status: %d", resp.StatusCode)
	debugLog("[VALIDATION] Response body: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		stats.validationFailures.Add(1)
		debugLog("[VALIDATION] Invalid ticket - status not OK")
		return "", nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		stats.validationFailures.Add(1)
		infoLog("[VALIDATION] Failed to parse response: %v", err)
		return "", err
	}

	if userID, ok := result["user_id"].(string); ok {
		debugLog("[VALIDATION] Got user_id (string): %s", userID)
		return userID, nil
	}
	if userIDFloat, ok := result["user_id"].(float64); ok {
		userID := strconv.FormatFloat(userIDFloat, 'f', -1, 64)
		debugLog("[VALIDATION] Got user_id (float): %s", userID)
		return userID, nil
	}

	stats.validationFailures.Add(1)
	debugLog("[VALIDATION] No user_id in response")
	return "", nil
}

type BroadcastPayload struct {
	UserID  string          `json:"user_id,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

func broadcastHandler(w http.ResponseWriter, r *http.Request) {
	stats.broadcastRequests.Add(1)
	if r.Method != http.MethodPost {
		stats.broadcastFailed.Add(1)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	secret := strings.TrimPrefix(authHeader, "Bearer ")

	matchedApp, matchedHub, matchedStats := findAppBySecret(secret)
	if matchedApp == nil || matchedHub == nil {
		stats.broadcastUnauthorized.Add(1)
		stats.broadcastFailed.Add(1)
		infoLog("Unauthorized broadcast request from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if matchedStats == nil {
		appsMu.Lock()
		matchedStats = ensureAppStatsLocked(matchedApp.AppID)
		appsMu.Unlock()
	}
	matchedStats.broadcastRequests.Add(1)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		stats.broadcastFailed.Add(1)
		matchedStats.broadcastFailed.Add(1)
		infoLog("Could not read broadcast body: %v", err)
		http.Error(w, "Could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload BroadcastPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		stats.broadcastFailed.Add(1)
		matchedStats.broadcastFailed.Add(1)
		infoLog("Invalid broadcast JSON: %v", err)
		debugLog("Invalid broadcast body: %s", string(body))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Channel != "" {
		infoLog("[App: %s] Broadcasting event '%s' to channel: %s", matchedApp.AppID, payload.Event, payload.Channel)
		matchedHub.BroadcastToChannel(payload.Channel, body)
	} else if payload.UserID != "" {
		infoLog("[App: %s] Broadcasting event '%s' to user: %s", matchedApp.AppID, payload.Event, payload.UserID)
		matchedHub.BroadcastToUser(payload.UserID, body)
	} else {
		stats.broadcastFailed.Add(1)
		matchedStats.broadcastFailed.Add(1)
		infoLog("[App: %s] Failed broadcast: Missing user_id or channel", matchedApp.AppID)
		http.Error(w, "Missing user_id or channel", http.StatusBadRequest)
		return
	}

	stats.broadcastSent.Add(1)
	matchedStats.broadcastSent.Add(1)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"sent"}`))
}
