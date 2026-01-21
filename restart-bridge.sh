#!/bin/bash

# Protocol Bridge Restart Script
# This script rebuilds, sets permissions, and restarts the bridge service

set -e

cd /home/optech/ProtocolBridge

echo "🛑 Stopping Protocol Bridge service..."
sudo systemctl stop protocol-bridge

echo "🔨 Building bridge binary..."
make build

if [ ! -f ./bridge ]; then
    echo "❌ Build failed - bridge binary not found"
    exit 1
fi

echo "🔐 Setting CAP_NET_BIND_SERVICE capability for port 502..."
sudo setcap 'cap_net_bind_service=+ep' ./bridge

echo "✅ Capabilities set. Verifying..."
getcap ./bridge

echo "🔄 Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "🚀 Starting Protocol Bridge service..."
sudo systemctl start protocol-bridge

echo "⏳ Waiting for service to start..."
sleep 3

echo "📊 Service Status:"
sudo systemctl status protocol-bridge --no-pager -l

echo ""
echo "✅ Bridge restart complete!"
echo ""
echo "📋 Useful commands:"
echo "   View logs: tail -f logs/bridge.log"
echo "   Check status: sudo systemctl status protocol-bridge"
echo "   View metrics: curl http://localhost:8080/metrics"
echo "   Health check: curl http://localhost:8080/health"
