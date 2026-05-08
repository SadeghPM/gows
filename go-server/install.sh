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
go build -o gows ./cmd/gows

# 2. Move binary to /usr/local/bin
echo "Moving binary to /usr/local/bin/gows..."
mv gows /usr/local/bin/gows
chmod +x /usr/local/bin/gows

# 3. Setup configuration directory and default config.yaml
echo "Setting up configuration in /etc/gows..."
mkdir -p /etc/gows

# Create default config file if it doesn't exist
if [ ! -f /etc/gows/config.yaml ]; then
cat <<EOF > /etc/gows/config.yaml
server:
  port: "8080"

apps:
  - id: "default"
    name: "Default"
    ticket_url: "http://localhost:8000/api/internal/ws/validate-ticket"
    secret: "super-secret-internal-key"
EOF
echo "Created default config at /etc/gows/config.yaml. Please configure this file manually."
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
echo "  sudo nano /etc/gows/config.yaml"
echo "  sudo systemctl restart gows"
echo "========================================="
