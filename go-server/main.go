package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

var hub *Hub
var laravelTicketURL = "http://localhost:8000/api/internal/ws/validate-ticket"
var internalSecret = "super-secret-internal-key"
var serverPort = "8080"

func initConfig() {
	// Attempt to load .env file if it exists, otherwise ignore error (e.g. systemd passes env directly)
	_ = godotenv.Load()

	if url := os.Getenv("LARAVEL_TICKET_URL"); url != "" {
		laravelTicketURL = url
	}
	if secret := os.Getenv("INTERNAL_SECRET"); secret != "" {
		internalSecret = secret
	}
	if port := os.Getenv("PORT"); port != "" {
		serverPort = port
	}
}

func main() {
	initConfig()
	hub = NewHub()

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
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		http.Error(w, "Missing ticket", http.StatusUnauthorized)
		return
	}

	userID, err := validateTicketWithLaravel(ticket)
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

	// Keep alive & Client Message pump (for subscribing to channels)
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

func validateTicketWithLaravel(ticket string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{"ticket": ticket})
	req, err := http.NewRequest("POST", laravelTicketURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+internalSecret)

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
	if authHeader != "Bearer "+internalSecret {
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
		log.Printf("Broadcasting event '%s' to channel: %s", payload.Event, payload.Channel)
		hub.BroadcastToChannel(payload.Channel, body)
	} else if payload.UserID != "" {
		log.Printf("Broadcasting event '%s' to user: %s", payload.Event, payload.UserID)
		hub.BroadcastToUser(payload.UserID, body)
	} else {
		log.Printf("Failed broadcast: Missing user_id or channel. Body: %s", string(body))
		http.Error(w, "Missing user_id or channel", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"sent"}`))
}
