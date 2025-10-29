# Protocol Bridge

A flexible Go-based protocol bridge that enables seamless communication between different IoT protocols with customizable message transformation capabilities.

## Features

- **MQTT Input/Output**: Connect to MQTT brokers for both consuming and publishing messages
- **JavaScript Transformers**: Customize message transformation using JavaScript with full JSON support
- **1-to-Many Routing**: Route messages from one input through multiple transformer+output chains
- **Retry Mechanism**: Configurable exponential backoff retry for failed operations
- **Dead Letter Queue**: Automatic handling of failed messages with configurable DLQ
- **Hosted Server Management**: Automatic lifecycle management of hosted servers (MQTT brokers)
  - **Auto-Detection**: Checks if MQTT server is already running before starting
  - **Docker Integration**: Automatically starts/stops MQTT servers using Docker Compose
  - **Health Monitoring**: Monitors server health and automatically restarts on failure
- **YAML Configuration**: Easy-to-read configuration format
- **Structured Logging**: Comprehensive logging with automatic rotation and compression
- **Long-Running Service**: Runs continuously until stopped (SIGINT/SIGTERM)
- **Production-Ready Reliability**: Built for years of unattended operation with automatic cleanup and monitoring

## Quick Start

### 1. Prerequisites

- Go 1.24 or later
- Docker and Docker Compose (for MQTT broker)

### 2. Build the Application

```bash
go build ./cmd/bridge
```

### 3. Configure the Bridge

The bridge can automatically manage MQTT servers for you, or you can start them manually.

Copy the example configuration and customize it:

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` to match your setup.

### 4. Run the Bridge

```bash
./bridge -config config.yaml
```

The bridge will:
- **Automatically detect** if MQTT servers are already running
- **Start MQTT servers** using Docker Compose if they're not running
- **Run continuously** until you stop it with Ctrl+C
- **Gracefully shutdown** and stop all managed servers when stopped

## Hosted Server Management

The Protocol Bridge can automatically manage MQTT servers for you:

### Automatic MQTT Server Management

When you configure a hosted MQTT server in your config:

```yaml
servers:
  - name: local-mqtt
    type: mqtt
    enabled: true
    broker: localhost:1883
```

The bridge will:

1. **Check if MQTT server is running** on the specified broker address
2. **Start the server automatically** using Docker Compose if it's not running
3. **Monitor server health** and restart if needed
4. **Stop the server** when the bridge shuts down

### Manual MQTT Server Management

If you prefer to manage MQTT servers manually:

```bash
# Start MQTT server manually
cd mqtt/server
docker-compose up -d

# Run the bridge (it will detect the running server)
./bridge -config config.yaml

# Stop MQTT server manually
cd mqtt/server
docker-compose down
```

## Configuration

The bridge uses YAML configuration files. Here's a basic example:

```yaml
# Input sources
inputs:
  - name: sensor-input
    type: mqtt
    broker: localhost:1883
    client_id: bridge-input-1
    topics:
      - sensors/temperature
      - sensors/humidity
    qos: 1

# Transformers
transformers:
  - name: temperature-transform
    type: javascript
    script: |
      function transform(input) {
        return {
          device_id: input.device,
          value: parseFloat(input.temp),
          unit: "celsius",
          timestamp: new Date().toISOString()
        };
      }

# Output destinations
outputs:
  - name: temp-output
    type: mqtt
    broker: localhost:1883
    client_id: bridge-output-temp
    topic: processed/temperature
    qos: 1

# Pipelines (1 input -> many outputs)
pipelines:
  - name: sensor-pipeline
    input: sensor-input
    routes:
      - transformer: temperature-transform
        outputs:
          - temp-output
```

## Architecture

### Message Flow

1. **Input**: Receives raw messages from configured sources (MQTT topics)
2. **Pipeline**: Routes messages through configured transformer+output chains
3. **Transformer**: Executes JavaScript to transform message payload
4. **Output**: Publishes transformed messages to configured destinations
5. **Error Handling**: Retries failed operations and sends failures to DLQ

### Components

- **Input**: MQTT client that subscribes to topics and forwards messages
- **Transformer**: JavaScript runtime for message transformation
- **Output**: MQTT client that publishes messages to topics
- **Pipeline**: Orchestrates message flow through the system
- **Retry Manager**: Handles retry logic with exponential backoff
- **DLQ Manager**: Manages failed message handling with size limits
- **Server Manager**: Manages lifecycle of hosted servers with health monitoring
- **Health Monitor**: Automatic server health checks and auto-recovery
- **Disk Monitor**: Prevents writes when disk space is critically low
- **Certificate Monitor**: Warns about TLS certificate expiration

### Reliability Features

The system is designed for long-term unattended operation:

- **Automatic Log Rotation**: Logs rotate at 100MB, kept for 7 days, compressed
- **File Cleanup**: Modbus persistence automatically removes entries older than 30 days
- **Certificate Monitoring**: Alerts 30/7/1 days before TLS certificate expiration
- **Auto-Recovery**: Servers automatically restart on failure with health checks every 60s
- **Disk Space Protection**: Prevents writes when disk usage exceeds thresholds
- **DLQ Size Limits**: Configurable limits prevent unbounded queue growth
- **Comprehensive Metrics**: Prometheus metrics for monitoring all critical aspects

## JavaScript Transformers

Transformers use JavaScript to process incoming messages. The `transform` function receives the parsed JSON input and should return the transformed data.

### Example Transformers

**Temperature Conversion:**
```javascript
function transform(input) {
  return {
    device_id: input.device,
    celsius: parseFloat(input.temp),
    fahrenheit: (parseFloat(input.temp) * 9/5) + 32,
    timestamp: new Date().toISOString()
  };
}
```

**Data Aggregation:**
```javascript
function transform(input) {
  return {
    sensor_id: input.device,
    readings: {
      temperature: input.temp,
      humidity: input.humidity,
      pressure: input.pressure
    },
    metadata: {
      location: input.location,
      timestamp: Date.now()
    }
  };
}
```

## Error Handling

The bridge includes comprehensive error handling:

- **Retry Logic**: Configurable exponential backoff for transient failures
- **Dead Letter Queue**: Failed messages are sent to a configurable DLQ topic
- **Graceful Degradation**: Individual message failures don't stop the entire pipeline
- **Connection Recovery**: Automatic reconnection for MQTT clients

## Logging

Structured logging with configurable levels and formats:

```yaml
logging:
  level: info  # debug, info, warn, error
  format: json # json, console
```

## Development

### Project Structure

```
/
├── cmd/bridge/           # Main application
├── internal/
│   ├── config/          # Configuration parsing
│   ├── input/           # Input implementations
│   ├── transformer/      # Transformer implementations
│   ├── output/          # Output implementations
│   ├── pipeline/        # Pipeline orchestration
│   ├── retry/           # Retry and DLQ logic
│   └── server/          # Server management
├── pkg/message/         # Common message types
└── config.example.yaml  # Example configuration
```

### Building

```bash
# Build the application
go build ./cmd/bridge

# Run tests
go test ./...

# Run with race detection
go test -race ./...
```

## License

This project is licensed under the MIT License.