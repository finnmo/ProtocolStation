# ProtocolBridge — Windows Deployment Guide

## Prerequisites

| Component | Purpose | Source |
|---|---|---|
| `bridge.exe` | Main binary | Cross-compiled (see below) |
| NSSM | Windows service wrapper | https://nssm.cc |
| Mosquitto for Windows | Local MQTT broker | https://mosquitto.org/download/ |
| Go 1.24+ (build machine only) | Cross-compile | https://go.dev |

---

## Step 1 — Build the Windows Binary

Run this on the Linux build machine (or any machine with Go):

```bash
make build-windows
# produces bridge.exe in the repo root
```

---

## Step 2 — Deploy Files to the Windows Server

Create `C:\ProtocolBridge\` and copy:

```
C:\ProtocolBridge\
  bridge.exe
  config.yaml            ← copy from config.example.yaml, fill in real values
  certs\
    input\aws-iot-ap-southeast-2\
      certificate.pem.crt
      private.pem.key
      ca.pem
    output\aws-iot-ap-southeast-2\
      certificate.pem.crt
      private.pem.key
      ca.pem
  tools\
    nssm.exe             ← download from https://nssm.cc
```

The `logs\` directory is created automatically by the installer.

---

## Step 3 — Install Mosquitto (local MQTT broker)

1. Download and run the Mosquitto Windows installer from https://mosquitto.org/download/
2. The installer registers Mosquitto as a Windows service (`mosquitto`) with auto-start.
3. Replace `C:\Program Files\mosquitto\mosquitto.conf` with a Windows-adapted version of `mqtt/server/mosquitto.conf`:
   ```
   allow_anonymous true
   listener 1883
   protocol mqtt
   listener 9001
   protocol websockets
   persistence true
   persistence_location C:\ProtocolBridge\mosquitto_data\
   log_dest file C:\ProtocolBridge\logs\mosquitto.log
   ```
4. Restart the Mosquitto service: `Restart-Service mosquitto`

The bridge's server manager detects that port 1883 is already listening and skips Docker entirely — no Docker required on Windows.

---

## Step 4 — Install ProtocolBridge as a Windows Service

Open PowerShell **as Administrator** and run:

```powershell
cd C:\ProtocolBridge
.\deploy\windows\install-service.ps1
```

This installs and starts the `ProtocolBridge` Windows service with:
- **Auto-start** on boot
- **Automatic restart** after 5 seconds on crash (matches Linux `RestartSec=5`)
- **Log rotation** at 100 MB (same as Linux config)
- **Firewall rules** for ports 502, 1883, and 8080

---

## Step 5 — Verify

```powershell
# Service status
Get-Service ProtocolBridge

# Health check
curl http://localhost:8080/health    # {"status":"healthy"}
curl http://localhost:8080/ready
curl http://localhost:8080/status

# Live logs
Get-Content C:\ProtocolBridge\logs\bridge.log -Tail 50 -Wait
```

---

## Day-to-Day Operations

| Task | Command |
|---|---|
| Start | `Start-Service ProtocolBridge` |
| Stop | `Stop-Service ProtocolBridge` |
| Restart | `Restart-Service ProtocolBridge` |
| Tail logs | `Get-Content C:\ProtocolBridge\logs\bridge.log -Tail 50 -Wait` |
| Deploy new binary | Stop service → overwrite `bridge.exe` → Start service |
| Uninstall | Run `deploy\windows\uninstall-service.ps1` as Administrator |

---

## Port Reference

| Port | Direction | Purpose |
|---|---|---|
| 502 | Inbound | Modbus TCP (PME / SCADA reads registers) |
| 1883 | Inbound | MQTT (cameras publish here) |
| 8080 | Inbound | Health / metrics (firewall to trusted hosts only) |
| 8883 | Outbound | AWS IoT MQTT TLS (allowed by default) |

---

## Fault Recovery

NSSM restarts `bridge.exe` within 5 seconds of any crash. To test:

```powershell
Stop-Process -Name bridge -Force
# Wait ~5 seconds
Get-Service ProtocolBridge   # should show Running
```

The Mosquitto service has its own `restart: unless-stopped` equivalent via the Windows service recovery settings set by the installer.

---

## Updating the Binary

```powershell
Stop-Service ProtocolBridge
# Copy new bridge.exe to C:\ProtocolBridge\bridge.exe
Start-Service ProtocolBridge
```

No `setcap` required — Windows does not use Linux capabilities. Port 502 is accessible because the service runs under the LocalSystem account.
