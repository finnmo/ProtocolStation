#Requires -RunAsAdministrator
# ProtocolBridge Windows Service Uninstaller

param(
    [string]$BridgeDir = "C:\ProtocolBridge",
    [string]$SvcName   = "ProtocolBridge"
)

$NssmExe = "$BridgeDir\tools\nssm.exe"

Write-Host "Stopping and removing $SvcName service..."

Stop-Service $SvcName -ErrorAction SilentlyContinue
& $NssmExe remove $SvcName confirm

# Remove firewall rules
Remove-NetFirewallRule -DisplayName "ProtocolBridge Modbus"  -ErrorAction SilentlyContinue
Remove-NetFirewallRule -DisplayName "ProtocolBridge MQTT"    -ErrorAction SilentlyContinue
Remove-NetFirewallRule -DisplayName "ProtocolBridge Health"  -ErrorAction SilentlyContinue

Write-Host "✅ $SvcName uninstalled."
