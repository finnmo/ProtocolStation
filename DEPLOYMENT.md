# Protocol Bridge Deployment Guide

## Current Deployment Method

### Manual Start (Current)
The bridge is currently deployed using the manual start script:

```bash
./start-bridge.sh
```

This will:
- Stop any existing bridge instances
- Start the MQTT server (if needed)
- Start the Modbus server
- Launch the bridge in the background

### Monitor the Bridge

```bash
# View live logs
tail -f logs/bridge.log

# Check if bridge is running
ps aux | grep "bridge -config"

# View metrics
curl http://localhost:8080/metrics

# Check health
curl http://localhost:8080/health

# View status
curl http://localhost:8080/status | jq .
```

### Stop the Bridge

```bash
pkill -f "./bridge"
docker-compose -f mqtt/server/docker-compose.yml down
```

---

## Production Deployment (Systemd Service)

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
- **Persistence File**: /home/optech/ProtocolBridge/modbus_registers.json

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
| Current setup | ✅ | ⏳ |

---

## Troubleshooting

See `docs/troubleshooting.md` for common issues and solutions.

