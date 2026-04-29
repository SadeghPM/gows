# GoWS: Laravel & Go WebSocket Integration

**GoWS** is a high-performance, ultra-lightweight WebSocket integration for Laravel applications, powered by a Go server. It completely replaces the need for complex solutions like Pusher, Laravel Echo, or Reverb, while delivering raw speed and minimal overhead.

---

## Introduction

At its core, **GoWS** consists of three key components working in harmony:

1. **The Go Hub (`go-server`)**: A blazing fast WebSocket server written in Go. Clients connect to it using an authorized ticket, and wait in "Channels" or "User Rooms" for payloads.
2. **The Laravel Backend (`laravel-app`)**: Handles your application logic. It generates secure authorization tickets, validates connections, and pushes real-time events to the Go Hub via a secure, internal HTTP webhook.
3. **The JS Client (`gows.js`)**: A tiny (100 LOC) vanilla JavaScript utility that manages ticket fetching, socket connection, automatic reconnects, and event listeners.

> **Note:** For a rapid 3-step setup, see the **[Quick Start Guide](QUICKSTART.md)**.

---

## Installation & Setup

### 1. Server Configuration (Go Hub)

The Go Server acts as your central WebSocket router. It operates completely standalone and can run on any domain or subdomain. It uses a `config.yaml` file to determine where to validate its connections.

Create a `config.yaml` file in the `go-server/` directory:

```yaml
server:
  port: "8080"

apps:
  - id: "default"
    ticket_url: "http://localhost:8000/api/internal/ws/validate-ticket"
    secret: "super-secret-internal-key"
```

**Starting the server:**
```bash
cd go-server
make deps
make run
```

> **Deployment Note:** If deploying to a production Linux server, run `make build-linux` to generate a compiled binary, or use `make install-ubuntu` to automatically install it as a systemd background service.

### 2. Laravel Integration

Integrating GoWS into your Laravel application only requires copying a few classes and updating your environment.

1. **Copy the necessary files** into your Laravel application structure:
    * `app/Services/GoWebSocketService.php`
    * `app/Http/Controllers/WebSocketTicketController.php`
    * `app/Providers/GoWebSocketServiceProvider.php`
2. **Register the Service Provider** in your Laravel bootstrap file to automatically bind the ticket endpoints (`bootstrap/providers.php` or `config/app.php`).
3. **Configure your `.env` variables**:

```env
# The App ID matching your GoWS config.yaml
WS_APP_ID=default

# Client-facing URL where browsers will connect
WS_SERVER_URL=wss://gows.mydomain.com/ws

# Internal Server-to-Server URL where Laravel broadcasts events to Go
WS_BROADCAST_URL=https://gows.mydomain.com/api/internal/broadcast

# The shared secret key protecting the Go server
WS_INTERNAL_SECRET=super-secret-internal-key
```

---

## Broadcasting Server Events

GoWS uses a strictly typed architecture that separates **User Targeted** messages from **Public Channel** messages.

#### Core Terminology
* **User Targeting (`user_id`)**: Securely routes a message *directly* to an authenticated user's socket. Because Laravel validates the user during the initial connection handshake, **no channel subscription is needed** on the client side.
* **Channels (`channel`)**: Public or logical group rooms (e.g., `global-news`). Clients must utilize `ws.subscribe('channel-name')` to opt-in to these messages.
* **Events (`event`)**: The exact string matching the action you want your JavaScript frontend to listen for (e.g., `OrderPlaced`).

### Broadcasting to a Specific User

Anywhere in your backend, trigger a push event straight to a specific user ID:

```php
use App\Services\GoWebSocketService;

GoWebSocketService::broadcastToUser(
    userId: auth()->id(), 
    event: 'OrderPlaced', 
    payload: ['order_id' => 99, 'amount' => '$59.00']
);
```

### Broadcasting to a Channel

To send an event to thousands of users simultaneously subscribed to a specific group:

```php
use App\Services\GoWebSocketService;

GoWebSocketService::broadcastToChannel(
    channel: 'global-news', 
    event: 'ArticlePublished', 
    payload: ['title' => 'Breaking News!']
);
```

---

## Client Side Usage

Drop the `gows.js` script into your frontend layout. The client library handles the heavy lifting, session cookie transmission, and JSON parsing.

### Initialization
> Ensure your layout contains the standard Laravel CSRF token: `<meta name="csrf-token" content="{{ csrf_token() }}">`

```html
<script src="/js/gows.js"></script>
<script>
    const ws = new GoWS({
        ticketUrl: "/ws/generate-ticket", // Path registered by the Service Provider
        csrfToken: document.querySelector('meta[name="csrf-token"]')?.getAttribute('content'),
        withCredentials: true // Strongly encourages sending session cookies to Laravel
    });
    
    // Connect to the Hub
    ws.connect();
</script>
```

### Listening for Events 

#### User Events
Since your connection is automatically bound to your secure Laravel Session, you instantly receive private user events without needing to join custom rooms:

```javascript
ws.on('OrderPlaced', (payload) => {
    console.log("Your order was confirmed:", payload);
});
```

#### Channel Events
If you wish to receive events sent to a `channel`, explicitly subscribe to it, then attach your event listener. You can subscribe to as many channels as you require!

```javascript
ws.subscribe('global-news');
ws.subscribe('sports-updates');

ws.on('ArticlePublished', (payload) => {
    alert("New Article: " + payload.title);
});
```

---

## Multi-Tenancy

GoWS supports out-of-the-box absolute multi-tenancy. You can serve 1, 10, or 100 completely distinct Laravel applications from a *single* running GoWS binary process without overlapping memory or connections.

Add additional applications to your server's `config.yaml`:

```yaml
server:
  port: "8080"

apps:
  - id: "laravel-app-1"
    ticket_url: "https://api.app1.com/api/internal/ws/validate-ticket"
    secret: "super-secret-key-1"

  - id: "laravel-app-2"
    ticket_url: "https://api.app2.com/api/internal/ws/validate-ticket"
    secret: "super-secret-key-2"
```

In Laravel App 2's `.env`, update the Application identifier:
```env
WS_APP_ID=laravel-app-2
```

GoWS natively maps the HTTP webhook authentication via Bearer Token directly to the `AppID`, ensuring bulletproof isolation across your Laravel ecosystem.