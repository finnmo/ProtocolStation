# Protocol Bridge Deployment Guide

## Current Deployment Method

The bridge runs as the **systemd service** `protocol-bridge` on the `optech` account.

```bash
# Quick-rebuild and restart (builds, setcap, restarts service)
./restart-bridge.sh

# Manual rebuild cycle
make build
sudo setcap 'cap_net_bind_service=+ep' ./bridge
sudo systemctl restart protocol-bridge
```

### Monitor the Bridge

```bash
# View live logs (bridge writes to file, not journald)
tail -f logs/bridge.log

# Service status
sudo systemctl status protocol-bridge

# Health / readiness / pipeline counters
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/status | jq .

# Prometheus metrics
curl http://localhost:8080/metrics
```

### Manual Start (development / fallback)

```bash
./start-bridge.sh
```

---

## Systemd Service

To run as a systemd service that auto-starts on boot:

### 1. Install the Service

```bash
# This requires sudo privileges
sudo ./install-service.sh
```

Or manually:
```bash
sudo cp deploy/systemd/protocol-bridge.service /etc/systemd/system/
sudo systemctl daemon-reload
```

### 2. Enable Auto-Start on Boot

```bash
sudo systemctl enable protocol-bridge
```

### 3. Start the Service

```bash
sudo systemctl start protocol-bridge
```

### 4. Check Status

```bash
sudo systemctl status protocol-bridge
```

### 5. View Logs

```bash
# System journal (if configured)
sudo journalctl -u protocol-bridge -f

# Or from file
tail -f logs/bridge.log
```

### 6. Stop/Disable

```bash
# Stop the service
sudo systemctl stop protocol-bridge

# Disable auto-start (but keep service installed)
sudo systemctl disable protocol-bridge

# Remove the service completely
sudo systemctl disable protocol-bridge
sudo rm /etc/systemd/system/protocol-bridge.service
sudo systemctl daemon-reload
```

---

## Current Configuration

- **User**: optech
- **Working Directory**: /home/optech/ProtocolBridge
- **Config File**: /home/optech/ProtocolBridge/config.yaml
- **Log Location**: /home/optech/ProtocolBridge/logs/bridge.log
- **Persistence File**: /home/optech/ProtocolBridge/register_values.json

---

## Reliability Features

The bridge includes automatic features for long-term unattended operation:

- **Log Rotation**: Automatic rotation at 100MB, kept for 7 days
- **Modbus Cleanup**: Entries older than 30 days are automatically removed
- **Certificate Monitoring**: Alerts when TLS certificates expire (30/7/1 days)
- **Auto-Recovery**: Docker-managed servers automatically restart on failure
- **Disk Space Protection**: Prevents writes when disk is critically low
- **DLQ Size Limits**: Prevents unbounded DLQ growth

---

## Deployment Options Comparison

| Feature | Manual Start | Systemd Service |
|---------|--------------|-----------------|
| Auto-start on boot | ❌ | ✅ |
| Requires sudo | ❌ | ✅ |
| Easy start/stop | ✅ | ✅ |
| Production-ready | ⚠️ | ✅ |
| Log rotation | ✅ | ✅ |
| Auto-recovery | ✅ | ✅ |
| Current setup | fallback only | ✅ (primary) |

---

## Troubleshooting

See `docs/troubleshooting.md` for common issues and solutions.

