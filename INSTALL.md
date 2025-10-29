# Installing Protocol Bridge as a Systemd Service

To make the bridge automatically start on boot and restart on failure:

## Installation

1. **Stop any running manual instance**:
```bash
pkill -f "./bridge"
```

2. **Install the systemd service**:
```bash
sudo ./install-service.sh
```

Or manually:
```bash
sudo cp deploy/systemd/protocol-bridge.service /etc/systemd/system/
sudo systemctl daemon-reload
```

3. **Enable auto-start on boot**:
```bash
sudo systemctl enable protocol-bridge
```

4. **Start the service**:
```bash
sudo systemctl start protocol-bridge
```

5. **Check status**:
```bash
sudo systemctl status protocol-bridge
```

## Managing the Service

### Start
```bash
sudo systemctl start protocol-bridge
```

### Stop
```bash
sudo systemctl stop protocol-bridge
```

### Restart
```bash
sudo systemctl restart protocol-bridge
```

### Status
```bash
sudo systemctl status protocol-bridge
```

### View Logs
```bash
# Live logs
tail -f logs/bridge.log

# Or through journal
sudo journalctl -u protocol-bridge -f
```

## Disabling the Service

To disable auto-start but keep the service installed:
```bash
sudo systemctl disable protocol-bridge
```

To completely remove:
```bash
sudo systemctl disable protocol-bridge
sudo rm /etc/systemd/system/protocol-bridge.service
sudo systemctl daemon-reload
```

## What Happens on Boot

Once enabled, the bridge will:
- ✅ Start automatically on system boot
- ✅ Restart automatically if it crashes (Restart=always)
- ✅ Start after Docker is ready
- ✅ Run health checks every 60s
- ✅ Auto-recover failed Docker services

## Current Manual Deployment

If you prefer to run it manually (current setup):
```bash
./start-bridge.sh
```

This is working perfectly, but requires you to manually start it after restarts.

