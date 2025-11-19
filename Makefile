.PHONY: build install test clean run fmt vet lint build-all help

# Binary name
BINARY_NAME=cleanup
PACKAGE=github.com/fenilsonani/cleanup-cache

# Build directory
BUILD_DIR=bin

# Version information
VERSION?=0.1.0
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Linker flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Default target
all: build

help: ## Display this help message
	@echo "CleanupCache - Makefile Commands"
	@echo "================================="
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cleanup
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: ## Install the binary to /usr/local/bin
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/cleanup
	@echo "Installed successfully"

run: build ## Build and run the application
	@$(BUILD_DIR)/$(BINARY_NAME)

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "Tests complete"

test-coverage: test ## Run tests and show coverage
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@echo "Formatting complete"

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "Linting complete"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.txt coverage.html
	go clean
	@echo "Clean complete"

# Cross-compilation targets
build-all: build-linux build-darwin ## Build for all platforms

build-linux: ## Build for Linux (amd64)
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux/$(BINARY_NAME) ./cmd/cleanup
	@echo "Linux build complete: $(BUILD_DIR)/linux/$(BINARY_NAME)"

build-darwin: ## Build for macOS (amd64 and arm64)
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin/$(BINARY_NAME)-amd64 ./cmd/cleanup
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/darwin/$(BINARY_NAME)-arm64 ./cmd/cleanup
	@echo "macOS builds complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

deps-upgrade: ## Upgrade dependencies
	@echo "Upgrading dependencies..."
	go get -u ./...
	go mod tidy
	@echo "Dependencies upgraded"

# Development helpers
dev: fmt vet test build ## Format, vet, test, and build

watch: ## Watch for changes and rebuild (requires entr)
	@echo "Watching for changes..."
	find . -name '*.go' | entr -r make run

# Release helpers
release-snapshot: ## Create a snapshot release (requires goreleaser)
	goreleaser release --snapshot --clean

release: ## Create a release (requires goreleaser)
	goreleaser release --clean
