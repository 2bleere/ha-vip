#!/bin/bash

# Setup script for non-root deployment of HA VIP Manager

set -e

echo "Setting up HA VIP Manager for non-root deployment..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "This setup script must be run as root (use sudo)"
    exit 1
fi

# Configuration
BINARY_PATH="/usr/local/bin/ha-vip"
CONFIG_DIR="/etc/ha-vip"
SERVICE_FILE="/etc/systemd/system/ha-vip.service"
USER_NAME="ha-vip"

# Check if binary exists
if [ ! -f "$BINARY_PATH" ]; then
    echo "Error: HA VIP Manager binary not found at $BINARY_PATH"
    echo "Please copy the binary to $BINARY_PATH first"
    exit 1
fi

echo "1. Creating dedicated user for HA VIP Manager..."
if ! id "$USER_NAME" &>/dev/null; then
    useradd -r -s /bin/false -M "$USER_NAME"
    echo "   Created user: $USER_NAME"
else
    echo "   User $USER_NAME already exists"
fi

echo "2. Setting up configuration directory..."
mkdir -p "$CONFIG_DIR"
chown "$USER_NAME:$USER_NAME" "$CONFIG_DIR"
chmod 750 "$CONFIG_DIR"

# Copy config if it doesn't exist
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    if [ -f "./config.yaml" ]; then
        cp "./config.yaml" "$CONFIG_DIR/"
        chown "$USER_NAME:$USER_NAME" "$CONFIG_DIR/config.yaml"
        chmod 600 "$CONFIG_DIR/config.yaml"
        echo "   Copied config.yaml to $CONFIG_DIR"
    else
        echo "   Warning: No config.yaml found to copy"
    fi
fi

echo "3. Setting network capabilities on binary..."
setcap 'cap_net_admin=+ep' "$BINARY_PATH"
echo "   Granted CAP_NET_ADMIN to $BINARY_PATH"

# Verify capabilities
CAPS=$(getcap "$BINARY_PATH")
echo "   Capabilities: $CAPS"

echo "4. Installing systemd service..."
cp "./deployments/ha-vip-nonroot.service" "$SERVICE_FILE"
systemctl daemon-reload
echo "   Service installed: $SERVICE_FILE"

echo "5. Setup complete!"
echo ""
echo "To start HA VIP Manager:"
echo "  sudo systemctl enable ha-vip"
echo "  sudo systemctl start ha-vip"
echo ""
echo "To check status:"
echo "  sudo systemctl status ha-vip"
echo "  sudo journalctl -u ha-vip -f"
echo ""
echo "To verify non-root operation:"
echo "  ps aux | grep ha-vip"
echo "  getcap $BINARY_PATH"
