# Kubernetes Integration Guide

This document describes how to configure HA VIP Manager for Kubernetes integration, including both in-cluster and external deployment scenarios.

## Overview

HA VIP Manager can monitor Kubernetes API server health to make intelligent failover decisions. It supports two authentication modes:

1. **External Configuration** (Primary): When deployed on Kubernetes control plane nodes
2. **In-Cluster Configuration**: When deployed as a Kubernetes pod

## Typical Deployment Architecture

The **primary use case** for HA VIP Manager is to run on Kubernetes control plane nodes to provide high availability for the API server itself:

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Control Plane  │  │  Control Plane  │  │  Control Plane  │
│     Node 1      │  │     Node 2      │  │     Node 3      │
│                 │  │                 │  │                 │
│  ┌───────────┐  │  │  ┌───────────┐  │  │  ┌───────────┐  │
│  │ API Server│  │  │  │ API Server│  │  │  │ API Server│  │
│  │ :6443     │  │  │  │ :6443     │  │  │  │ :6443     │  │
│  └───────────┘  │  │  └───────────┘  │  │  └───────────┘  │
│  ┌───────────┐  │  │  ┌───────────┐  │  │  ┌───────────┐  │
│  │  HA-VIP   │  │  │  │  HA-VIP   │  │  │  │  HA-VIP   │  │
│  │  Manager  │  │  │  │  Manager  │  │  │  │  Manager  │  │
│  └───────────┘  │  │  └───────────┘  │  │  └───────────┘  │
└─────────────────┘  └─────────────────┘  └─────────────────┘
         │                     │                     │
         └─────────────────────┼─────────────────────┘
                               │
                    ┌─────────────────┐
                    │   Virtual IP    │
                    │ 192.168.1.100   │
                    │ (API Server LB) │
                    └─────────────────┘
```

In this setup, HA VIP Manager monitors the local API server health (`127.0.0.1:6443`) and manages the virtual IP assignment to ensure clients always connect to a healthy API server.

## Configuration Options

### Common Configuration

```yaml
k8s:
  enabled: true|false     # Enable/disable Kubernetes integration
  in_cluster: true|false  # Use in-cluster vs external configuration
```

### External Configuration (Primary Use Case)

When `in_cluster: false`, HA VIP Manager connects to the local API server on control plane nodes. This is the **recommended deployment mode** for managing API server high availability.

```yaml
k8s:
  enabled: true
  in_cluster: false
  api_server: "https://127.0.0.1:6443"      # Local API server on control plane
  token: "your-service-account-token"
  ca_cert: "/etc/kubernetes/pki/ca.crt"     # Typically located here on control plane
```

**Requirements for External Deployment:**

1. **Control Plane Access**: Deployed on Kubernetes control plane nodes with local API server access
2. **Service Account Token**: A valid service account token with appropriate permissions
3. **CA Certificate**: (Optional but recommended) The cluster's CA certificate for TLS verification

**Obtaining Service Account Token:**

```bash
# Create service account and role binding (see RBAC example below)
kubectl apply -f rbac.yaml

# Get the token (Kubernetes 1.24+)
kubectl create token ha-vip -n kube-system

# Or for older versions with secret-based tokens
kubectl get secret $(kubectl get serviceaccount ha-vip -n kube-system -o jsonpath='{.secrets[0].name}') -o jsonpath='{.data.token}' | base64 -d
```

### In-Cluster Configuration (Alternative)

When `in_cluster: true`, HA VIP Manager automatically uses the service account credentials mounted in the pod. This mode is useful for specialized deployments where HA VIP runs as a pod within the cluster.

```yaml
k8s:
  enabled: true
  in_cluster: true
  # The following fields are ignored when in_cluster=true:
  api_server: ""   # Auto-discovered from environment
  token: ""        # Auto-discovered from service account
  ca_cert: ""      # Auto-discovered from service account
```

**Requirements for In-Cluster Deployment:**

1. **Service Account**: The pod must have a service account with appropriate permissions
2. **RBAC Permissions**: The service account needs read access to cluster health endpoints

**RBAC Configuration (Required for Both Modes):**

**Example RBAC Configuration:**

```yaml
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
```

**Example Pod Deployment:**

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ha-vip
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: ha-vip
  template:
    metadata:
      labels:
        app: ha-vip
    spec:
      serviceAccountName: ha-vip
      hostNetwork: true
      containers:
      - name: ha-vip
        image: your-registry/ha-vip:latest
        securityContext:
          capabilities:
            add: ["NET_ADMIN"]
        volumeMounts:
        - name: config
          mountPath: /etc/ha-vip
        - name: tls-certs
          mountPath: /etc/ha-vip/certs
      volumes:
      - name: config
        configMap:
          name: ha-vip-config
      - name: tls-certs
        secret:
          secretName: ha-vip-tls
```

## Health Check Behavior

HA VIP Manager uses a two-stage health check process:

1. **TCP Connectivity**: Basic network connectivity to the API server
2. **Readiness Check**: HTTP GET to `/readyz` endpoint (authoritative)

The `/readyz` endpoint is considered authoritative for routing decisions. If `/readyz` returns unhealthy, the node will not be considered for VIP assignment, even if basic connectivity works.

## Security Considerations

### In-Cluster Deployment

- Use minimal RBAC permissions (read-only access to health endpoints)
- Run with a dedicated service account
- Consider using Pod Security Policies or Pod Security Standards
- Use network policies to restrict pod-to-pod communication if needed

### External Deployment

- Secure service account tokens (rotate regularly)
- Use TLS verification with valid CA certificates
- Restrict network access to API server (firewall rules)
- Consider using short-lived tokens or certificate-based authentication

## Troubleshooting

### Common Issues

1. **"Failed to create in-cluster config"**
   - Ensure the pod has a mounted service account
   - Check that the service account exists
   - Verify RBAC permissions

2. **"401 Unauthorized" or "403 Forbidden"**
   - Check RBAC permissions for the service account
   - Verify the token is valid and not expired
   - Ensure access to `/readyz` endpoint

3. **"Connection refused" or "Timeout"**
   - Check network connectivity to API server
   - Verify API server is running and healthy
   - Check firewall rules and network policies

### Debug Commands

```bash
# Check service account and permissions
kubectl auth can-i get nodes --as=system:serviceaccount:kube-system:ha-vip
kubectl auth can-i get /readyz --as=system:serviceaccount:kube-system:ha-vip

# Test API server connectivity
curl -k https://127.0.0.1:6443/readyz

# Check pod logs
kubectl logs -n kube-system -l app=ha-vip
```

## Configuration Examples

See the following example configurations:

- [`config-in-cluster.yaml`](config-in-cluster.yaml) - For in-cluster deployment
- [`config-external.yaml`](config-external.yaml) - For external deployment
- [`config.yaml`](config.yaml) - General example with external config

## Performance Impact

Kubernetes health checking adds minimal overhead:

- Health checks run every 2 seconds by default
- Uses lightweight `/readyz` endpoint
- Includes anti-flapping protection with 5-second stability buffer
- TCP connectivity test is very fast (sub-second)

The health checks are designed to be efficient and not impact cluster performance.
