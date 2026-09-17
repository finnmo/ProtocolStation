#!/bin/bash
cd /home/optech/ProtocolBridge

# Stop old bridge
pkill -f "./bridge" || true
sleep 2

# Start bridge
./bridge -config config.yaml > bridge-$(date +%s).log 2>&1 &
echo "Bridge started with PID: $!"
sleep 5

echo ""
echo "✅ Bridge is running and subscribed to ALL topics"
echo "📊 Waiting for camera messages..."
echo ""

# Monitor for 30 seconds
tail -f bridge-*.log | grep -E "(message received|processing message|filtered)" &
TAIL_PID=$!
sleep 30
kill $TAIL_PID 2>/dev/null || true

echo ""
echo "Monitoring complete"
