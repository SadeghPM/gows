package apps

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHub_RegisterAndSubscribe(t *testing.T) {
	hub := NewHub()

	// Create a dummy websocket server exactly to get a real connection instance
	connections := make(chan *websocket.Conn, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		c, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			connections <- c
		}
	}))
	defer s.Close()

	// Connect a dummy client
	url := strings.Replace(s.URL, "http://", "ws://", 1)
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer clientConn.Close()

	// Wait for server connection
	var serverConn *websocket.Conn
	select {
	case serverConn = <-connections:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for server connection")
	}

	// 1. Test Register
	hub.Register("user1", serverConn)
	hub.mu.RLock()
	if !hub.connections["user1"][serverConn] {
		t.Error("Connection not registered")
	}
	hub.mu.RUnlock()

	// 2. Test Subscribe
	hub.Subscribe(serverConn, "channel1")
	hub.mu.RLock()
	if !hub.channels["channel1"][serverConn] {
		t.Error("Connection not subscribed to channel")
	}
	hub.mu.RUnlock()

	// 3. Test Unsubscribe
	hub.Unsubscribe(serverConn, "channel1")
	hub.mu.RLock()
	if hub.channels["channel1"][serverConn] {
		t.Error("Connection still subscribed after Unsubscribe")
	}
	hub.mu.RUnlock()

	// 4. Test Unregister (This will also close the connection)
	hub.Unregister("user1", serverConn)
	hub.mu.RLock()
	if len(hub.connections["user1"]) != 0 {
		t.Error("Connection still registered after Unregister")
	}
	hub.mu.RUnlock()
}
