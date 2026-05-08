package main

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu sync.RWMutex
	// Maps a string User ID to a set of active connections
	connections map[string]map[*websocket.Conn]bool
	// Maps a string Channel Name to a set of active connections
	channels map[string]map[*websocket.Conn]bool
}

type HubSnapshot struct {
	Users       int
	Connections int
	Channels    int
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
	debugLog("User %s connected.", userID)
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
			debugLog("Connection unsubscribed from channel: %s", channelName)
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
	debugLog("Connection subscribed to channel: %s", channel)
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
			debugLog("Connection unsubscribed from channel: %s", channel)
		}
	}
}

// BroadcastToUser sends a message to all active connections of a specific user
func (h *Hub) BroadcastToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.connections[userID]; ok {
		debugLog("Found %d connections for user %s", len(conns), userID)
		for conn := range conns {
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Error writing to user %s: %v", userID, err)
			} else {
				debugLog("Successfully sent message to user %s", userID)
			}
		}
	} else {
		debugLog("No active connections found for user: %s", userID)
	}
}

// BroadcastToChannel sends a message to all connections subscribed to a specific channel
func (h *Hub) BroadcastToChannel(channel string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.channels[channel]; ok {
		debugLog("Found %d connections for channel %s", len(conns), channel)
		for conn := range conns {
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Error writing to channel %s: %v", channel, err)
			} else {
				debugLog("Successfully sent message to channel %s", channel)
			}
		}
	} else {
		debugLog("No active connections found for channel: %s", channel)
	}
}

func (h *Hub) Snapshot() HubSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	snapshot := HubSnapshot{
		Users:    len(h.connections),
		Channels: len(h.channels),
	}
	for _, conns := range h.connections {
		snapshot.Connections += len(conns)
	}

	return snapshot
}
