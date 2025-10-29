#!/bin/bash

# Start Protocol Bridge for People Counter
cd /home/optech/ProtocolBridge

# Stop any existing bridge
echo "🛑 Stopping existing bridge instances..."
pkill -f "./bridge" || true
sleep 2

echo "🔍 Checking server requirements from config..."

# Check if config.yaml defines MQTT server
if grep -q "type: mqtt" config.yaml; then
    echo "📡 MQTT server configured, checking status..."
    
    # Check if MQTT container is running
    if ! docker ps | grep -q "mosquitto-broker"; then
        echo "📦 MQTT server not running, starting Docker containers..."
        
        # Start MQTT server using Docker Compose
        cd mqtt/server
        docker-compose up -d
        
        if [ $? -eq 0 ]; then
            echo "✅ MQTT server started successfully"
            
            # Wait for MQTT server to be ready
            echo "⏳ Waiting for MQTT server to be ready..."
            for i in {1..30}; do
                if nc -z localhost 1883 2>/dev/null; then
                    echo "✅ MQTT server is ready!"
                    break
                fi
                echo -n "."
                sleep 1
            done
            echo ""
        else
            echo "❌ Failed to start MQTT server"
            cd /home/optech/ProtocolBridge
            exit 1
        fi
        
        cd /home/optech/ProtocolBridge
    else
        echo "✅ MQTT server already running"
    fi
    
    # Check if MQTT server is actually accessible
    if ! nc -z localhost 1883 2>/dev/null; then
        echo "⚠️  Warning: MQTT server may not be fully ready"
    fi
else
    echo "ℹ️  No MQTT server configured, skipping"
fi

# Check if config.yaml defines Modbus server
if grep -q "type: modbus" config.yaml; then
    echo "📡 Modbus server configured"
    echo "ℹ️  Note: Modbus server on port 502 requires CAP_NET_BIND_SERVICE capability"
    echo "    Run: sudo setcap 'cap_net_bind_service=+ep' ./bridge"
fi

# Create log directory if needed
mkdir -p logs

echo "🚀 Starting Protocol Bridge..."

# Start the bridge (logs will go to logs/bridge.log per config)
nohup ./bridge -config config.yaml > /dev/null 2>&1 &

# Wait for startup
sleep 3

# Show status
echo "✅ Protocol Bridge Started!"
echo ""
echo "📊 Status:"
tail -10 logs/bridge.log 2>/dev/null || echo "Waiting for logs..."
echo ""
echo "🧪 Test Commands:"
echo "   Subscribe to output: mosquitto_sub -h localhost -p 1883 -t 'pfd/ot/peopleCounter/bosch5100i/raw'"
echo "   Publish test: mosquitto_pub -h localhost -p 1883 -t 'site/bentley/418-PCD-01-0001/onvif-ej/RuleEngine/CountAggregation/Counter/&1/Internal Door Inbound' -m '{\"UtcTime\":\"2025-10-28T12:00:00Z\",\"Data\":{\"Count\":5}}'"
echo ""
echo "📋 Monitor logs: tail -f logs/bridge.log"
echo "📈 View metrics: curl http://localhost:8080/metrics"
echo "❤️  Health check: curl http://localhost:8080/health"
