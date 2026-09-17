# Certificate Organization

TLS certificates are gitignored and must be placed manually on each server.

## Directory Structure

```
certs/
├── input/
│   └── aws-iot-ap-southeast-2/     ← Water & gas meter input (AWS IoT)
│       ├── certificate.pem.crt
│       ├── private.pem.key
│       └── ca.pem
└── output/
    ├── aws-iot-ap-southeast-2/     ← People counter output (AWS IoT)
    │   ├── certificate.pem.crt
    │   ├── private.pem.key
    │   └── ca.pem
    └── aws-iot-parking/            ← Parking output (AWS IoT, separate account/endpoint)
        ├── certificate.pem.crt
        ├── private.pem.key
        └── ca.pem
```

## Naming Convention

| File | Description |
|---|---|
| `certificate.pem.crt` | Device certificate |
| `private.pem.key` | Private key |
| `ca.pem` | CA / root certificate |

## Config Reference

```yaml
# Input example (water/gas meter)
cert_path: certs/input/aws-iot-ap-southeast-2/certificate.pem.crt
key_path:  certs/input/aws-iot-ap-southeast-2/private.pem.key
ca_path:   certs/input/aws-iot-ap-southeast-2/ca.pem

# Output example (people counter)
cert_path: certs/output/aws-iot-ap-southeast-2/certificate.pem.crt
key_path:  certs/output/aws-iot-ap-southeast-2/private.pem.key
ca_path:   certs/output/aws-iot-ap-southeast-2/ca.pem

# Output example (parking)
cert_path: certs/output/aws-iot-parking/certificate.pem.crt
key_path:  certs/output/aws-iot-parking/private.pem.key
ca_path:   certs/output/aws-iot-parking/ca.pem
```

## Certificate Expiry

The bridge checks TLS certificate expiry on startup:
- **≤ 30 days** — warning logged
- **≤ 7 days** — warning logged
- **≤ 1 day** — error logged

Check expiry manually:
```bash
openssl x509 -in certs/output/aws-iot-ap-southeast-2/certificate.pem.crt -noout -dates
```
