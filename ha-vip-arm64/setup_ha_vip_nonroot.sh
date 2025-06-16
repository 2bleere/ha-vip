#!/bin/bash

set -e

# Variables
BIN_SRC="./ha-vip"
BIN_DST="/usr/local/bin/ha-vip"
CONFIG_DIR="/etc/ha-vip"
TLS_CERT="./cert.pem"
TLS_KEY="./key.pem"
CONFIG_FILE="./config.yaml"
SERVICE_FILE="/etc/systemd/system/ha-vip.service"
USER="ha-user"

# Copy binary
echo "📦 Copying binary to /usr/local/bin..."
sudo cp "$BIN_SRC" "$BIN_DST"
sudo chmod +x "$BIN_DST"

# 1. Create system user
if ! id "$USER" &>/dev/null; then
    echo "Creating system user: $USER"
    sudo useradd -r -s /bin/false "$USER"
fi

# 2. Set capabilities on the binary
echo "Setting CAP_NET_ADMIN capability on $BIN_DST"
sudo setcap cap_net_admin+ep "$BIN_DST"

# 3. Ensure config directory exists and set permissions
echo "Setting ownership of $CONFIG_DIR to $USER"
sudo mkdir -p "$CONFIG_DIR"
sudo chown -R "$USER:$USER" "$CONFIG_DIR"

echo "📄 Copying configuration and TLS files..."
sudo cp "$CONFIG_FILE" "$TLS_CERT" "$TLS_KEY" "$CONFIG_DIR"

# 4. Create systemd service file
echo "Creating systemd service file at $SERVICE_FILE"
sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=HA VIP Manager (Non-root)
After=network.target

[Service]
Type=simple
ExecStart=$BIN_DST
WorkingDirectory=$CONFIG_DIR
Restart=always
RestartSec=5
User=$USER
Group=$USER
AmbientCapabilities=CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_ADMIN
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

# 5. Reload systemd and enable service
echo "Reloading systemd and enabling ha-vip service"
sudo systemctl daemon-reexec
sudo systemctl enable ha-vip
sudo systemctl start ha-vip

echo "✅ Setup complete. HA VIP Manager is running as $USER."
