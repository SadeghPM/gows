# Go & Laravel Simple WebSocket Integration

🚀 **Want to get up and running fast? See the [Quick Start Guide](QUICKSTART.md).**

This workspace contains all the necessary components for an ultra-lightweight WebSocket integration completely avoiding complex libraries like Pusher, Laravel Echo, or Reverb.

## Architecture Highlights
- **Go Server (`go-server/`)**: Extremely low overhead. Acts as the `Hub`. Clients connect here using a ticket and wait for payloads.
- **Laravel Backend (`laravel-app/`)**: Generates authorization tickets internally via caching. Broadcasts real-time events to the Go Hub over a secure HTTP request.
- **Security Check:** All server-to-server communication (Laravel -> Go Broadcast, Go -> Laravel Ticket Validation) is authenticated purely via an `Authorization: Bearer <Secret Key>` header. There are no IP or internal network restrictions built-in. Therefore, deploying the Go server on a standalone public domain (e.g. `gows.maindom.com`) works seamlessly as long as both servers share the same secret key.
- **JS Client (`client/gows.js`)**: A tiny (100 LOC) vanilla Javascript utility that manages fetching the ticket, connecting to the Go server, handling reconnects, and firing typed JavaScript events.

## How to test the proof-of-concept

### 1. Start the Go Server

You can configure the Go server using environment variables.
**Note on Security:** Even if the Go Server is hosted on a public domain (like `gows.maindom.com`), all server-to-server communication is protected by the `INTERNAL_SECRET`. There are no IP or network restrictions by default; as long as Laravel and Go share the exact same secret key, they will trust each other!

```bash
# The absolute URL where Go will ask Laravel if a ticket is valid
export LARAVEL_TICKET_URL="http://localhost:8000/api/internal/ws/validate-ticket"
# The shared secret key protecting server-to-server communication
export INTERNAL_SECRET="super-secret-internal-key"
export PORT="8080"
```

You can also create a `.env` file right inside the `go-server/` directory, which the binary will automatically load on boot!

#### Run manually via Makefile:
```bash
cd go-server
make run
```

#### Run as a Background Service (Ubuntu / systemd):
An installer script and `gows.service` file are included. Run the setup once to turn it into a system daemon!
```bash
cd go-server
make install-ubuntu

# Useful systemd commands afterwards:
sudo systemctl status gows
sudo nano /etc/gows/.env    # Edit ENV vars here and restart
sudo systemctl restart gows
sudo journalctl -u gows -f  # Tail logs
```

### 2. Implement the Laravel code
Copy the files in `laravel-app` to your real Laravel backend. 

Key environment variables to add to your `.env` (adjust these for production domain names):
```env
# What the JS client uses to connect (give the public domain to the client)
WS_SERVER_URL=wss://gows.maindom.com/ws

# What Laravel uses to push events (internal HTTP call)
WS_BROADCAST_URL=https://gows.maindom.com/api/internal/broadcast

# The shared secret key protecting the internal communication
WS_INTERNAL_SECRET=super-secret-internal-key
```

### 3. Usage inside Laravel application 

**Terminology Clarification:**
- **User Targeting (`user_id`)**: A securely routed message sent directly to an authenticated user. **No channel subscription is needed** on the client side. The Go Server automatically links the active connection to the `user_id` when the ticket is validated.
- **Channel / Room (`channel`)**: A grouping or logical room (e.g., `global-news`, `room-123`). Clients must explicitly `subscribe('channel-name')` to receive messages sent to a channel.
- **Event (`event`)**: The specific action or message type being matched on the client-side (e.g., `ArticlePublished`, `OrderPlaced`). Clients must listen for this specific string using `ws.on('EventName', ...)`.

Anywhere in your Laravel code, to securely push an event to a specific user ONLY:
```php
\App\Services\GoWebSocketService::broadcastToUser(
    userId: 9, 
    event: 'OrderPlaced', 
    payload: ['order_id' => 99, 'amount' => '$59.00']
);
```

Or to push an event to an entire channel simultaneously:
```php
\App\Services\GoWebSocketService::broadcastToChannel(
    channel: 'global-news', 
    event: 'ArticlePublished', 
    payload: ['title' => 'Breaking News!']
);
```

### 4. Client Side Implementation
In your frontend, import the JS library and subscribe to the events targeted at your user logic.
```javascript
// Make sure you have <meta name="csrf-token" content="{{ csrf_token() }}"> in your <head>
const ws = new GoWS({
    ticketUrl: "/ws/generate-ticket", // Path to the ticket generation web route
    csrfToken: document.querySelector('meta[name="csrf-token"]')?.getAttribute('content'),
    withCredentials: true // send session cookies to laravel
});

// PRIVATE USER MESSAGES: Start listening for standard user-targeted events securely
// Because your ticket verifies your Laravel Session, your connection is ALREADY bound to your user ID.
// You DO NOT need to subscribe to 'user.1'. Just listen for the event!
ws.on('OrderPlaced', (payload) => {
    console.log("New private order for me!", payload);
});

// PUBLIC/GROUP CHANNELS:
// IMPORTANT: Tell the Go WebSocket server we want to receive messages sent to specific channels.
// You can subscribe to as many channels as you need!
ws.subscribe('global-news');
ws.subscribe('sports-updates');

// Listen for the specific "event" string broadcasted from Laravel
ws.on('ArticlePublished', (payload) => {
    console.log("News arrived", payload.title);
});

ws.on('MatchScoreUpdate', (payload) => {
    console.log("Score update:", payload.score);
});

ws.connect();
```
