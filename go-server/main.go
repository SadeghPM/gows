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

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

type AppConfig struct {
	AppID     string `json:"app_id"`
	TicketURL string `json:"ticket_url"`
	Secret    string `json:"secret"`
}

var apps = make(map[string]*AppConfig)
var hubs = make(map[string]*Hub)
var serverPort = "8080"

func initConfig() {
	_ = godotenv.Load()
	if port := os.Getenv("PORT"); port != "" {
		serverPort = port
	}

	// Try reading apps.json for multi-tenant setup
	file, err := os.ReadFile("apps.json")
	if err == nil {
		var configApps []AppConfig
		if err := json.Unmarshal(file, &configApps); err == nil {
			for _, app := range configApps {
				a := app // copy
				apps[a.AppID] = &a
				hubs[a.AppID] = NewHub()
			}
			log.Printf("Loaded %d apps from apps.json", len(apps))
			return
		} else {
			log.Printf("Failed to parse apps.json: %v. Falling back to env defaults.", err)
		}
	}

	// Fallback to single-tenant Env vars
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
	log.Printf("Loaded single-tenant fallback (AppID: %s)", app.AppID)
}

func main() {
	initConfig()

	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/api/internal/broadcast", broadcastHandler)

	port := ":" + serverPort
	log.Printf("Go WebSocket server running on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
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

	app, ok := apps[appID]
	if !ok {
		http.Error(w, "Invalid app_id", http.StatusUnauthorized)
		return
	}
	hub := hubs[appID]

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		http.Error(w, "Missing ticket", http.StatusUnauthorized)
		return
	}

	userID, err := validateTicketWithLaravel(ticket, app)
	if err != nil || userID == "" {
		http.Error(w, "Invalid ticket", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	hub.Register(userID, conn)
	defer hub.Unregister(userID, conn)

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

func validateTicketWithLaravel(ticket string, app *AppConfig) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{"ticket": ticket})
	req, err := http.NewRequest("POST", app.TicketURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+app.Secret)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil // Invalid ticket
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}

	if userID, ok := result["user_id"].(string); ok {
		return userID, nil
	}
	if userIDFloat, ok := result["user_id"].(float64); ok {
		return strconv.FormatFloat(userIDFloat, 'f', -1, 64), nil
	}

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
