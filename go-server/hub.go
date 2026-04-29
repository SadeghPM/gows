package main

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu sync.RWMutex
	// Maps a string User ID to a set of active connections
	connections map[string]map[*websocket.Conn]bool
	// Maps a string Channel Name to a set of active connections
	channels map[string]map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]map[*websocket.Conn]bool),
		channels:    make(map[string]map[*websocket.Conn]bool),
	}
}

// Register adds a new connection for a specific user ID
func (h *Hub) Register(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.connections[userID] == nil {
		h.connections[userID] = make(map[*websocket.Conn]bool)
	}
	h.connections[userID][conn] = true
	log.Printf("User %s connected.", userID)
}

// Unregister removes a connection from the user list and all subscribed channels
func (h *Hub) Unregister(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from user connections
	if conns, ok := h.connections[userID]; ok {
		if _, exists := conns[conn]; exists {
			delete(conns, conn)
			conn.Close()
			if len(conns) == 0 {
				delete(h.connections, userID)
			}
		}
	}

	// Remove from all channels
	for channelName, conns := range h.channels {
		if _, exists := conns[conn]; exists {
			delete(conns, conn)
			if len(conns) == 0 {
				delete(h.channels, channelName)
			}
			log.Printf("Connection unsubscribed from channel: %s", channelName)
		}
	}
}

// Subscribe adds a connection to a specific channel
func (h *Hub) Subscribe(conn *websocket.Conn, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*websocket.Conn]bool)
	}
	h.channels[channel][conn] = true
	log.Printf("Connection subscribed to channel: %s", channel)
}

// Unsubscribe removes a connection from a specific channel
func (h *Hub) Unsubscribe(conn *websocket.Conn, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, ok := h.channels[channel]; ok {
		if _, exists := conns[conn]; exists {
			delete(conns, conn)
			if len(conns) == 0 {
				delete(h.channels, channel)
			}
			log.Printf("Connection unsubscribed from channel: %s", channel)
		}
	}
}

// BroadcastToUser sends a message to all active connections of a specific user
func (h *Hub) BroadcastToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.connections[userID]; ok {
		log.Printf("Found %d connections for user %s", len(conns), userID)
		for conn := range conns {
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Error writing to user %s: %v", userID, err)
			} else {
				log.Printf("Successfully sent message to user %s", userID)
			}
		}
	} else {
		log.Printf("No active connections found for user: %s", userID)
	}
}

// BroadcastToChannel sends a message to all connections subscribed to a specific channel
func (h *Hub) BroadcastToChannel(channel string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.channels[channel]; ok {
		log.Printf("Found %d connections for channel %s", len(conns), channel)
		for conn := range conns {
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Error writing to channel %s: %v", channel, err)
			} else {
				log.Printf("Successfully sent message to channel %s", channel)
			}
		}
	} else {
		log.Printf("No active connections found for channel: %s", channel)
	}
}
