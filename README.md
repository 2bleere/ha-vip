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

## System Requirements

- Linux (ARM64 or AMD64)
- Root privileges (for IP assignment)
- Network connectivity between all nodes
- Shared subnet for the Virtual IP

## Installation

### Option 1: Using the pre-built binary

```bash
# Download the binary
wget https://example.com/releases/ha-vip-linux-arm64.tar.gz
tar -xzvf ha-vip-linux-arm64.tar.gz
cd ha-vip

# Copy files
sudo cp ha-vip-linux-arm64 /usr/local/bin/ha-vip
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

## License

MIT License - See LICENSE file for details

## Contributing

Contributions welcome! Please see CONTRIBUTING.md for details.
