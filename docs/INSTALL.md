# HA VIP Manager v1.0 - Installation Guide

## Overview
HA VIP Manager v1.0 provides high-availability Virtual IP management with Kubernetes integration support. This guide covers installation methods for different environments.

## Prerequisites

- Linux system (AMD64 or ARM64)
- Root privileges for VIP assignment
- Network connectivity between cluster nodes
- For Kubernetes integration: Access to Kubernetes API server

## Installation Methods

### Method 1: Download and Install from GitHub Release

#### 1. Download the Release
```bash
# For AMD64 systems
wget https://github.com/2bleere/ha-vip/releases/download/ha-vip-1.0/ha-vip-1.0-x86_64.tar.gz

# For ARM64 systems  
wget https://github.com/2bleere/ha-vip/releases/download/ha-vip-1.0/ha-vip-1.0-arm64.tar.gz
```

#### 2. Extract and Install
```bash
# Extract the archive
tar -xzf ha-vip-1.0-*.tar.gz
cd ha-vip-1.0*

# Run the setup script (requires sudo)
sudo ./setup_ha_vip.sh
```

#### 3. Verify Installation
```bash
# Check if service is running
sudo systemctl status ha-vip

# Check version
ha-vip -version
```

### Method 2: Manual Installation

#### 1. Download Binary Only
```bash
# For AMD64 systems - download and extract to get binary
wget https://github.com/2bleere/ha-vip/releases/download/ha-vip-1.0/ha-vip-1.0-x86_64.tar.gz
tar -xzf ha-vip-1.0-x86_64.tar.gz
cd ha-vip-1.0
chmod +x ha-vip

# For ARM64 systems
wget https://github.com/2bleere/ha-vip/releases/download/ha-vip-1.0/ha-vip-1.0-arm64.tar.gz
tar -xzf ha-vip-1.0-arm64.tar.gz
cd ha-vip-1.0-arm64
chmod +x ha-vip-linux-arm64
# Note: ARM64 binary has different name
```

#### 2. Install Manually
```bash
# Copy binary to system path (adjust binary name for your architecture)
# For AMD64:
sudo cp ha-vip /usr/local/bin/
# For ARM64:
# sudo cp ha-vip-linux-arm64 /usr/local/bin/ha-vip

sudo chmod +x /usr/local/bin/ha-vip

# Set required capabilities for non-root execution
sudo setcap cap_net_admin+ep /usr/local/bin/ha-vip

# Create configuration directory
sudo mkdir -p /etc/ha-vip
```

#### 3. Create Configuration
```bash
# Create basic configuration file
sudo tee /etc/ha-vip/config.yaml > /dev/null << 'EOF'
# Basic HA VIP configuration
node_id: "node1"
priority: 1
interface: "eth0"
vip: "192.168.1.200/24"
peers:
  - "192.168.1.101:9999"
  - "192.168.1.102:9999"
port: 9999
heartbeat_interval: 1
election_timeout: 2
tls_cert: "cert.pem"
tls_key: "key.pem"

# Kubernetes integration (optional)
k8s:
  enabled: false
  api_server: ""
  token: ""
  ca_cert: ""
EOF
```

#### 4. Generate TLS Certificates (Optional)
```bash
# Generate self-signed certificates for peer communication
cd /etc/ha-vip
sudo openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=ha-vip"
```

#### 5. Create Systemd Service
```bash
sudo tee /etc/systemd/system/ha-vip.service > /dev/null << 'EOF'
[Unit]
Description=HA VIP Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ha-vip -config /etc/ha-vip/config.yaml
WorkingDirectory=/etc/ha-vip
Restart=always
RestartSec=5
User=root
Environment=GODEBUG=x509ignoreCN=0

[Install]
WantedBy=multi-user.target
EOF
```

#### 6. Enable and Start Service
```bash
sudo systemctl daemon-reload
sudo systemctl enable ha-vip
sudo systemctl start ha-vip
```

## Configuration

### Basic Configuration
Edit `/etc/ha-vip/config.yaml`:

```yaml
# Node identification
node_id: "node1"          # Unique identifier for this node
priority: 1               # Lower number = higher priority

# Network settings
interface: "eth0"          # Interface to assign VIP to
vip: "192.168.1.200/24"   # Virtual IP address with subnet

# Cluster peers
peers:
  - "192.168.1.101:9999"  # Other cluster members
  - "192.168.1.102:9999"

# Communication
port: 9999
heartbeat_interval: 1     # Seconds between heartbeats
election_timeout: 2       # Seconds before re-election
```

### Kubernetes Integration
For Kubernetes-aware VIP management:

```yaml
k8s:
  enabled: true
  api_server: "https://k8s-api.example.com:6443"
  token: "eyJhbGciOiJSUzI1NiIs..."  # Service account token
  ca_cert: "/etc/kubernetes/pki/ca.crt"  # Optional CA cert path
```

## Multi-Node Setup

### Node 1 (Priority 1 - Highest)
```yaml
node_id: "k8s-master-1"
priority: 1
# ... rest of config
```

### Node 2 (Priority 2 - Medium)
```yaml
node_id: "k8s-master-2"
priority: 2
# ... rest of config
```

### Node 3 (Priority 3 - Lowest)
```yaml
node_id: "k8s-master-3"
priority: 3
# ... rest of config
```

## Service Management

```bash
# Start service
sudo systemctl start ha-vip

# Stop service
sudo systemctl stop ha-vip

# Restart service
sudo systemctl restart ha-vip

# Check status
sudo systemctl status ha-vip

# View logs
sudo journalctl -u ha-vip -f

# Enable auto-start
sudo systemctl enable ha-vip

# Disable auto-start
sudo systemctl disable ha-vip
```

## Troubleshooting

### Check Service Status
```bash
sudo systemctl status ha-vip
sudo journalctl -u ha-vip --no-pager -l
```

### Test Configuration
```bash
# Test configuration without starting service
ha-vip -config /etc/ha-vip/config.yaml &
# Check logs for any errors
sudo journalctl -u ha-vip -f
```

### Network Connectivity
```bash
# Test peer connectivity
nc -zv <peer-ip> 9999

# Check if VIP is assigned
ip addr show dev eth0 | grep 192.168.1.200
```

### Kubernetes Integration Issues
```bash
# Test API server connectivity
curl -k -H "Authorization: Bearer $TOKEN" https://k8s-api.example.com:6443/readyz

# Check if client-go can connect
ha-vip -config /etc/ha-vip/config.yaml 2>&1 | grep "K8s"
```

## Security Considerations

1. **TLS Certificates**: Use proper certificates for peer communication
2. **Firewall Rules**: Ensure port 9999 (or configured port) is open between peers
3. **Service Account**: Use minimal privileges for Kubernetes service account tokens
4. **File Permissions**: Secure configuration files containing tokens

## Uninstall

```bash
# Stop and disable service
sudo systemctl stop ha-vip
sudo systemctl disable ha-vip

# Remove files
sudo rm -f /usr/local/bin/ha-vip
sudo rm -f /etc/systemd/system/ha-vip.service
sudo rm -rf /etc/ha-vip

# Reload systemd
sudo systemctl daemon-reload
```

## Support

- **GitHub Issues**: https://github.com/2bleere/ha-vip/issues
- **Documentation**: https://github.com/2bleere/ha-vip/blob/main/README.md
- **Releases**: https://github.com/2bleere/ha-vip/releases
