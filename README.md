# Protocol Bridge

A Go-based bridge for moving data between protocols (e.g., MQTT → Modbus), with JavaScript-based transforms and simple YAML configuration.

Think of it as a central station: messages arrive from different "lines" (inputs like MQTT), and the station orchestrates their transfers through "platforms" (transformers) onto the correct "departures" (outputs like Modbus or MQTT). You define the routes (pipelines), and the station reliably keeps trains moving—recovering from delays, logging traffic, and ensuring everything gets to the right destination.

## Highlights

- **MQTT input/output** with auto-reconnect (subscriptions restored on reconnect)
- **Modbus TCP server** on port 502 by default, 32-bit SINT values stored big‑endian across 2 holding registers
- **JavaScript transformers** for payload shaping, with persistent state across messages
- **1→N routing**, retries, DLQ, circuit breaker, and hosted server management
- **HTTP API** on port 8080 — `/health`, `/ready`, `/status`, `/metrics` (Prometheus)
- **Structured logging** with rotation; includes Modbus poll logs (FC03) and startup details

## Quick Start

### 1) Prerequisites

- Go 1.24+
- Docker + Docker Compose (if you want the local MQTT broker started for you)
- If running Modbus on port 502 under systemd, allow low-port binding (see below)

### 2) Build

```bash
make build-install
```

### 3) Configure

```bash
cp config.example.yaml config.yaml
# edit config.yaml to your environment (MQTT certs, topics, etc.)
```

Key defaults that matter:
- Modbus server listens on `:502`
- Persistence file: `register_values.json`
- Legacy Modbus addressing: serverRegister → register offset = serverRegister − 400001

### 4) Run (Development)

```bash
./bridge -config config.yaml
```

The bridge automatically detects/starts the local MQTT server (if configured), runs until stopped, and shuts down gracefully.

### 5) Install as a Service (systemd)

```bash
make install

# if binding port 502 under systemd, set capabilities on the binary once:
sudo setcap 'cap_net_bind_service=+ep' /home/optech/ProtocolBridge/bridge

# allow the capability in the unit (recommended, one-time):
# In /etc/systemd/system/protocol-bridge.service under [Service]
#   NoNewPrivileges=false
#   AmbientCapabilities=CAP_NET_BIND_SERVICE
#   CapabilityBoundingSet=CAP_NET_BIND_SERVICE

sudo systemctl daemon-reload
sudo systemctl enable protocol-bridge
sudo systemctl start protocol-bridge
```

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
1) Check if the MQTT server is running
2) Start it with Docker Compose if not
3) Monitor health and restart if needed
4) Stop it on shutdown

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

See `config.example.yaml` for a complete, working template. Important excerpts:

```yaml
servers:
  - name: modbus-server
    type: modbus
    enabled: true
    address: ":502"
    persistence_file: "register_values.json"

inputs:
  - name: water-aws-input
    type: mqtt
    broker: tls://a2ucaobdsgkqr9-ats.iot.ap-southeast-2.amazonaws.com:8883
    client_id: protocol-bridge-water
    topics:
      - pfd/ot/water
    qos: 1
    cert_path: certs/input/aws-iot-ap-southeast-2/certificate.pem.crt
    key_path:  certs/input/aws-iot-ap-southeast-2/private.pem.key
    ca_path:   certs/input/aws-iot-ap-southeast-2/ca.pem

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

outputs:
  - name: water-modbus-output
    type: modbus
    server: modbus-server

pipelines:
  - name: water-modbus-pipeline
    input: water-aws-input
    routes:
      - transformer: water-pass-through
        outputs:
          - water-modbus-output
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
- **Output**: MQTT or Modbus
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

## Modbus Semantics and Client Access

- Modbus TCP server listens on `:502` (configurable).
- Each 32‑bit signed value is stored across two holding registers in big‑endian word order: high word at offset N, low word at offset N+1.
- Legacy addressing is supported: for a conventional server register R (e.g., 403021), the bridge writes at register offset `R − 400001` (e.g., 3020).
- Reads are logged at info level: `Modbus poll (FC03)` with server address, remote, unitID, startAddress, quantity.

Example (read Unit ID 5, server register 403021):

```bash
# offset = 403021 - 400001 = 3020, read 2 registers
mbpoll -m tcp -a 5 -r 3020 -c 2 127.0.0.1 502
```

## Troubleshooting

- Port 502 permission denied under systemd:
  - Ensure the unit has `NoNewPrivileges=false`, and includes
    `AmbientCapabilities=CAP_NET_BIND_SERVICE` and `CapabilityBoundingSet=CAP_NET_BIND_SERVICE`.
  - Alternatively, run `sudo setcap 'cap_net_bind_service=+ep' ./bridge` (must re-run after rebuilding the binary unless the unit grants caps).
- No MQTT messages after long uptime:
  - Subscriptions are automatically restored on reconnect. Check logs for `connected to MQTT broker` and ensure your topics are correct.
- See `docs/troubleshooting.md` for deeper diagnostics.

## Development

### Project Structure

```
/
├── cmd/
│   ├── bridge/          # Main binary
│   └── encrypt-value/   # CLI tool to encrypt config values
├── internal/
│   ├── config/          # Configuration parsing and validation
│   ├── health/          # Health/readiness checks, disk space monitor
│   ├── input/           # Input implementations (MQTT)
│   ├── logging/         # Structured logger setup with rotation
│   ├── metrics/         # Prometheus metrics
│   ├── output/          # Output implementations (MQTT, Modbus)
│   ├── pipeline/        # Pipeline orchestration, circuit breaker
│   ├── retry/           # Retry manager and DLQ
│   ├── security/        # AES-256-GCM encryption for config secrets
│   ├── server/          # HTTP API server, server lifecycle manager
│   │   ├── modbus/      # Modbus TCP server
│   │   └── bacnet/      # BACnet (stub, disabled)
│   ├── shutdown/        # Graceful shutdown coordination
│   └── transformer/     # JavaScript transformer + file-based state
├── .github/workflows/   # CI (build, vet, race-detected tests)
├── deploy/
│   ├── systemd/         # Systemd service file
│   └── windows/         # NSSM installer scripts (windows-deployment branch)
├── pkg/message/         # Shared message type
└── config.example.yaml  # Configuration template
```

### Building

```bash
# Build (Linux)
make build

# Build + set CAP_NET_BIND_SERVICE (required for port 502)
make build-install

# Run tests (race detector enabled)
go test -race -timeout 60s ./...
```

## License

This project is licensed under the MIT License.