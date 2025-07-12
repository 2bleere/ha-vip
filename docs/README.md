# HA VIP Manager

A high-availability Virtual IP manager for Linux systems with fast failover capabilities.

## Overview

HA VIP Manager provides automatic Virtual IP (VIP) management across multiple nodes in a cluster, ensuring your services remain accessible even when individual nodes fail. It uses a distributed heartbeat and election mechanism to maintain high availability with minimal downtime.

## Features

- **Fast Failover**: Sub-second VIP reassignment during node failures
- **Event-Driven Architecture**: Immediate response to leadership changes
- **Simple Configuration**: Easy YAML-based setup
- **No External Dependencies**: Self-contained binary with no database requirements
- **Cross-Platform**: Works on any Linux distribution (ARM64/AMD64)
- **Priority-Based Elections**: Configure node priorities for election control
- **Optimized Performance**: Tunable parameters for different network conditions
- **Kubernetes Integration**: Health-aware VIP assignment based on Kubernetes API server readiness

## Kubernetes Integration

HA VIP Manager can integrate with Kubernetes to make VIP assignment decisions based on the health of the local Kubernetes API server. This ensures that the VIP is always assigned to a node with a healthy Kubernetes API server.

### How it works

1. **Health Monitoring**: Continuously monitors the local Kubernetes API server using both client-go and `/readyz` endpoint
2. **Priority-Based Selection**: Among healthy nodes, selects the one with the highest priority (lowest priority number)
3. **Fallback Strategy**: If no nodes have healthy API servers, assigns VIP to the highest priority node
4. **Real-time Failover**: Immediately reassigns VIP when the current leader's API server becomes unhealthy

### Configuration

Enable Kubernetes integration in your `config.yaml`:

```yaml
k8s:
  enabled: true
  api_server: "https://k8s-api.example.com"
  token: "your-k8s-token"           # Service account token
  ca_cert: "ca.crt"                 # Optional: CA certificate file
node_id: "node1"
priority: 1                         # Lower number = higher priority
# ...rest of configuration
```

### Authentication

The tool uses Kubernetes service account tokens for authentication. You can:

1. **Use in-cluster config**: When running inside Kubernetes, the tool can use the pod's service account
2. **Provide token explicitly**: Set the `token` field in the configuration
3. **Skip TLS verification**: Leave `ca_cert` empty (not recommended for production)

### Requirements

- Kubernetes cluster with accessible API server
- Service account with permissions to access the API server
- Network connectivity to the API server from all nodes

## System Requirements

- Linux (ARM64 or AMD64)
- Root privileges (for IP assignment)
- Network connectivity between all nodes
- Shared subnet for the Virtual IP

## Installation

### Quick Install from GitHub Release (v1.0)

```bash
# Download and extract (AMD64)
wget https://github.com/2bleere/ha-vip/releases/download/ha-vip-1.0/ha-vip-1.0-x86_64.tar.gz
tar -xzf ha-vip-1.0-x86_64.tar.gz
cd ha-vip-1.0

# Run automated setup
sudo ./setup_ha_vip.sh
```

For ARM64 systems, use `ha-vip-1.0-arm64.tar.gz` instead.

**📖 Complete Installation Guide**: See [INSTALL.md](./INSTALL.md) for detailed installation instructions, configuration options, and troubleshooting.

### Option 1: Using the pre-built binary

```bash
# Download the binary
wget https://github.com/2bleere/ha-vip/releases/download/ha-vip-1.0/ha-vip-1.0-x86_64.tar.gz
tar -xzf ha-vip-1.0-x86_64.tar.gz
cd ha-vip-1.0

# Copy files
sudo cp ha-vip /usr/local/bin/ha-vip
sudo cp ha-vip.service /etc/systemd/system/
sudo mkdir -p /etc/ha-vip
sudo cp config.yaml /etc/ha-vip/
sudo cp *.pem /etc/ha-vip/  # If using TLS

# Enable and start the service
sudo systemctl enable ha-vip
sudo systemctl start ha-vip
```

### Option 2: Using the setup script

```bash
wget https://example.com/releases/setup_ha_vip.sh
chmod +x setup_ha_vip.sh
sudo ./setup_ha_vip.sh
```

### Option 3: Building from source

```bash
git clone https://github.com/example/ha-vip.git
cd ha-vip
go build -o ha-vip .

# Or cross-compile for ARM64
GOOS=linux GOARCH=arm64 go build -o ha-vip-linux-arm64 .
```

## Configuration

Create a config file at `/etc/ha-vip/config.yaml`:

```yaml
node_id: "node1"          # Unique identifier for this node
priority: 1               # Election priority (lower wins)
interface: "eth0"         # Network interface for the VIP
vip: "192.168.1.200/24"   # Virtual IP with CIDR notation
peers:                    # Other nodes in the cluster
  - "192.168.1.201:9999"  # Format: IP:Port
  - "192.168.1.202:9999"
  - "192.168.1.203:9999"
port: 9999                # UDP port for heartbeat communication
heartbeat_interval: 1     # Heartbeat frequency in seconds
election_timeout: 2       # Election frequency in seconds
tls_cert: "cert.pem"      # TLS certificate (optional)
tls_key: "key.pem"        # TLS key (optional)
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `node_id` | Unique identifier for this node | Required |
| `priority` | Election priority (lower number = higher priority) | Required |
| `interface` | Network interface for VIP assignment | Required |
| `vip` | Virtual IP address with CIDR notation | Required |
| `peers` | List of other nodes in the format `IP:Port` | Required |
| `port` | UDP port for heartbeat communication | Required |
| `heartbeat_interval` | Seconds between heartbeats | 1 |
| `election_timeout` | Seconds between leadership evaluations | 2 |
| `tls_cert` | Path to TLS certificate | Optional |
| `tls_key` | Path to TLS key | Optional |

## Systemd Service

Create a systemd service file at `/etc/systemd/system/ha-vip.service`:

```ini
[Unit]
Description=HA VIP Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ha-vip
Restart=on-failure
RestartSec=5
WorkingDirectory=/etc/ha-vip

[Install]
WantedBy=multi-user.target
```

## Running Without Root

To run as a non-root user:

1. Give the binary necessary capabilities:
   ```bash
   sudo setcap cap_net_admin=+ep /usr/local/bin/ha-vip
   ```

2. Update the service file:
   ```ini
   [Service]
   User=hauser
   Group=hagroup
   ExecStart=/usr/local/bin/ha-vip
   ```

## Monitoring

### Check Status

```bash
systemctl status ha-vip
```

### View Logs

```bash
journalctl -u ha-vip -f
```

### Expected Log Output

```
2025/06/15 12:34:56 Starting HA VIP Manager for node1
2025/06/15 12:34:56 Current leader: node1
2025/06/15 12:34:56 Successfully assigned VIP: 192.168.1.200/24
```

## Performance Tuning

For low-latency environments, you can optimize for even faster failover by editing `/etc/ha-vip/config.yaml`:

```yaml
heartbeat_interval: 0.5  # 500ms between heartbeats
election_timeout: 1      # 1 second election timeout
```

See `PERFORMANCE_OPTIMIZATIONS.md` for detailed tuning information.

## Troubleshooting

### VIP Not Assigned

1. Check logs: `journalctl -u ha-vip -f`
2. Verify interface exists: `ip a show eth0`
3. Check connectivity to peers: `ping 192.168.1.201`
4. Verify permissions: `systemctl status ha-vip`

### Multiple Leaders

1. Verify all nodes have unique `node_id` values
2. Check for network partitioning
3. Ensure all nodes can communicate with each other

### Service Won't Start

1. Check logs: `journalctl -u ha-vip -e`
2. Verify config file exists and is valid YAML
3. Ensure VIP subnet is correct

## Security Considerations

- By default, heartbeat communication is not encrypted
- For production, place heartbeat traffic on a secure management network
- Use firewall rules to restrict UDP port access to cluster members only

# HA VIP Manager Configuration Reference

## Network Features

### Gratuitous ARP

When HA VIP Manager assigns a virtual IP to a node, it automatically sends gratuitous ARP packets to notify the network of the IP-to-MAC address mapping. This ensures fast failover by updating ARP caches on switches and other network devices.

**How it works:**
1. **Primary Method**: Uses `arping` tool with gratuitous ARP flags (`-A` or `-U`)
2. **Fallback Method**: Sends broadcast ping to trigger ARP learning if `arping` is not available

**Benefits:**
- **Fast Failover**: Network devices immediately learn the new MAC address for the VIP
- **Reduced Downtime**: Eliminates ARP cache timeout delays (typically 5-300 seconds)
- **Automatic**: No manual network configuration required

**Requirements:**
- For optimal performance, install `arping` on control plane nodes:
  ```bash
  # Ubuntu/Debian
  sudo apt-get install iputils-arping
  
  # CentOS/RHEL
  sudo yum install iputils
  ```

**Logging:**
The gratuitous ARP process is logged for troubleshooting:
```
Successfully assigned VIP: 192.168.1.100
Sending gratuitous ARP for VIP 192.168.1.100 on interface eth0
Sent gratuitous ARP using arping for 192.168.1.100
```

## Configuration Options

### Basic Configuration
