#!/usr/bin/env bash

# GoWS Installer Script for Ubuntu/systemd

set -e

# Ensure we're running as root
if [ "$EUID" -ne 0 ]
  then echo "Please run as root (sudo ./install.sh)"
  exit
fi

echo "Installing GoWS Server..."

# 1. Build the server
echo "Building the Go binary..."
go build -o gows main.go hub.go

# 2. Move binary to /usr/local/bin
echo "Moving binary to /usr/local/bin/gows..."
mv gows /usr/local/bin/gows
chmod +x /usr/local/bin/gows

# 3. Setup configuration directory and default .env
echo "Setting up configuration in /etc/gows..."
mkdir -p /etc/gows

# Create default .env file if it doesn't exist
if [ ! -f /etc/gows/.env ]; then
cat <<EOF > /etc/gows/.env
# Go WebSocket Server Environment Variables
LARAVEL_TICKET_URL="http://localhost:8000/api/internal/ws/validate-ticket"
INTERNAL_SECRET="super-secret-internal-key"
PORT=8080
EOF
echo "Created default config at /etc/gows/.env. Please configure this file manually."
fi

# 4. Install systemd service
echo "Installing systemd service file..."
cp gows.service /etc/systemd/system/gows.service
chmod 644 /etc/systemd/system/gows.service

# 5. Reload systemd, enable and start the service
echo "Reloading systemd daemon..."
systemctl daemon-reload

echo "Enabling GoWS service on boot..."
systemctl enable gows

echo "Starting GoWS service..."
systemctl restart gows

echo "========================================="
echo "GoWS successfully installed and started!"
echo "Check status:"
echo "  sudo systemctl status gows"
echo "View logs:"
echo "  sudo journalctl -u gows -f"
echo "Edit configuration (restart required):"
echo "  sudo nano /etc/gows/.env"
echo "  sudo systemctl restart gows"
echo "========================================="
