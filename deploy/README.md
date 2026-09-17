# Protocol Bridge Deployment Files

This directory contains deployment configurations for various environments.

## Directory Structure

```
deploy/
├── README.md              # This file
├── docker-compose.yml     # Docker Compose for local/production
├── kubernetes/            # Kubernetes manifests
│   ├── deployment.yaml   # Deployment configuration
│   ├── service.yaml      # Service configuration
│   └── configmap.yaml    # ConfigMap template
└── systemd/              # Systemd service files
    └── protocol-bridge.service
```

## Quick Start

### Docker Compose

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f protocol-bridge

# Stop services
docker-compose down
```

### Kubernetes

```bash
# Create namespace
kubectl create namespace protocol-bridge

# Create ConfigMap
kubectl create configmap bridge-config \
  --from-file=config.yaml=config.yaml \
  -n protocol-bridge

# Create secrets
kubectl create secret generic bridge-master-key \
  --from-file=master.key=master.key \
  -n protocol-bridge

kubectl create secret generic bridge-certs \
  --from-file=./certs \
  -n protocol-bridge

# Deploy
kubectl apply -f kubernetes/

# Check status
kubectl get pods -n protocol-bridge

# View logs
kubectl logs -f deployment/protocol-bridge -n protocol-bridge
```

### Systemd

```bash
# Copy service file
sudo cp systemd/protocol-bridge.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start
sudo systemctl enable protocol-bridge
sudo systemctl start protocol-bridge

# Check status
sudo systemctl status protocol-bridge
```

## Configuration

### Required Files

- `config.yaml` - Bridge configuration
- `master.key` - Master key for encrypted values
- `certs/` - TLS certificates

### Environment Variables

- `BRIDGE_MASTER_KEY` - Master key for encryption (optional if master.key exists)

## Monitoring

### Health Checks

- **Liveness:** `http://localhost:8080/health`
- **Readiness:** `http://localhost:8080/ready`
- **Metrics:** `http://localhost:8080/metrics`

### Logs

**Docker:**
```bash
docker-compose logs -f protocol-bridge
```

**Kubernetes:**
```bash
kubectl logs -f deployment/protocol-bridge -n protocol-bridge
```

**Systemd:**
```bash
sudo journalctl -u protocol-bridge -f
```

## Troubleshooting

See [Troubleshooting Guide](../docs/troubleshooting.md) for common issues.

## Security Notes

- Never commit `master.key` or certificates to version control
- Use secrets management (Vault, AWS Secrets Manager, etc.) in production
- Restrict network access to metrics endpoint
- Use TLS for all external connections
- Regularly rotate master keys and certificates


