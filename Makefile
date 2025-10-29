.PHONY: test test-unit test-integration coverage build clean

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt

# Directories
PKG_DIR=./internal/...
CMD_DIR=./cmd/...
COVERAGE_FILE=coverage.txt

# Test targets
test: test-unit test-integration
	@echo "✅ All tests passed"

test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -race -coverprofile=$(COVERAGE_FILE) $(PKG_DIR)

test-integration:
	@echo "Running integration tests..."
	@if [ -d "./test/integration" ]; then \
		$(GOTEST) -tags=integration ./test/integration/...; \
	else \
		echo "⚠️  Integration tests not yet implemented"; \
	fi

coverage:
	@echo "Generating coverage report..."
	@if [ -f $(COVERAGE_FILE) ]; then \
		$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o coverage.html; \
		$(GOCMD) tool cover -func=$(COVERAGE_FILE) | grep total; \
		echo "Coverage report saved to coverage.html"; \
	else \
		echo "⚠️  Coverage file not found. Run 'make test-unit' first."; \
	fi

# Build targets
build:
	@echo "Building bridge..."
	$(GOBUILD) ./cmd/bridge

build-install: build setcap
	@echo "✅ Build complete. Bridge is ready to use."

setcap:
	@echo "Setting CAP_NET_BIND_SERVICE on bridge binary (requires sudo)..."
	@if [ -f ./bridge ]; then \
		sudo setcap 'cap_net_bind_service=+ep' ./bridge; \
		echo "Capabilities set on ./bridge"; \
	else \
		echo "bridge binary not found. Run 'make build' first."; \
		exit 1; \
	fi

build-encrypt:
	@echo "Building encrypt-value tool..."
	$(GOBUILD) ./cmd/encrypt-value

# Lint and format
lint:
	@echo "Running linters..."
	$(GOVET) $(PKG_DIR) $(CMD_DIR)

fmt:
	@echo "Formatting code..."
	$(GOFMT) -w .

# Security
security:
	@echo "Running security checks..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Clean
clean:
	@echo "Cleaning..."
	rm -f $(COVERAGE_FILE) coverage.html
	rm -f bridge encrypt-value

# Development shortcuts
dev: build
	@./bridge -config config.yaml

dev-install: build-install
	@echo "🚀 Starting bridge..."
	@./bridge -config config.yaml

# Installation
install: build
	@echo "Installing Protocol Bridge as systemd service..."
	@sudo mkdir -p /etc/protocol-bridge
	@sudo mkdir -p /etc/protocol-bridge/certs
	@sudo mkdir -p /var/log/protocol-bridge
	@sudo cp bridge /usr/local/bin/protocol-bridge
	@sudo chmod 755 /usr/local/bin/protocol-bridge
	@sudo cp config.yaml /etc/protocol-bridge/
	@sudo chmod 600 /etc/protocol-bridge/config.yaml
	@sudo cp -r certs/* /etc/protocol-bridge/certs/
	@sudo chmod 600 /etc/protocol-bridge/certs/**/*
	@sudo cp deploy/systemd/protocol-bridge.service /etc/systemd/system/
	@sudo systemctl daemon-reload
	@echo "✅ Installation complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  sudo systemctl enable protocol-bridge"
	@echo "  sudo systemctl start protocol-bridge"
	@echo "  sudo systemctl status protocol-bridge"

uninstall:
	@echo "Uninstalling Protocol Bridge..."
	@sudo systemctl stop protocol-bridge 2>/dev/null || true
	@sudo systemctl disable protocol-bridge 2>/dev/null || true
	@sudo rm -f /etc/systemd/system/protocol-bridge.service
	@sudo rm -f /usr/local/bin/protocol-bridge
	@sudo systemctl daemon-reload
	@echo "✅ Uninstalled"

# Help
help:
	@echo "Available targets:"
	@echo "  make test          - Run all tests"
	@echo "  make test-unit    - Run unit tests with race detection"
	@echo "  make test-integration - Run integration tests"
	@echo "  make coverage     - Generate coverage report"
	@echo "  make build        - Build the bridge"
	@echo "  make build-install - Build and set capabilities for port 502"
	@echo "  make build-encrypt - Build encrypt-value tool"
	@echo "  make install      - Install as systemd service (PRODUCTION)"
	@echo "  make uninstall    - Remove systemd service"
	@echo "  make dev-install  - Build with capabilities and run bridge (DEV)"
	@echo "  make lint         - Run linters"
	@echo "  make fmt          - Format code"
	@echo "  make security     - Run security checks"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make dev          - Build and run bridge"
	@echo "  make help         - Show this help"

