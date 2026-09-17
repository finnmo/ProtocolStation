# Protocol Bridge Restart Guide

## Quick Restart (Automated)

Use the provided script:

```bash
./restart-bridge.sh
```

This script will:
1. Stop the service
2. Rebuild the binary
3. Set capabilities for port 502
4. Restart the service
5. Show status

## Manual Restart Steps

### 1. Stop the Bridge Service

```bash
sudo systemctl stop protocol-bridge
```

### 2. Rebuild the Binary

```bash
cd /home/optech/ProtocolBridge
make build
```

Or with capabilities set automatically:

```bash
make build-install
```

### 3. Set Capabilities for Port 502 (Modbus)

The bridge needs `CAP_NET_BIND_SERVICE` to bind to port 502:

```bash
sudo setcap 'cap_net_bind_service=+ep' /home/optech/ProtocolBridge/bridge
```

Verify capabilities are set:

```bash
getcap /home/optech/ProtocolBridge/bridge
```

Expected output:
```
/home/optech/ProtocolBridge/bridge cap_net_bind_service=ep
```

### 4. Reload Systemd and Restart

```bash
sudo systemctl daemon-reload
sudo systemctl start protocol-bridge
```

### 5. Verify Service Status

```bash
sudo systemctl status protocol-bridge
```

### 6. Check Logs

```bash
tail -f logs/bridge.log
```

## Alternative: Update Systemd Service for Capabilities

If you prefer to grant capabilities through systemd instead of setcap:

1. Edit the service file:
```bash
sudo nano /etc/systemd/system/protocol-bridge.service
```

2. Update the `[Service]` section:
```ini
[Service]
Type=notify
NotifyAccess=all
User=optech
Group=optech
WorkingDirectory=/home/optech/ProtocolBridge
ExecStart=/home/optech/ProtocolBridge/bridge -config /home/optech/ProtocolBridge/config.yaml
Restart=always
RestartSec=5

# Security settings - Modified for port 502 capability
NoNewPrivileges=false
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
PrivateTmp=true

# ... rest of config ...
```

3. Reload and restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart protocol-bridge
```

## Troubleshooting

### Service Won't Start

Check logs:
```bash
sudo journalctl -u protocol-bridge -n 50 --no-pager
tail -50 logs/bridge.log
```

### Port 502 Permission Denied

If you see "permission denied" errors for port 502:

1. Verify capabilities:
```bash
getcap /home/optech/ProtocolBridge/bridge
```

2. If not set, run:
```bash
sudo setcap 'cap_net_bind_service=+ep' /home/optech/ProtocolBridge/bridge
```

3. If using systemd, ensure `NoNewPrivileges=false` in the service file

### Service Keeps Restarting

Check for errors:
```bash
sudo systemctl status protocol-bridge -l
journalctl -u protocol-bridge -f
```

## Useful Commands

```bash
# View live logs
tail -f logs/bridge.log

# Check service status
sudo systemctl status protocol-bridge

# Restart service (without rebuild)
sudo systemctl restart protocol-bridge

# Stop service
sudo systemctl stop protocol-bridge

# Start service
sudo systemctl start protocol-bridge

# View metrics
curl http://localhost:8080/metrics

# Health check
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready

# Status endpoint
curl http://localhost:8080/status | jq .
```
