# HA VIP Manager

High-availability virtual IP management system with Kubernetes integration.

## Project Structure

```
ha-vip/
├── cmd/ha-vip/          # Main application entry point
├── internal/            # Internal packages
│   ├── config/          # Configuration management
│   ├── election/        # Leader election logic
│   ├── heartbeat/       # Peer heartbeat system
│   ├── k8s/            # Kubernetes health checking
│   └── vip/            # VIP management
├── configs/            # Configuration templates
├── deployments/        # Deployment files (systemd, scripts)
├── docs/              # Documentation
├── examples/          # Example configurations
└── scripts/           # Build and utility scripts
```

## Quick Start

For Kubernetes control plane deployment, see [Control Plane Setup](docs/CONTROL-PLANE-SETUP.md).

For general installation, see [Installation Guide](docs/INSTALL.md).

## Features

- High-availability virtual IP failover with gratuitous ARP
- Kubernetes API server health monitoring (in-cluster and external)
- Priority-based leader election with anti-flapping
- Service account and token-based authentication
- Stability controls with 5-second response time
- Fast network convergence with automatic ARP updates
- Systemd service integration

## Documentation

- [Installation Guide](docs/INSTALL.md)
- [Control Plane Setup](docs/CONTROL-PLANE-SETUP.md)
- [Kubernetes Integration](docs/KUBERNETES.md)
- [Configuration Reference](docs/README.md)

## Building

```bash
# Build for current platform
go build -o ha-vip ./cmd/ha-vip

# Cross-compile for ARM64 Linux
GOOS=linux GOARCH=arm64 go build -o ha-vip-linux-arm64 ./cmd/ha-vip
```

## License

MIT License
