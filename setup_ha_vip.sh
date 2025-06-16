#!/bin/bash

set -e

# Define paths
BIN_SRC="./ha-vip"
BIN_DST="/usr/local/bin/ha-vip"
CONFIG_DIR="/etc/ha-vip"
SERVICE_FILE="./ha-vip.service"
TLS_CERT="./cert.pem"
TLS_KEY="./key.pem"
CONFIG_FILE="./config.yaml"

echo "🔧 Installing HA VIP Manager..."

# Copy binary
echo "📦 Copying binary to /usr/local/bin..."
sudo cp "$BIN_SRC" "$BIN_DST"
sudo chmod +x "$BIN_DST"

# Set capability
echo "🔐 Setting cap_net_admin capability..."
sudo setcap cap_net_admin+ep "$BIN_DST"

# Create config directory
echo "📁 Creating configuration directory at $CONFIG_DIR..."
sudo mkdir -p "$CONFIG_DIR"

# Copy config and TLS files
echo "📄 Copying configuration and TLS files..."
sudo cp "$CONFIG_FILE" "$TLS_CERT" "$TLS_KEY" "$CONFIG_DIR"

# Copy systemd service file
echo "🛠️ Installing systemd service..."
sudo cp "$SERVICE_FILE" /etc/systemd/system/ha-vip.service
sudo systemctl daemon-reexec
sudo systemctl enable ha-vip
sudo systemctl start ha-vip

echo "✅ HA VIP Manager setup complete and service started."
