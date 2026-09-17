#!/bin/bash

# Install Protocol Bridge as systemd service
# This requires sudo privileges

set -e

echo "🔧 Installing Protocol Bridge as systemd service..."
echo ""

# Check if running as root or with sudo
if [ "$EUID" -ne 0 ]; then 
    echo "Please run with sudo:"
    echo "  sudo ./install-service.sh"
    exit 1
fi

# Install the service file
echo "📝 Installing service file..."
cp deploy/systemd/protocol-bridge.service /etc/systemd/system/

# Reload systemd
echo "🔄 Reloading systemd daemon..."
systemctl daemon-reload

echo ""
echo "✅ Installation complete!"
echo ""
echo "Next steps:"
echo "  sudo systemctl enable protocol-bridge   # Enable auto-start on boot"
echo "  sudo systemctl start protocol-bridge     # Start the service"
echo "  sudo systemctl status protocol-bridge     # Check status"
echo ""
echo "To stop the service:"
echo "  sudo systemctl stop protocol-bridge"
echo ""
echo "To disable auto-start:"
echo "  sudo systemctl disable protocol-bridge"
echo ""
echo "To view logs:"
echo "  sudo journalctl -u protocol-bridge -f    # Live logs"
echo "  tail -f logs/bridge.log                  # File logs"

