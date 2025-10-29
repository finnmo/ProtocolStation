# Protocol Bridge Deployment Guide

This guide covers deploying the Protocol Bridge in various environments: Docker, Kubernetes, and systemd.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Docker Deployment](#docker-deployment)
3. [Kubernetes Deployment](#kubernetes-deployment)
4. [Systemd Service](#systemd-service)
5. [Configuration Management](#configuration-management)
6. [TLS Certificate Management](#tls-certificate-management)
7. [Monitoring Setup](#monitoring-setup)

---

## Prerequisites

- Go 1.24 or later (for building from source)
- Docker and Docker Compose (for containerized deployment)
- Kubernetes cluster (for K8s deployment)
- Systemd (for service deployment)
- Access to MQTT brokers
- TLS certificates (if using secure connections)

---

## Docker Deployment

### Quick Start

1. **Build the Docker image:**

```bash
cd /home/optech/ProtocolBridge
docker build -t protocol-bridge:latest .
```

2. **Create configuration file:**

Copy the example config and customize:

```bash
cp config.example.yaml config.yaml
vim config.yaml  # Edit your settings
```

3. **Create master key (for encrypted config values):**

```bash
echo "your-secure-master-key" > master.key
chmod 600 master.key
```

4. **Run with Docker Compose:**

```bash
docker-compose up -d
```

### Production Docker Setup

The `deploy/docker-compose.yml` includes:

- Bridge service with health checks
- Volume mounts for config and certificates
- Network isolation
- Resource limits
- Restart policies

**Example docker-compose.yml:**

```yaml
version: '3.8'

services:
  protocol-bridge:
    image: protocol-bridge:latest
    container_name: protocol-bridge
    restart: unless-stopped
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./certs:/app/certs:ro
      - ./master.key:/app/master.key:ro
    environment:
      - BRIDGE_MASTER_KEY=${BRIDGE_MASTER_KEY}
    ports:
      - "8080:8080"  # Metrics and health endpoints
    networks:
      - bridge-network
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    resources:
      limits:
        cpus: '2'
        memory: 512M
      reservations:
        cpus: '0.5'
        memory: 256M

networks:
  bridge-network:
    driver: bridge
```

### Building the Image

**Dockerfile:**

```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bridge ./cmd/bridge

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /build/bridge .
COPY --from=builder /build/config.yaml ./config.yaml

EXPOSE 8080

CMD ["./bridge", "-config", "config.yaml"]
```

---

## Kubernetes Deployment

### 1. Create Namespace

```bash
kubectl create namespace protocol-bridge
```

### 2. Create ConfigMap

```bash
kubectl create configmap bridge-config \
  --from-file=config.yaml=config.yaml \
  -n protocol-bridge
```

### 3. Create Secrets

**Master key:**

```bash
kubectl create secret generic bridge-master-key \
  --from-file=master.key=master.key \
  -n protocol-bridge
```

**TLS certificates:**

```bash
kubectl create secret generic bridge-certs \
  --from-file=./certs \
  -n protocol-bridge
```

### 4. Create Deployment

**deploy/kubernetes/deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: protocol-bridge
  namespace: protocol-bridge
  labels:
    app: protocol-bridge
spec:
  replicas: 2
  selector:
    matchLabels:
      app: protocol-bridge
  template:
    metadata:
      labels:
        app: protocol-bridge
    spec:
      containers:
      - name: bridge
        image: protocol-bridge:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: http
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
          readOnly: true
        - name: master-key
          mountPath: /app/master.key
          subPath: master.key
          readOnly: true
        - name: certs
          mountPath: /app/certs
          readOnly: true
        env:
        - name: BRIDGE_MASTER_KEY
          valueFrom:
            secretKeyRef:
              name: bridge-master-key
              key: master.key
        resources:
          requests:
            cpu: 500m
            memory: 256Mi
          limits:
            cpu: 2000m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: bridge-config
      - name: master-key
        secret:
          secretName: bridge-master-key
      - name: certs
        secret:
          secretName: bridge-certs
```

### 5. Create Service

**deploy/kubernetes/service.yaml:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: protocol-bridge
  namespace: protocol-bridge
  labels:
    app: protocol-bridge
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: protocol-bridge
```

### 6. Deploy

```bash
kubectl apply -f deploy/kubernetes/
```

---

## Systemd Service

### Installation

1. **Copy files:**

```bash
sudo cp bridge /usr/local/bin/
sudo cp deploy/systemd/protocol-bridge.service /etc/systemd/system/
sudo mkdir -p /etc/protocol-bridge
```

2. **Create configuration:**

```bash
sudo cp config.yaml /etc/protocol-bridge/
sudo mkdir -p /etc/protocol-bridge/certs
sudo cp -r certs/* /etc/protocol-bridge/certs/
```

3. **Set permissions:**

```bash
sudo chmod 600 /etc/protocol-bridge/config.yaml
sudo chmod 600 /etc/protocol-bridge/certs/*
```

4. **Enable and start:**

```bash
sudo systemctl daemon-reload
sudo systemctl enable protocol-bridge
sudo systemctl start protocol-bridge
sudo systemctl status protocol-bridge
```

### Service File

**deploy/systemd/protocol-bridge.service:**

```ini
[Unit]
Description=Protocol Bridge
After=network.target

[Service]
Type=simple
User=bridge
Group=bridge
WorkingDirectory=/etc/protocol-bridge
ExecStart=/usr/local/bin/bridge -config /etc/protocol-bridge/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/etc/protocol-bridge

[Install]
WantedBy=multi-user.target
```

### Logs

```bash
# View logs
sudo journalctl -u protocol-bridge -f

# View last 100 lines
sudo journalctl -u protocol-bridge -n 100
```

---

## Configuration Management

### Best Practices

1. **Version Control:**
   - Keep config templates in Git (without secrets)
   - Use environment-specific configs (dev/staging/prod)
   - Never commit master.key or certificates

2. **Secret Management:**
   - Encrypt sensitive values using `encrypt-value` tool
   - Store master keys securely (HashiCorp Vault, AWS Secrets Manager)
   - Rotate master keys regularly

3. **Configuration Updates:**
   - Test config changes in staging first
   - Use hot reload (when implemented) for zero-downtime updates
   - Keep backup of previous working config

### Example Encrypted Config

```yaml
inputs:
  - name: secure-input
    username: "admin"
    password: "ENC:dGVzdA..."  # Encrypted password

outputs:
  - name: secure-output
    username: "user"
    password: "ENC:dGVzdA..."  # Encrypted password
```

---

## TLS Certificate Management

### Organization Structure

```
certs/
├── input/
│   ├── aws-iot-ap-southeast-2/
│   │   ├── certificate.pem.crt
│   │   ├── private.pem.key
│   │   └── ca.pem
│   └── local-broker/
│       └── ca.pem
└── output/
    ├── aws-iot-ap-southeast-2/
    │   ├── certificate.pem.crt
    │   ├── private.pem.key
    │   └── ca.pem
    └── other-brokers/
        └── ca.pem
```

### Certificate Rotation

1. **Place new certificates in temporary directory:**

```bash
cp new-certificates/* certs/output/aws-iot-ap-southeast-2/
```

2. **Reload configuration (when hot reload is implemented):**

```bash
curl -X POST http://localhost:8080/config/reload
```

3. **Verify connections:**

```bash
curl http://localhost:8080/health
```

---

## Monitoring Setup

### Prometheus Configuration

**prometheus.yml:**

```yaml
scrape_configs:
  - job_name: 'protocol-bridge'
    static_configs:
      - targets: ['bridge:8080']
    metrics_path: '/metrics'
```

### Grafana Dashboard

Import the dashboard configuration from `deploy/grafana/dashboard.json`:

1. Open Grafana
2. Import dashboard
3. Select Prometheus as data source
4. Apply dashboard

**Key Metrics to Monitor:**
- `messages_received_total` - Incoming message rate
- `messages_sent_total` - Outgoing message rate
- `messages_failed_total` - Error rate
- `message_processing_duration_seconds` - Processing latency
- `connections_active` - Active connections
- `buffer_size` - Message buffer utilization

### Critical Reliability Metrics

**Long-term Operation Monitoring:**
- `cert_expiration_days` - Days until TLS certificate expiration (critical alerts at 30/7/1 days)
- `disk_free_bytes` / `disk_used_percent` - Disk space monitoring
- `dlq_size_bytes` - Dead letter queue size tracking
- `server_restarts_total` - Server health and auto-recovery tracking
- `goroutines_active` - Goroutine leak detection

### Alerting Rules

**alerts.yml:**

```yaml
groups:
  - name: protocol_bridge
    rules:
      - alert: HighMessageFailureRate
        expr: rate(messages_failed_total[5m]) > 10
        for: 5m
        
      - alert: ConnectionLost
        expr: connections_active == 0
        for: 2m

  # Reliability monitoring alerts
  - name: protocol_bridge_reliability
    rules:
      # Certificate expiration
      - alert: CertificateExpiringSoon
        expr: cert_expiration_days < 7
        for: 1h
        annotations:
          summary: "Certificate expiring within 7 days"
          
      - alert: CertificateExpiringCritical
        expr: cert_expiration_days < 1
        for: 1h
        severity: critical
        annotations:
          summary: "Certificate expiring within 24 hours - ACTION REQUIRED"
          
      # Disk space
      - alert: LowDiskSpace
        expr: disk_used_percent > 90
        for: 5m
        annotations:
          summary: "Disk usage above 90%"
          
      - alert: CriticalDiskSpace
        expr: disk_used_percent > 95
        for: 1m
        severity: critical
        annotations:
          summary: "Disk usage above 95% - immediate action required"
          
      # DLQ monitoring
      - alert: DLQSizeGrowing
        expr: dlq_size_bytes > 52428800  # 50MB
        for: 5m
        annotations:
          summary: "DLQ size exceeds 50MB - investigate failed messages"
          
      # Server health
      - alert: ServerRestarts
        expr: rate(server_restarts_total[5m]) > 0
        for: 5m
        annotations:
          summary: "Servers are restarting frequently - check logs"
```

---

## Next Steps

- See [Troubleshooting Guide](troubleshooting.md) for common issues
- See [API Documentation](api.md) for endpoints
- See [README](../README.md) for usage instructions


