# Flashduty Runner Makefile
.PHONY: all build test lint fmt clean install help

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

# Output directory
OUTPUT_DIR := dist

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := gofumpt
GOLINT := golangci-lint

# Binary name
BINARY_NAME := flashduty-runner

all: fmt lint test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(OUTPUT_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd

# Build for all platforms
build-all: build-linux build-darwin

build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(OUTPUT_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

# Format code
fmt:
	@echo "Formatting code..."
	gofumpt -l -w .
	gci write --skip-generated \
		--section standard \
		--section default \
		--section "prefix(github.com/flashcat)" \
		--section alias \
		--section blank \
		.
	$(GOMOD) tidy

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(OUTPUT_DIR)
	@rm -f coverage.out

# Install binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(OUTPUT_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Install development tools
tools:
	@echo "Installing development tools..."
	go install mvdan.cc/gofumpt@latest
	go install github.com/daixiang0/gci@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Show help
help:
	@echo "Flashduty Runner Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all        - Format, lint, test, and build"
	@echo "  build      - Build the binary for current platform"
	@echo "  build-all  - Build for all platforms (linux, darwin)"
	@echo "  test       - Run tests with coverage"
	@echo "  lint       - Run golangci-lint"
	@echo "  fmt        - Format code with gofumpt and gci"
	@echo "  tidy       - Tidy go.mod dependencies"
	@echo "  clean      - Remove build artifacts"
	@echo "  install    - Install binary to GOPATH/bin"
	@echo "  tools      - Install development tools"
	@echo "  help       - Show this help message"
