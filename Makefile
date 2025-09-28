# Makefile for minio2rustfs migration tool

# Variables
BINARY_NAME=minio2rustfs
MAIN_PATH=./cmd/main.go
BUILD_DIR=build
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Colors for output
RED=\033[31m
GREEN=\033[32m
YELLOW=\033[33m
BLUE=\033[34m
RESET=\033[0m

.PHONY: all build clean test coverage deps fmt vet lint run help docker docker-build docker-run install

# Default target
all: clean fmt vet test build

# Build the binary
build:
	@echo "$(BLUE)Building $(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)$(RESET)"

# Build for multiple platforms
build-all: clean
	@echo "$(BLUE)Building for multiple platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "$(GREEN)Cross-platform build completed!$(RESET)"

# Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning...$(RESET)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "$(GREEN)Clean completed$(RESET)"

# Run tests
test:
	@echo "$(BLUE)Running tests...$(RESET)"
	$(GOTEST) -v ./...

# Run tests with coverage
coverage:
	@echo "$(BLUE)Running tests with coverage...$(RESET)"
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(RESET)"

# Download dependencies
deps:
	@echo "$(BLUE)Downloading dependencies...$(RESET)"
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "$(GREEN)Dependencies updated$(RESET)"

# Format code
fmt:
	@echo "$(BLUE)Formatting code...$(RESET)"
	$(GOFMT) ./...

# Vet code
vet:
	@echo "$(BLUE)Vetting code...$(RESET)"
	$(GOVET) ./...

# Lint code (requires golangci-lint)
lint:
	@echo "$(BLUE)Linting code...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(RESET)"; \
	fi

# Run the application
run: build
	@echo "$(BLUE)Running $(BINARY_NAME)...$(RESET)"
	./$(BUILD_DIR)/$(BINARY_NAME) --help

# Install the binary to GOPATH/bin
install:
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(RESET)"
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(GOPATH)/bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Installed to $(GOPATH)/bin/$(BINARY_NAME)$(RESET)"

# Docker targets
docker: docker-build

docker-build:
	@echo "$(BLUE)Building Docker image...$(RESET)"
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .
	@echo "$(GREEN)Docker image built: $(BINARY_NAME):$(VERSION)$(RESET)"

docker-run:
	@echo "$(BLUE)Running Docker container...$(RESET)"
	docker run --rm -it $(BINARY_NAME):latest --help

# Development helpers
dev-setup:
	@echo "$(BLUE)Setting up development environment...$(RESET)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "$(GREEN)Development environment ready$(RESET)"

# Quick development build and run
dev: fmt vet build

# Release build
release: clean test build-all
	@echo "$(GREEN)Release build completed!$(RESET)"
	@echo "$(BLUE)Artifacts:$(RESET)"
	@ls -la $(BUILD_DIR)/

# Check project status
status:
	@echo "$(BLUE)Project Status:$(RESET)"
	@echo "Go version: $(shell go version)"
	@echo "Module: $(shell go mod tidy && go list -m)"
	@echo "Dependencies: $(shell go list -m all | wc -l) modules"
	@echo "Source files: $(shell find . -name '*.go' | wc -l) files"
	@echo "Test files: $(shell find . -name '*_test.go' | wc -l) files"

# Show help
help:
	@echo "$(BLUE)Available targets:$(RESET)"
	@echo "  $(YELLOW)build$(RESET)       - Build the binary"
	@echo "  $(YELLOW)build-all$(RESET)   - Build for multiple platforms"
	@echo "  $(YELLOW)clean$(RESET)       - Clean build artifacts"
	@echo "  $(YELLOW)test$(RESET)        - Run tests"
	@echo "  $(YELLOW)coverage$(RESET)    - Run tests with coverage report"
	@echo "  $(YELLOW)deps$(RESET)        - Download and tidy dependencies"
	@echo "  $(YELLOW)fmt$(RESET)         - Format code"
	@echo "  $(YELLOW)vet$(RESET)         - Vet code"
	@echo "  $(YELLOW)lint$(RESET)        - Lint code (requires golangci-lint)"
	@echo "  $(YELLOW)run$(RESET)         - Build and run the application"
	@echo "  $(YELLOW)install$(RESET)     - Install binary to GOPATH/bin"
	@echo "  $(YELLOW)docker$(RESET)      - Build Docker image"
	@echo "  $(YELLOW)docker-run$(RESET)  - Run Docker container"
	@echo "  $(YELLOW)dev-setup$(RESET)   - Setup development environment"
	@echo "  $(YELLOW)dev$(RESET)         - Quick development build"
	@echo "  $(YELLOW)release$(RESET)     - Build release artifacts"
	@echo "  $(YELLOW)status$(RESET)      - Show project status"
	@echo "  $(YELLOW)help$(RESET)        - Show this help message"
	@echo ""
	@echo "$(BLUE)Example usage:$(RESET)"
	@echo "  make build"
	@echo "  make test"
	@echo "  make docker"
	@echo "  make release"