.PHONY: build test test-verbose test-coverage test-race clean help check lint

BINARY_NAME=gollama
BUILD_DIR=build
GO=go

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

build:  ## Build gollama binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) .

check:  ## Run format, vet, lint, and tests
	@echo "Checking code formatting..."
	@test -z "$$($(GO) fmt ./...)" || (echo "Code formatting issues found. Run 'go fmt ./...' to fix." && exit 1)
	@echo "Running go vet..."
	@$(GO) vet ./...
	@echo "Running golangci-lint..."
	@golangci-lint run
	@echo "Running tests..."
	@$(GO) test ./...

lint:  ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run

test:  ## Run all tests
	@echo "Running tests..."
	$(GO) test ./...

test-verbose:  ## Run all tests with verbose output
	@echo "Running tests with verbose output..."
	$(GO) test -v ./...

test-coverage:  ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	$(GO) test -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated: $(BUILD_DIR)/coverage.html"

test-race:  ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GO) test -race ./...

clean:  ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	$(GO) clean

.DEFAULT_GOAL := help
