# GoWS Quick Start Guide
Get your high-performance, real-time WebSocket integration running in 3 simple steps.

## Step 1: Run the Go WebSocket Server

First, configure the Go server by creating a `.env` file inside the `go-server/` directory:
```env
# URL where Go asks Laravel to validate tickets
LARAVEL_TICKET_URL=http://localhost:8000/api/internal/ws/validate-ticket

# Shared secret key protecting server-to-server communication
INTERNAL_SECRET=super-secret-internal-key

# Port for the Go WebSocket server
PORT=8080
```

You can run the server locally on your machine or deploy it to a Linux server using the compiled binary.

### Local Development:
```bash
cd go-server
make deps
make run
```

### Production Deployment (Linux):
Compile the binary for Linux, move it to your server, and run it:
```bash
cd go-server
make build-linux
# This creates a 'gows-linux-amd64' binary file. Upload this to your production server.

# On your linux server:
./gows-linux-amd64
```
*(Optional: Run `make install-ubuntu` on the server to install it as a systemd service!)*

---

## Step 2: Configure Laravel Backend

Drop the provided code into your Laravel application and connect the two servers.

1. **Environment Variables**: Add these to your Laravel `.env` (update domains if in production):
    ```env
    WS_SERVER_URL=ws://localhost:8080/ws
    WS_BROADCAST_URL=http://localhost:8080/api/internal/broadcast
    WS_INTERNAL_SECRET=super-secret-internal-key
    ```
2. **Move Files**:
    - Copy `laravel-app/app/Services/GoWebSocketService.php` to your `app/Services/`.
    - Copy `laravel-app/app/Http/Controllers/WebSocketTicketController.php` to your `app/Http/Controllers/`.
3. **Register Routes**: Add to your `routes/web.php` and `routes/api.php` as shown in the example code so clients can fetch tickets and Go can validate them.

4. **Send an Event**: Anywhere in your Laravel controllers or jobs:
    ```php
    \App\Services\GoWebSocketService::broadcastToUser(
        auth()->id(), 
        'NewAlert', 
        ['msg' => 'Hello from Laravel!']
    );
    ```

---

## Step 3: Wire up the JS Client

Drop the lightning-fast client script into your frontend and listen for events!

1. Include the script in your layout (e.g. `public/js/gows.js`):
    ```html
    <!-- Make sure Laravel outputs the CSRF token in the head -->
    <meta name="csrf-token" content="{{ csrf_token() }}">
    
    <script src="/js/gows.js"></script>
    ```

2. Connect and listen:
    ```html
    <script>
        const ws = new GoWS({
            ticketUrl: "/ws/generate-ticket", // Your Laravel web route
            csrfToken: document.querySelector('meta[name="csrf-token"]').getAttribute('content')
        });

        // 1. Listen for the 'NewAlert' event you just broadcasted from Laravel
        ws.on('NewAlert', (payload) => {
            alert(payload.msg); // "Hello from Laravel!"
        });

        // 2. Start the connection!
        ws.connect();
    </script>
    ```

You're done! Your application is now real-time.