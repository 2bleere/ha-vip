# Non-Root Deployment Guide

This guide explains how to run HA VIP Manager as a non-root user and the different capabilities/permissions required.

## Permission Requirements

HA VIP Manager requires certain network privileges to function:

1. **VIP Assignment**: Ability to add/remove IP addresses from network interfaces
2. **Gratuitous ARP**: Ability to send ARP packets for fast failover

## Deployment Options

### Option 1: Root User (Full Privileges)

**Pros:**
- All features work without additional configuration
- Fastest gratuitous ARP using `arping`
- Simple deployment

**Cons:**
- Security implications of running as root

```bash
# Standard root deployment
sudo systemctl start ha-vip
```

### Option 2: Non-Root with Capabilities

**Pros:**
- Better security than root
- Most features work
- Can use `arping` with proper capabilities

**Cons:**
- Requires capability configuration

```bash
# Grant network admin capability to the binary
sudo setcap 'cap_net_admin=+ep' /usr/local/bin/ha-vip

# Run as non-root user
systemctl --user start ha-vip
```

### Option 3: Non-Root Fallback Mode (Current Implementation)

**Pros:**
- Works without special privileges
- Secure execution
- Automatic fallback methods

**Cons:**
- Slower network convergence (still fast, but not optimal)
- Uses alternative ARP announcement methods

## Gratuitous ARP Methods by Permission Level

### Root User
1. `arping -A` (gratuitous announce) ✅
2. `arping -U` (unsolicited) ✅  
3. All network manipulation ✅

### Non-Root with CAP_NET_ADMIN
1. `arping -A` (gratuitous announce) ✅
2. `arping -U` (unsolicited) ✅
3. `ip neigh` manipulation ✅
4. VIP assignment ✅

### Non-Root without Capabilities (Auto-detected)
1. ~~`arping` attempts skipped~~ (optimized - no permission errors)
2. Self-ping to establish ARP ✅
3. Broadcast ping ✅
4. Gateway ping ✅
5. VIP assignment ❌ (requires sudo/capabilities)

**Note**: The application automatically detects non-root execution at startup and skips `arping` attempts to avoid permission errors and improve efficiency.

## Setting Up Non-Root with Capabilities

### 1. Create Service User

```bash
# Create dedicated user for ha-vip
sudo useradd -r -s /bin/false -M ha-vip

# Create configuration directory
sudo mkdir -p /etc/ha-vip
sudo chown ha-vip:ha-vip /etc/ha-vip
```

### 2. Set Binary Capabilities

```bash
# Grant network admin capability
sudo setcap 'cap_net_admin=+ep' /usr/local/bin/ha-vip

# Verify capabilities
getcap /usr/local/bin/ha-vip
```

### 3. Update Systemd Service

```ini
[Unit]
Description=HA VIP Manager (Non-root)
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ha-vip
WorkingDirectory=/etc/ha-vip
Restart=always
RestartSec=5
User=ha-vip
Group=ha-vip

# Additional security restrictions
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/etc/ha-vip

[Install]
WantedBy=multi-user.target
```

## Troubleshooting Non-Root Issues

### Common Error Messages

1. **"operation not permitted" with arping**
   ```bash
   # Check if capabilities are set
   getcap /usr/local/bin/ha-vip
   
   # Set capabilities if missing
   sudo setcap 'cap_net_admin=+ep' /usr/local/bin/ha-vip
   ```

2. **"Failed to assign VIP"**
   ```bash
   # VIP assignment requires CAP_NET_ADMIN
   sudo setcap 'cap_net_admin=+ep' /usr/local/bin/ha-vip
   ```

3. **"Using fallback methods for ARP announcement"**
   ```bash
   # This is normal for non-privileged mode and indicates efficient operation
   # The application detected non-root execution and skipped arping attempts
   # Check logs for specific methods being used:
   sudo journalctl -u ha-vip -f | grep -i arp
   ```

### Verification Commands

```bash
# Check if running as non-root
ps aux | grep ha-vip

# Check capabilities
getcap /usr/local/bin/ha-vip

# Test VIP assignment manually
sudo -u ha-vip /usr/local/bin/ha-vip -test-vip

# Check network permissions
sudo -u ha-vip ip addr show
```

## Security Considerations

### Capabilities vs Root

**CAP_NET_ADMIN** allows:
- Network interface configuration
- ARP table manipulation
- Routing table changes
- Network namespace operations

**NOT allowed:**
- File system access outside working directory
- Process manipulation
- System-wide configuration changes

### Recommended Security Setup

```bash
# 1. Use dedicated user
sudo useradd -r -s /bin/false ha-vip

# 2. Restrict file permissions
sudo chmod 600 /etc/ha-vip/config.yaml
sudo chown ha-vip:ha-vip /etc/ha-vip/config.yaml

# 3. Use systemd security features
# (see systemd service example above)

# 4. Set minimal capabilities
sudo setcap 'cap_net_admin=+ep' /usr/local/bin/ha-vip
```

## Performance Comparison

| Method | Root | CAP_NET_ADMIN | Non-Privileged |
|--------|------|---------------|----------------|
| VIP Assignment | ✅ Fast | ✅ Fast | ❌ Requires sudo |
| Gratuitous ARP | ✅ Optimal | ✅ Optimal | ⚠️ Fallback |
| Network Convergence | < 1s | < 1s | 1-3s |
| Security | ❌ High Risk | ⚠️ Medium | ✅ Low Risk |

## Recommendation

For production deployments, use **non-root with CAP_NET_ADMIN**:
- Good security posture
- Full functionality
- Fast network convergence
- Easy to audit and maintain
