package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v2"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

type AppConfig struct {
	AppID     string `json:"app_id" yaml:"id"`
	TicketURL string `json:"ticket_url" yaml:"ticket_url"`
	Secret    string `json:"secret" yaml:"secret"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type YAMLConfig struct {
	Server ServerConfig `yaml:"server"`
	Apps   []AppConfig  `yaml:"apps"`
}

var apps = make(map[string]*AppConfig)
var hubs = make(map[string]*Hub)
var serverPort = "8080"

const (
	writeWait    = 10 * time.Second
	pongWait     = 60 * time.Second
	pingInterval = (pongWait * 9) / 10 // 54 seconds
)

func initConfig() {
	// Try reading config.yaml
	file, err := os.ReadFile("config.yaml")
	if err == nil {
		var config YAMLConfig
		if err := yaml.Unmarshal(file, &config); err == nil {
			// Load server config
			if config.Server.Port != "" {
				serverPort = config.Server.Port
			}

			// Load apps
			for _, app := range config.Apps {
				a := app // copy
				apps[a.AppID] = &a
				hubs[a.AppID] = NewHub()
			}
			log.Printf("Loaded %d apps from config.yaml (server port: %s)", len(apps), serverPort)
			return
		} else {
			log.Printf("Failed to parse config.yaml: %v. Falling back to env defaults.", err)
		}
	}

	// Fallback to environment variables
	if port := os.Getenv("PORT"); port != "" {
		serverPort = port
	}

	app := &AppConfig{
		AppID:     os.Getenv("APP_ID"),
		TicketURL: os.Getenv("LARAVEL_TICKET_URL"),
		Secret:    os.Getenv("INTERNAL_SECRET"),
	}
	if app.AppID == "" {
		app.AppID = "default"
	}
	if app.TicketURL == "" {
		app.TicketURL = "http://localhost:8000/api/internal/ws/validate-ticket"
	}
	if app.Secret == "" {
		app.Secret = "super-secret-internal-key"
	}
	apps[app.AppID] = app
	hubs[app.AppID] = NewHub()
	log.Printf("Loaded single-tenant fallback from env vars (AppID: %s)", app.AppID)
}

func main() {
	initConfig()

	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/api/internal/broadcast", broadcastHandler)
	http.HandleFunc("/up", healthHandler)

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
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		appID = "default"
	}

	log.Printf("[WS] Connection attempt - app_id: %s", appID)

	app, ok := apps[appID]
	if !ok {
		log.Printf("[WS] App not found: %s (available: %v)", appID, getAppIDs())
		http.Error(w, "Invalid app_id", http.StatusUnauthorized)
		return
	}
	hub := hubs[appID]

	log.Printf("[WS] App config - ticket_url: %s", app.TicketURL)

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		log.Printf("[WS] Missing ticket")
		http.Error(w, "Missing ticket", http.StatusUnauthorized)
		return
	}

	log.Printf("[WS] Validating ticket: %s", ticket)
	userID, err := validateTicketWithLaravel(ticket, app)
	if err != nil || userID == "" {
		log.Printf("[WS] Validation failed - err: %v, userID: %s", err, userID)
		http.Error(w, "Invalid ticket", http.StatusUnauthorized)
		return
	}

	log.Printf("[WS] Validation success - user_id: %s", userID)

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
				log.Printf("Unexpected close error: %v", err)
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

func getAppIDs() []string {
	keys := make([]string, 0, len(apps))
	for k := range apps {
		keys = append(keys, k)
	}
	return keys
}

func validateTicketWithLaravel(ticket string, app *AppConfig) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{"ticket": ticket})
	req, err := http.NewRequest("POST", app.TicketURL, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("[VALIDATION] Request creation error: %v", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+app.Secret)

	log.Printf("[VALIDATION] Requesting: %s with ticket: %s", app.TicketURL, ticket)
	log.Printf("[VALIDATION] Secret being used: %s", app.Secret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[VALIDATION] Request error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[VALIDATION] Response status: %d", resp.StatusCode)
	log.Printf("[VALIDATION] Response body: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		log.Printf("[VALIDATION] Invalid ticket - status not OK")
		return "", nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Printf("[VALIDATION] Failed to parse response: %v", err)
		return "", err
	}

	if userID, ok := result["user_id"].(string); ok {
		log.Printf("[VALIDATION] Got user_id (string): %s", userID)
		return userID, nil
	}
	if userIDFloat, ok := result["user_id"].(float64); ok {
		userID := strconv.FormatFloat(userIDFloat, 'f', -1, 64)
		log.Printf("[VALIDATION] Got user_id (float): %s", userID)
		return userID, nil
	}

	log.Printf("[VALIDATION] No user_id in response")
	return "", nil
}

type BroadcastPayload struct {
	UserID  string          `json:"user_id,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

func broadcastHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	secret := strings.TrimPrefix(authHeader, "Bearer ")

	var matchedApp *AppConfig
	var matchedHub *Hub
	for _, app := range apps {
		if app.Secret == secret {
			matchedApp = app
			matchedHub = hubs[app.AppID]
			break
		}
	}

	if matchedApp == nil {
		log.Printf("Unauthorized broadcast request from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Could not read broadcast body: %v", err)
		http.Error(w, "Could not read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload BroadcastPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Invalid broadcast JSON: %v. Body: %s", err, string(body))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Channel != "" {
		log.Printf("[App: %s] Broadcasting event '%s' to channel: %s", matchedApp.AppID, payload.Event, payload.Channel)
		matchedHub.BroadcastToChannel(payload.Channel, body)
	} else if payload.UserID != "" {
		log.Printf("[App: %s] Broadcasting event '%s' to user: %s", matchedApp.AppID, payload.Event, payload.UserID)
		matchedHub.BroadcastToUser(payload.UserID, body)
	} else {
		log.Printf("[App: %s] Failed broadcast: Missing user_id or channel", matchedApp.AppID)
		http.Error(w, "Missing user_id or channel", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"sent"}`))
}
