# Protocol Bridge API Documentation

HTTP endpoints exposed by the Protocol Bridge for metrics, health checks, and management.

## Table of Contents

1. [Overview](#overview)
2. [Health Endpoints](#health-endpoints)
3. [Metrics Endpoint](#metrics-endpoint)
4. [Configuration Endpoints](#configuration-endpoints)

---

## Overview

The Protocol Bridge exposes HTTP endpoints on port 8080 by default:

- **Base URL:** `http://localhost:8080`
- **Health check:** `/health` - Liveness probe
- **Readiness check:** `/ready` - Readiness probe
- **Metrics:** `/metrics` - Prometheus metrics

---

## Health Endpoints

### GET /health

Liveness probe for Kubernetes/Docker health checks.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-28T12:00:00Z"
}
```

**Status Codes:**
- `200 OK` - Service is alive
- `503 Service Unavailable` - Service is unhealthy

**Usage in Kubernetes:**

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

---

### GET /ready

Readiness probe for Kubernetes/Docker readiness checks.

**Response:**
```json
{
  "status": "ready",
  "components": {
    "inputs": {
      "people-counter-input": "connected"
    },
    "outputs": {
      "aws-iot-output": "connected"
    },
    "pipelines": {
      "people-counter-pipeline": "running"
    }
  },
  "timestamp": "2025-01-28T12:00:00Z"
}
```

**Component Status:**
- `connected` - Active and working
- `connecting` - Establishing connection
- `disconnected` - Connection lost
- `error` - Error state

**Status Codes:**
- `200 OK` - All components ready
- `503 Service Unavailable` - Not ready

**Usage in Kubernetes:**

```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

---

## Metrics Endpoint

### GET /metrics

Prometheus metrics endpoint.

**Response Format:**
Prometheus exposition format

**Example Output:**

```
# HELP messages_received_total Total number of messages received
# TYPE messages_received_total counter
messages_received_total{input="people-counter-input",pipeline="people-counter-pipeline"} 12345

# HELP messages_sent_total Total number of messages sent
# TYPE messages_sent_total counter
messages_sent_total{output="aws-iot-output",pipeline="people-counter-pipeline"} 12340

# HELP messages_failed_total Total number of failed messages
# TYPE messages_failed_total counter
messages_failed_total{output="aws-iot-output",pipeline="people-counter-pipeline",reason="timeout"} 5

# HELP messages_processing_duration_seconds Message processing duration
# TYPE messages_processing_duration_seconds histogram
messages_processing_duration_seconds_bucket{pipeline="people-counter-pipeline",le="0.001"} 1000
messages_processing_duration_seconds_bucket{pipeline="people-counter-pipeline",le="0.005"} 5000
messages_processing_duration_seconds_bucket{pipeline="people-counter-pipeline",le="0.01"} 8000
messages_processing_duration_seconds_sum{pipeline="people-counter-pipeline"} 75.5
messages_processing_duration_seconds_count{pipeline="people-counter-pipeline"} 12345

# HELP connections_active Number of active connections
# TYPE connections_active gauge
connections_active{type="input",name="people-counter-input"} 1
connections_active{type="output",name="aws-iot-output"} 1

# HELP buffer_size Current message buffer size
# TYPE buffer_size gauge
buffer_size{pipeline="people-counter-pipeline"} 0

# HELP goroutines_active Number of active goroutines
# TYPE goroutines_active gauge
goroutines_active 42
```

**Metrics:**
- `messages_received_total` - Counter
- `messages_transformed_total` - Counter
- `messages_sent_total` - Counter
- `messages_failed_total` - Counter
- `messages_dlq_total` - Counter
- `message_processing_duration_seconds` - Histogram
- `transformation_duration_seconds` - Histogram
- `output_publish_duration_seconds` - Histogram
- `connections_active` - Gauge
- `buffer_size` - Gauge
- `goroutines_active` - Gauge

**Usage with Prometheus:**

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'protocol-bridge'
    static_configs:
      - targets: ['bridge:8080']
```

---

## Configuration Endpoints

### POST /config/reload

> **Note:** This endpoint is planned for future implementation.

Hot reload configuration without restarting the bridge.

**Request:**
```
POST /config/reload
Content-Type: application/json

{
  "config_path": "/app/config.yaml"
}
```

**Response:**
```json
{
  "status": "reloaded",
  "timestamp": "2025-01-28T12:00:00Z",
  "changes": {
    "pipelines_updated": 2,
    "inputs_updated": 1,
    "outputs_updated": 1
  }
}
```

**Status Codes:**
- `200 OK` - Configuration reloaded successfully
- `400 Bad Request` - Invalid configuration
- `500 Internal Server Error` - Reload failed

---

### GET /config/status

Get current configuration status.

**Response:**
```json
{
  "active_config": {
    "inputs": 1,
    "transformers": 1,
    "outputs": 1,
    "pipelines": 1
  },
  "last_loaded": "2025-01-28T12:00:00Z",
  "config_path": "/app/config.yaml"
}
```

---

### PUT /log/level

> **Note:** This endpoint is planned for future implementation.

Dynamically change log level.

**Request:**
```
PUT /log/level
Content-Type: application/json

{
  "level": "debug"
}
```

**Response:**
```json
{
  "previous_level": "info",
  "new_level": "debug",
  "timestamp": "2025-01-28T12:00:00Z"
}
```

**Valid levels:**
- `debug` - Most verbose
- `info` - Default
- `warn` - Warnings only
- `error` - Errors only

---

## Example Usage

### Health Check Script

```bash
#!/bin/bash

# Check if bridge is healthy
if curl -f http://localhost:8080/health > /dev/null 2>&1; then
  echo "Bridge is healthy"
  exit 0
else
  echo "Bridge is unhealthy"
  exit 1
fi
```

### Readiness Check Script

```bash
#!/bin/bash

# Wait for bridge to be ready
for i in {1..30}; do
  if curl -f http://localhost:8080/ready > /dev/null 2>&1; then
    echo "Bridge is ready"
    exit 0
  fi
  sleep 1
done

echo "Bridge failed to become ready"
exit 1
```

### Metrics Collection

```bash
# Fetch metrics
curl http://localhost:8080/metrics > metrics.txt

# Query specific metric
curl http://localhost:8080/metrics | grep "messages_received_total"
```

---

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": "error message",
  "code": "ERROR_CODE",
  "timestamp": "2025-01-28T12:00:00Z"
}
```

**Status Codes:**
- `200 OK` - Request successful
- `400 Bad Request` - Invalid request
- `404 Not Found` - Endpoint not found
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service unavailable

---

## Security Considerations

### Authentication

> **Note:** Authentication is planned for future implementation.

Currently, endpoints are unprotected. In production:

1. **Use Kubernetes network policies** to restrict access
2. **Use service mesh** (Istio, Linkerd) for security
3. **Deploy behind reverse proxy** with authentication
4. **Bind to localhost** in non-containerized deployments

### TLS/HTTPS

For production deployments, use TLS:

```nginx
server {
  listen 443 ssl;
  server_name bridge.example.com;
  
  ssl_certificate /path/to/cert.pem;
  ssl_certificate_key /path/to/key.pem;
  
  location / {
    proxy_pass http://bridge:8080;
  }
}
```

---

## Integration Examples

### Grafana Dashboard

Create dashboard with metrics:

- **Messages/sec** - Rate of `messages_received_total`
- **Processing Time** - `message_processing_duration_seconds`
- **Error Rate** - Rate of `messages_failed_total`
- **Active Connections** - `connections_active`

### Alerting Rules

```yaml
groups:
  - name: protocol_bridge
    rules:
      - alert: HighFailureRate
        expr: rate(messages_failed_total[5m]) > 0.1
        for: 5m
        annotations:
          summary: "High message failure rate"
          
      - alert: ConnectionLost
        expr: connections_active == 0
        for: 1m
        annotations:
          summary: "All connections lost"
```

---

## Next Steps

- See [Deployment Guide](deployment.md) for installation
- See [Troubleshooting Guide](troubleshooting.md) for common issues
- See [README](../README.md) for usage instructions


