class GoWS {
    /**
     * @param {Object} config Options for initialization
     * @param {string} config.ticketUrl The URL on Laravel pointing to generation endpoint (e.g. /api/ws/ticket)
     * @param {string} [config.token] Authorization token if required for your /api/ws/ticket HTTP request
     * @param {boolean} [config.autoReconnect=true] Automatically reconnect if connection dies
     * @param {number} [config.reconnectInterval=3000] Interval before retry in ms
     */
    constructor(config) {
        this.ticketUrl = config.ticketUrl;
        this.token = config.token || null;
        this.csrfToken = config.csrfToken || null;
        this.withCredentials = config.withCredentials !== false; // Default to true to send session cookies
        this.autoReconnect = config.autoReconnect !== false;
        this.reconnectInterval = config.reconnectInterval || 3000;
        
        this.ws = null;
        this.eventListeners = {};
        this.activeChannels = new Set();
        this.isReconnecting = false;
        this.intentionalClose = false;

        if (!this.ticketUrl) {
            throw new Error('Config property "ticketUrl" is required');
        }
    }

    /**
     * Start the connection sequence
     */
    async connect() {
        this.intentionalClose = false;
        try {
            console.log('[GoWS] Fetching connection ticket from Laravel...');
            const ticketData = await this._fetchTicket();
            
            if (!ticketData.ticket || !ticketData.ws_url) {
                throw new Error("Invalid ticket response from server");
            }
            
            console.log('[GoWS] Ticket acquired. Connecting immediately to ' + ticketData.ws_url);
            
            // Assuming ws_url returned from laravel is e.g. "ws://localhost:8080/ws"
            const urlWithParams = new URL(ticketData.ws_url);
            urlWithParams.searchParams.append('ticket', ticketData.ticket);

            this._establishWS(urlWithParams.toString());
        } catch (error) {
            console.error('[GoWS] Connection initialization failed:', error);
            this._scheduleReconnect();
        }
    }

    /**
     * Disconnect gracefully
     */
    disconnect() {
        this.intentionalClose = true;
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }

    /**
     * Subscribe to a channel
     */
    subscribe(channel) {
        this.activeChannels.add(channel);
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ action: "subscribe", channel: channel }));
        }
    }

    /**
     * Unsubscribe from a channel
     */
    unsubscribe(channel) {
        this.activeChannels.delete(channel);
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ action: "unsubscribe", channel: channel }));
        }
    }

    /**
     * Register a callback for a specific event sent from Go (originally fired by Laravel)
     */
    on(event, callback) {
        if (!this.eventListeners[event]) {
            this.eventListeners[event] = [];
        }
        this.eventListeners[event].push(callback);
    }
    
    _emit(event, payload) {
        const callbacks = this.eventListeners[event] || [];
        callbacks.forEach(cb => cb(payload));
    }

    async _fetchTicket() {
        const headers = {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
        };
        
        // If your Laravel backend requires an auth token
        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }
        if (this.csrfToken) {
            headers['X-CSRF-TOKEN'] = this.csrfToken;
        }

        const fetchConfig = {
            method: 'POST', 
            headers: headers 
        };

        // Send session cookies automatically so Laravel can read the session
        if (this.withCredentials) {
            fetchConfig.credentials = 'include';
        }

        const res = await fetch(this.ticketUrl, fetchConfig);
        
        if (!res.ok) {
            throw new Error(`HTTP Error ${res.status}: Could not fetch ticket`);
        }
        
        return await res.json();
    }

    _establishWS(url) {
        this.ws = new WebSocket(url);

        this.ws.onopen = (event) => {
            console.log('[GoWS] WebSocket Connected!');
            this.isReconnecting = false;

            // Restore channel subscriptions
            this.activeChannels.forEach(channel => {
                this.ws.send(JSON.stringify({ action: "subscribe", channel: channel }));
            });

            this._emit('connect', event);
        };

        this.ws.onmessage = (event) => {
            try {
                // Parse the JSON payload coming from the server
                const data = JSON.parse(event.data);
                
                // Expecting structure { "event": "...", "payload": { ... } }
                if (data.event) {
                    this._emit(data.event, data.payload);
                }
                
                // Generic catch-all listener
                this._emit('*', data);
            } catch (error) {
                console.error('[GoWS] Failed to parse incoming message JSON:', event.data);
            }
        };

        this.ws.onclose = (event) => {
            console.log(`[GoWS] Connection closed (code ${event.code}).`);
            this._emit('disconnect', event);
            if (!this.intentionalClose) {
                this._scheduleReconnect();
            }
        };

        this.ws.onerror = (error) => {
            console.error('[GoWS] WebSocket connection error:', error);
        };
    }

    _scheduleReconnect() {
        if (!this.autoReconnect || this.isReconnecting) return;
        this.isReconnecting = true;

        console.log(`[GoWS] Scheduling reconnect in ${this.reconnectInterval}ms...`);
        setTimeout(() => {
            this.connect();
        }, this.reconnectInterval);
    }
}
