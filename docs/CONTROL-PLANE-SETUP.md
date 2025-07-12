# Control Plane Deployment Guide

This guide shows how to deploy HA VIP Manager on Kubernetes control plane nodes for API server high availability.

## Quick Setup for Control Plane Nodes

### 1. Create Service Account and RBAC

```bash
# Create RBAC configuration
cat > ha-vip-rbac.yaml << EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ha-vip
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ha-vip-reader
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
- nonResourceURLs: ["/readyz", "/healthz"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ha-vip-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ha-vip-reader
subjects:
- kind: ServiceAccount
  name: ha-vip
  namespace: kube-system
EOF

# Apply the configuration
kubectl apply -f ha-vip-rbac.yaml
```

### 2. Get Service Account Token

```bash
# For Kubernetes 1.24+
TOKEN=$(kubectl create token ha-vip -n kube-system)

# For older versions
TOKEN=$(kubectl get secret $(kubectl get serviceaccount ha-vip -n kube-system -o jsonpath='{.secrets[0].name}') -o jsonpath='{.data.token}' | base64 -d)

echo "Service account token: $TOKEN"
```

### 3. Create Configuration File

```bash
# Create configuration directory
sudo mkdir -p /etc/ha-vip

# Create configuration file
sudo cat > /etc/ha-vip/config.yaml << EOF
# HA VIP Manager Configuration for Control Plane
k8s:
  enabled: true
  in_cluster: false
  api_server: "https://127.0.0.1:6443"
  token: "$TOKEN"
  ca_cert: "/etc/kubernetes/pki/ca.crt"

# Node configuration (customize for each control plane node)
node_id: "control-plane-1"    # Change for each node
priority: 100                 # Higher number = higher priority

# Network configuration
interface: "eth0"              # Network interface for VIP
vip: "192.168.1.100"          # Virtual IP for API server load balancing

# Peer configuration (all control plane nodes)
peers:
  - "192.168.1.10:8080"       # control-plane-1
  - "192.168.1.11:8080"       # control-plane-2  
  - "192.168.1.12:8080"       # control-plane-3
port: 8080

# Timing configuration
heartbeat_interval: 1
election_timeout: 5

# TLS for peer communication (optional but recommended)
tls_cert: "/etc/ha-vip/cert.pem"
tls_key: "/etc/ha-vip/key.pem"
EOF
```

### 4. Install Dependencies (Recommended)

```bash
# Install arping for optimal gratuitous ARP performance
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install -y iputils-arping

# CentOS/RHEL/Rocky Linux
sudo yum install -y iputils
```

### 5. Deploy HA VIP Manager

```bash
# Download and install binary
sudo curl -L https://github.com/2bleere/ha-vip/releases/latest/download/ha-vip-linux-arm64 -o /usr/local/bin/ha-vip
sudo chmod +x /usr/local/bin/ha-vip

# Install systemd service
sudo curl -L https://github.com/2bleere/ha-vip/releases/latest/download/ha-vip.service -o /etc/systemd/system/ha-vip.service

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable ha-vip
sudo systemctl start ha-vip
```

### 6. Verify Deployment

```bash
# Check service status
sudo systemctl status ha-vip

# Check logs
sudo journalctl -u ha-vip -f

# Test local API server health
curl -k https://127.0.0.1:6443/readyz

# Check if VIP is assigned (on the leader node)
ip addr show dev eth0

# Verify gratuitous ARP capability
which arping && echo "arping available" || echo "arping not found - will use fallback method"
```

## Configuration Notes

- **Node ID**: Must be unique for each control plane node
- **Priority**: Higher numbers have higher priority for VIP assignment
- **VIP**: Should be an IP address that clients will use to connect to the API server
- **Peers**: List all control plane node IPs and ports for heartbeat communication
- **Interface**: The network interface where the VIP will be assigned

## Troubleshooting

### Common Issues

1. **Service fails to start**
   ```bash
   # Check configuration
   sudo ha-vip -config /etc/ha-vip/config.yaml -version
   
   # Validate token
   curl -k -H "Authorization: Bearer $TOKEN" https://127.0.0.1:6443/readyz
   ```

2. **VIP not being assigned**
   ```bash
   # Check election status in logs
   sudo journalctl -u ha-vip -f | grep -i election
   
   # Verify peer connectivity
   nc -zv <peer-ip> 8080
   ```

3. **Permission errors**
   ```bash
   # Verify RBAC permissions
   kubectl auth can-i get /readyz --as=system:serviceaccount:kube-system:ha-vip
   ```

## Load Balancer Configuration

Once HA VIP is running, configure your load balancer or client applications to connect to the VIP address instead of individual control plane node IPs:

```bash
# Example: Update kubeconfig to use VIP
kubectl config set-cluster <cluster-name> --server=https://192.168.1.100:6443
```

This ensures clients always connect to the currently healthy API server.
