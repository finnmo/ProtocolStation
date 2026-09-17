# Troubleshooting Guide

Common issues and solutions for the Protocol Bridge.

## Table of Contents

1. [Connection Failures](#connection-failures)
2. [Certificate Issues](#certificate-issues)
3. [Performance Problems](#performance-problems)
4. [Message Transformation Errors](#message-transformation-errors)
5. [DLQ Investigation](#dlq-investigation)
6. [Log Analysis](#log-analysis)
7. [Reliability Issues](#reliability-issues)

---

## Connection Failures

### Issue: Cannot connect to MQTT broker

**Symptoms:**
- Bridge starts but logs show "failed to connect to MQTT broker"
- No messages are being processed

**Diagnosis:**

```bash
# Check broker is running
mosquitto_sub -h localhost -p 1883 -t 'test'

# Check network connectivity
ping <broker-host>

# Check port is open
nc -zv <broker-host> 1883
```

**Solutions:**

1. **Verify broker configuration:**
   ```yaml
   inputs:
     - name: my-input
       broker: "localhost:1883"  # Check host and port
       client_id: "unique-client-id"  # Must be unique
   ```

2. **Check authentication:**
   - If broker requires auth, provide username/password
   - For encrypted passwords, use ENC: prefix

3. **Firewall issues:**
   ```bash
   # Allow outbound connections
   sudo ufw allow out 1883/tcp
   ```

4. **TLS/SSL issues:**
   - Verify certificate paths are correct
   - Check certificate expiration dates
   - Ensure CA certificate is trusted

### Issue: Frequent reconnections

**Symptoms:**
- Log shows frequent "lost connection" and "attempting to reconnect" messages

**Solutions:**

1. **Increase KeepAlive:**
   ```yaml
   # In input/output config
   keep_alive: 60  # seconds
   ```

2. **Check network stability:**
   ```bash
   # Monitor packet loss
   ping -c 100 <broker-host>
   ```

3. **Update MQTT client library:**
   ```bash
   go get -u github.com/eclipse/paho.mqtt.golang
   ```

---

## Certificate Issues

### Issue: TLS handshake failed

**Symptoms:**
- Error: "failed to create TLS config"
- Error: "certificate verify failed"

**Diagnosis:**

```bash
# Test certificate validity
openssl x509 -in certificate.pem.crt -text -noout

# Test TLS connection
openssl s_client -connect <broker>:8883 \
  -cert certificate.pem.crt \
  -key private.pem.key \
  -CAfile ca.pem
```

**Solutions:**

1. **Verify certificate files exist:**
   ```bash
   ls -la certs/output/aws-iot-ap-southeast-2/
   ```

2. **Check file permissions:**
   ```bash
   chmod 600 certs/**/*.pem*
   ```

3. **Validate certificate format:**
   - Certificates must be PEM format
   - Private key must match certificate
   - CA certificate must be compatible

4. **Update expired certificates:**
   ```bash
   # Download new certificates
   aws iot describe-endpoint --endpoint-type iot:Data-ATS
   
   # Replace old certificates
   cp new-cert/* certs/output/aws-iot-ap-southeast-2/
   ```

### Issue: Certificate path not found

**Symptoms:**
- Error: "failed to read CA certificate"
- Error: "certificate path does not exist"

**Solutions:**

1. **Use absolute paths in config:**
   ```yaml
   outputs:
     - name: aws-output
       cert_path: /app/certs/output/aws-iot/certificate.pem.crt
       key_path: /app/certs/output/aws-iot/private.pem.key
       ca_path: /app/certs/output/aws-iot/AmazonRootCA1.pem
   ```

2. **Check certificate organization:**
   ```
   certs/output/<broker-name>/
   ├── certificate.pem.crt
   ├── private.pem.key
   └── ca.pem
   ```

---

## Performance Problems

### Issue: High latency in message processing

**Symptoms:**
- Messages take long to process
- Buffer fills up

**Diagnosis:**

```bash
# Check metrics
curl http://localhost:8080/metrics | grep duration

# Check buffer utilization
curl http://localhost:8080/metrics | grep buffer_size
```

**Solutions:**

1. **Increase buffer size:**
   ```yaml
   pipelines:
     - name: my-pipeline
       buffer_size: 5000  # Increase from default 1000
       max_workers: 20     # Increase from default 10
   ```

2. **Optimize transformer:**
   - Reduce JavaScript execution time
   - Cache frequently computed values
   - Avoid heavy computations in transformation

3. **Check network latency:**
   ```bash
   # Check latency to brokers
   ping -c 10 <broker-host>
   ```

### Issue: Memory usage growing

**Symptoms:**
- Memory usage increases over time
- OOM kills

**Diagnosis:**

```bash
# Monitor memory
ps aux | grep bridge

# Check for memory leaks
go tool pprof http://localhost:8080/debug/pprof/heap
```

**Solutions:**

1. **Reduce buffer sizes:**
   ```yaml
   pipelines:
     - name: my-pipeline
       buffer_size: 500  # Reduce buffer
   ```

2. **Enable message expiration:**
   ```yaml
   pipelines:
     - name: my-pipeline
       message_ttl: 30s  # Discard old messages
   ```

3. **Restart periodically:**
   ```bash
   # Add to systemd service
   [Service]
   Restart=always
   RuntimeMaxSec=86400  # Restart after 24 hours
   ```

---

## Message Transformation Errors

### Issue: Transform function returning null

**Symptoms:**
- Messages are filtered out
- No messages in outputs

**Diagnosis:**

Check transformer script:

```javascript
function transform(input, context) {
  // This will filter out all messages
  return null;
  
  // Should return transformed object
  return { ... };
}
```

**Solutions:**

1. **Check filtering logic:**
   ```javascript
   function transform(input, context) {
     // Debug: log the input
     console.log("Input:", JSON.stringify(input));
     console.log("Context:", JSON.stringify(context));
     
     // Only filter if truly unwanted
     if (!input || !context.topic) {
       return null;
     }
     
     return transformed;
   }
   ```

2. **Validate input format:**
   - Ensure input is valid JSON
   - Check field names match expected schema

### Issue: JavaScript execution errors

**Symptoms:**
- Error: "failed to transform message"
- Error: "ReferenceError: X is not defined"

**Diagnosis:**

```bash
# Enable debug logging
logging:
  level: debug
```

**Solutions:**

1. **Fix JavaScript syntax:**
   ```javascript
   // Wrong
   function transform(input, context {
     return input
   }
   
   // Correct
   function transform(input, context) {
     return input;
   }
   ```

2. **Handle undefined values:**
   ```javascript
   function transform(input, context) {
     const value = input.field || 0;  // Safe default
     return { value: value };
   }
   ```

---

## DLQ Investigation

### Issue: Messages going to DLQ

**Symptoms:**
- Messages accumulating in DLQ
- No messages in actual outputs

**Diagnosis:**

```bash
# Subscribe to DLQ topic
mosquitto_sub -h localhost -p 1883 -t 'dlq/failed-messages'

# Check DLQ metrics
curl http://localhost:8080/metrics | grep dlq
```

**Solutions:**

1. **Check error messages in DLQ:**
   DLQ messages include original message + error:
   ```json
   {
     "message_id": "...",
     "topic": "...",
     "original_payload": {...},
     "error": "MQTT client not connected",
     "timestamp": "2025-01-28T12:00:00Z"
   }
   ```

2. **Fix connection issues:**
   - Ensure output connections are established
   - Check broker availability
   - Verify credentials

3. **Retry DLQ messages:**
   ```bash
   # Manually republish from DLQ
   mosquitto_pub -h localhost -p 1883 \
     -t 'sensors/temperature' \
     -m '<dlq-payload>'
   ```

---

## Log Analysis

### Parsing Logs

**Structured JSON logs:**

```bash
# View recent logs
tail -f bridge.log

# Filter errors only
cat bridge.log | jq 'select(.level=="error")'

# Count messages by topic
cat bridge.log | jq -r '.topic' | sort | uniq -c

# Find transformation errors
cat bridge.log | jq 'select(.msg | contains("transform"))'
```

**Key Log Fields:**

- `level`: debug, info, warn, error
- `msg`: Log message
- `message_id`: Message identifier
- `topic`: MQTT topic
- `pipeline`: Pipeline name
- `error`: Error details

### Common Log Patterns

**Connection established:**
```json
{"level":"info","msg":"connected to MQTT broker","broker":"localhost:1883"}
```

**Message processed:**
```json
{"level":"info","msg":"processing message","message_id":"123","topic":"sensors/temp"}
```

**Message failed:**
```json
{"level":"error","msg":"failed to send message","message_id":"123","error":"timeout"}
```

**Transformer filtered:**
```json
{"level":"info","msg":"message filtered out by transformer","message_id":"123"}
```

### Debug Mode

Enable detailed logging:

```yaml
logging:
  level: debug
  format: json
```

This shows:
- All messages received
- Transformation steps
- Connection attempts
- Buffer states
- Performance metrics

---

## Reliability Issues

### Issue: Certificate Expiration Warning

**Symptoms:**
- Log shows "certificate expiring within X days"
- Metrics show `cert_expiration_days` approaching zero

**Solutions:**
1. **Check certificate expiration:**
   ```bash
   openssl x509 -in certs/input/aws-iot-ap-southeast-2/certificate.pem.crt -noout -dates
   ```

2. **Update certificate:**
   - Download new certificate from your certificate authority
   - Replace certificate files in `certs/` directory
   - Restart the bridge to load new certificates

3. **Certificate rotation best practices:**
   - Set calendar reminder for 30 days before expiration
   - Automate certificate updates if possible
   - Keep backup of current working certificates

### Issue: Low Disk Space Alerts

**Symptoms:**
- Log shows "low disk space" warning
- Metrics show `disk_used_percent` > 90%

**Diagnosis:**
```bash
# Check disk usage
df -h

# Check specific directory
du -sh logs/ modbus_registers.json
```

**Solutions:**
1. **Clean up old logs:**
   ```bash
   # Logs auto-rotate, but you can manually clean
   rm logs/bridge.log.*
   ```

2. **Check Modbus persistence file:**
   ```bash
   # File is auto-cleaned, but check size
   ls -lh modbus_registers.json
   ```

3. **Increase disk space or relocate data:**
   ```yaml
   # In config.yaml
   logging:
     file: /mnt/larger-disk/logs/bridge.log
   ```

### Issue: DLQ Growing

**Symptoms:**
- Metrics show `dlq_size_bytes` increasing
- DLQ exceeds configured `alert_size`

**Diagnosis:**
```bash
# Check DLQ messages
mosquitto_sub -h localhost -p 1883 -t 'dlq/failed-messages' -v
```

**Solutions:**
1. **Investigate failed messages:**
   - Review DLQ message errors
   - Check if output servers are healthy
   - Verify network connectivity

2. **Temporarily increase DLQ size:**
   ```yaml
   error_handling:
     dead_letter_queue:
       max_size: 500  # MB
   ```

3. **Archive and clear DLQ:**
   ```bash
   # Subscribe to DLQ and archive
   mosquitto_sub -h localhost -p 1883 -t 'dlq/failed-messages' > archived-dlq.json
   
   # Clear DLQ topic by unsubscribing
   ```

4. **Fix underlying issue:**
   - Address connection problems
   - Fix transformer errors
   - Resolve output broker issues

### Issue: Server Auto-Restarting Frequently

**Symptoms:**
- Logs show frequent "server restarted successfully"
- Metrics show `server_restarts_total` increasing

**Diagnosis:**
```bash
# Check server health
curl http://localhost:8080/ready

# Check logs for restart reason
tail -f logs/bridge.log | grep "server restarted"
```

**Solutions:**
1. **Check Docker container logs:**
   ```bash
   docker logs mosquitto-broker
   ```

2. **Investigate network issues:**
   - Check if ports are being used by other services
   - Verify firewall rules
   - Check DNS resolution

3. **Review resource limits:**
   - Ensure sufficient CPU/memory
   - Check Docker container resource limits
   - Monitor Docker daemon health

4. **Temporarily disable auto-restart for diagnosis:**
   - Reduce health check frequency in code (requires rebuild)
   - Monitor logs for specific failure pattern

## Getting Help

If issues persist:

1. **Collect diagnostics:**
   ```bash
   # System info
   uname -a
   go version
   
   # Bridge logs
   tail -1000 logs/bridge.log > diagnostics.log
   
   # Metrics
   curl http://localhost:8080/metrics > metrics.txt
   
   # Disk space
   df -h > disk-usage.txt
   
   # Config (remove secrets)
   cat config.yaml > config-public.txt
   ```

2. **Report with:**
   - Bridge version
   - Configuration (sanitized)
   - Error logs
   - Metrics output
   - Steps to reproduce

3. **Community support:**
   - Check GitHub issues
   - Submit detailed bug report
   - Contribute fix if possible


