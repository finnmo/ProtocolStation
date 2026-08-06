#Requires -RunAsAdministrator
# ProtocolBridge Windows Service Installer
# Run from an elevated PowerShell prompt: .\install-service.ps1

param(
    [string]$BridgeDir = "C:\ProtocolBridge",
    [string]$SvcName   = "ProtocolBridge"
)

$NssmExe = "$BridgeDir\tools\nssm.exe"

# Validate pre-requisites
if (-not (Test-Path "$BridgeDir\bridge.exe")) {
    Write-Error "bridge.exe not found at $BridgeDir\bridge.exe. Copy the binary first."
    exit 1
}
if (-not (Test-Path $NssmExe)) {
    Write-Error "nssm.exe not found at $NssmExe. Download from https://nssm.cc and place it there."
    exit 1
}
if (-not (Test-Path "$BridgeDir\config.yaml")) {
    Write-Error "config.yaml not found at $BridgeDir\config.yaml. Copy and configure it first."
    exit 1
}

# Create logs directory
New-Item -ItemType Directory -Force -Path "$BridgeDir\logs" | Out-Null

Write-Host "Installing $SvcName service..."

# Install and configure the service via NSSM
& $NssmExe install      $SvcName "$BridgeDir\bridge.exe"
& $NssmExe set          $SvcName AppParameters    "-config `"$BridgeDir\config.yaml`""
& $NssmExe set          $SvcName AppDirectory     $BridgeDir
& $NssmExe set          $SvcName Start            SERVICE_AUTO_START
& $NssmExe set          $SvcName AppStdout        "$BridgeDir\logs\bridge.log"
& $NssmExe set          $SvcName AppStderr        "$BridgeDir\logs\bridge.log"
& $NssmExe set          $SvcName AppRotateFiles   1
& $NssmExe set          $SvcName AppRotateOnline  1
& $NssmExe set          $SvcName AppRotateBytes   104857600   # 100 MB
& $NssmExe set          $SvcName AppRestartDelay  5000        # 5s restart delay (matches Linux RestartSec=5)
& $NssmExe set          $SvcName AppKillProcessTree 1
& $NssmExe set          $SvcName DisplayName      "Protocol Bridge - IoT Message Protocol Bridge"
& $NssmExe set          $SvcName Description      "Routes IoT messages between MQTT, Modbus, and AWS IoT."

# Firewall rules
Write-Host "Adding Windows Firewall rules..."
New-NetFirewallRule -DisplayName "ProtocolBridge Modbus"  -Direction Inbound -Protocol TCP -LocalPort 502  -Action Allow -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName "ProtocolBridge MQTT"    -Direction Inbound -Protocol TCP -LocalPort 1883 -Action Allow -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName "ProtocolBridge Health"  -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -ErrorAction SilentlyContinue

# Start the service
Write-Host "Starting $SvcName..."
Start-Service $SvcName
Start-Sleep -Seconds 3
$svc = Get-Service $SvcName
Write-Host "Service status: $($svc.Status)"

if ($svc.Status -eq "Running") {
    Write-Host ""
    Write-Host "✅ $SvcName installed and running."
    Write-Host ""
    Write-Host "Useful commands:"
    Write-Host "  Get-Service $SvcName"
    Write-Host "  Stop-Service $SvcName"
    Write-Host "  Restart-Service $SvcName"
    Write-Host "  curl http://localhost:8080/health"
    Write-Host "  curl http://localhost:8080/status"
    Write-Host "  Get-Content $BridgeDir\logs\bridge.log -Tail 50 -Wait"
} else {
    Write-Warning "$SvcName did not reach Running state. Check logs at $BridgeDir\logs\bridge.log"
}
