#!/bin/bash

set -e

# Detect binary name
if [ -f "./ha-vip" ]; then
    BIN_SRC="./ha-vip"
elif [ -f "./ha-vip-linux-arm64" ]; then
    BIN_SRC="./ha-vip-linux-arm64"
else
    echo "❌ Error: Could not find ha-vip binary in current directory"
    echo "Expected: ./ha-vip or ./ha-vip-linux-arm64"
    exit 1
fi

# Define paths
BIN_DST="/usr/local/bin/ha-vip"
CONFIG_DIR="/etc/ha-vip"
SERVICE_FILE="./ha-vip.service"
TLS_CERT="./cert.pem"
TLS_KEY="./key.pem"
CONFIG_FILE="./config.yaml"

echo "🔧 Installing HA VIP Manager..."
echo "📋 Using binary: $BIN_SRC"

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
