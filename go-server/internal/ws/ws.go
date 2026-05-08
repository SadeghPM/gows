package ws

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sadegh/gows/internal/apps"
	"github.com/sadegh/gows/internal/config"
	"github.com/sadegh/gows/internal/logging"
	"github.com/sadegh/gows/internal/stats"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	writeWait    = 10 * time.Second
	pongWait     = 60 * time.Second
	pingInterval = (pongWait * 9) / 10
)

type ClientMessage struct {
	Action  string `json:"action"`
	Channel string `json:"channel"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	stats.Server.WSAttempts.Add(1)
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		appID = "default"
	}

	logging.Debug("[WS] Connection attempt - app_id: %s", appID)

	app, hub, ok := apps.GetByID(appID)
	if !ok || app == nil || hub == nil {
		stats.Server.WSRejected.Add(1)
		logging.Info("[WS] App not found: %s", appID)
		logging.Debug("[WS] Available apps: %v", apps.GetIDs())
		http.Error(w, "Invalid app_id", http.StatusUnauthorized)
		return
	}

	logging.Debug("[WS] App config - ticket_url: %s", app.TicketURL)

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		stats.Server.WSRejected.Add(1)
		logging.Info("[WS] Missing ticket for app_id: %s", appID)
		http.Error(w, "Missing ticket", http.StatusUnauthorized)
		return
	}

	logging.Debug("[WS] Validating ticket: %s", ticket)
	userID, err := validateTicketWithLaravel(ticket, app)
	if err != nil || userID == "" {
		stats.Server.WSRejected.Add(1)
		logging.Info("[WS] Validation failed for app_id: %s", appID)
		logging.Debug("[WS] Validation failure details - err: %v, userID: %s", err, userID)
		http.Error(w, "Invalid ticket", http.StatusUnauthorized)
		return
	}

	stats.Server.WSAccepted.Add(1)
	logging.Debug("[WS] Validation success - user_id: %s", userID)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Info("Upgrade error: %v", err)
		return
	}

	hub.Register(userID, conn)
	defer hub.Unregister(userID, conn)

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logging.Debug("Unexpected close error: %v", err)
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

func validateTicketWithLaravel(ticket string, app *config.AppConfig) (string, error) {
	stats.Server.ValidationRequests.Add(1)
	reqBody, _ := json.Marshal(map[string]string{"ticket": ticket})
	req, err := http.NewRequest("POST", app.TicketURL, bytes.NewBuffer(reqBody))
	if err != nil {
		stats.Server.ValidationFailures.Add(1)
		logging.Info("[VALIDATION] Request creation error: %v", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+app.Secret)

	logging.Debug("[VALIDATION] Requesting: %s with ticket: %s", app.TicketURL, ticket)
	logging.Debug("[VALIDATION] Secret being used: %s", app.Secret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		stats.Server.ValidationFailures.Add(1)
		logging.Info("[VALIDATION] Request error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	logging.Debug("[VALIDATION] Response status: %d", resp.StatusCode)
	logging.Debug("[VALIDATION] Response body: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		stats.Server.ValidationFailures.Add(1)
		logging.Debug("[VALIDATION] Invalid ticket - status not OK")
		return "", nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		stats.Server.ValidationFailures.Add(1)
		logging.Info("[VALIDATION] Failed to parse response: %v", err)
		return "", err
	}

	if userID, ok := result["user_id"].(string); ok {
		logging.Debug("[VALIDATION] Got user_id (string): %s", userID)
		return userID, nil
	}
	if userIDFloat, ok := result["user_id"].(float64); ok {
		userID := strconv.FormatFloat(userIDFloat, 'f', -1, 64)
		logging.Debug("[VALIDATION] Got user_id (float): %s", userID)
		return userID, nil
	}

	stats.Server.ValidationFailures.Add(1)
	logging.Debug("[VALIDATION] No user_id in response")
	return "", nil
}
