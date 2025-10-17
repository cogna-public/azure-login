.PHONY: build build-static test clean install help

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
STATIC_LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -extldflags '-static'"

# Binary names
BINARY_NAME := azure-login
BUILD_DIR := bin

help:
	@echo "Available targets:"
	@echo "  build         - Build the binary for current platform"
	@echo "  build-static  - Build statically-linked binaries for all platforms"
	@echo "  test          - Run tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install binary to /usr/local/bin"
	@echo "  help          - Show this help message"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/azure-login

build-static:
	@echo "Building statically-linked binaries..."
	@mkdir -p $(BUILD_DIR)
	@echo "  -> Linux AMD64"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a $(STATIC_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/azure-login
	@echo "  -> Linux ARM64"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a $(STATIC_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/azure-login
	@echo "  -> macOS AMD64"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a $(STATIC_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/azure-login
	@echo "  -> macOS ARM64"
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -a $(STATIC_LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/azure-login
	@echo "Done! Binaries are in $(BUILD_DIR)/"

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Done! You can now run: $(BINARY_NAME)"

fmt:
	@echo "Formatting code..."
	go fmt ./...

lint:
	@echo "Linting code..."
	golangci-lint run ./...

.DEFAULT_GOAL := help
