# Certificate Organization for Protocol Bridge
# 
# Directory Structure:
#
# certs/
#   ├── input/
#   │   ├── aws-iot-ap-southeast-2/     # For AWS IoT Core as INPUT
#   │   │   ├── certificate.pem.crt
#   │   │   ├── private.pem.key
#   │   │   └── AmazonRootCA1.pem
#   │   ├── local-broker/                # For local broker as INPUT (if needed)
#   │   └── other-brokers/               # Other input brokers
#   │
#   └── output/
#       ├── aws-iot-ap-southeast-2/      # For AWS IoT Core as OUTPUT
#       │   ├── certificate.pem.crt
#       │   ├── private.pem.key
#       │   └── AmazonRootCA1.pem
#       ├── local-broker/                # For local broker as OUTPUT (if needed)
#       └── other-brokers/               # Other output brokers
#
# Naming Convention:
# - certificate.pem.crt  (the device certificate)
# - private.pem.key      (the private key)
# - AmazonRootCA1.pem    (or similar CA certificate)
#
# Usage in config.yaml:
#
# For OUTPUT:
# outputs:
#   - name: aws-iot-output
#     type: mqtt
#     broker: agi8wtqgu97at-ats.iot.ap-southeast-2.amazonaws.com:8883
#     cert_path: certs/output/aws-iot-ap-southeast-2/certificate.pem.crt
#     key_path: certs/output/aws-iot-ap-southeast-2/private.pem.key
#     ca_path: certs/output/aws-iot-ap-southeast-2/AmazonRootCA1.pem
#
# For INPUT (if needed in future):
# inputs:
#   - name: aws-iot-input
#     type: mqtt
#     broker: agi8wtqgu97at-ats.iot.ap-southeast-2.amazonaws.com:8883
#     cert_path: certs/input/aws-iot-ap-southeast-2/certificate.pem.crt
#     key_path: certs/input/aws-iot-ap-southeast-2/private.pem.key
#     ca_path: certs/input/aws-iot-ap-southeast-2/AmazonRootCA1.pem
