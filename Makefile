# Variables
BINARY_NAME := die
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
LDFLAGS := -X github.com/olekukonko/die.Version=$(VERSION)
LDFLAGS += -X github.com/olekukonko/die.BuildTime=$(BUILD_TIME)
LDFLAGS += -X github.com/olekukonko/die.GitCommit=$(GIT_COMMIT)
LDFLAGS += -s -w

# Build directories
BUILD_DIR := ./build
DIST_DIR := ./dist

# Go commands
GO := go
GOFLAGS := -ldflags "$(LDFLAGS)"
GOTEST := $(GO) test
GOMOD := $(GO) mod
GOCLEAN := $(GO) clean

# Platforms for cross-compilation
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
TEMP_DIR := $(shell mktemp -d)

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

.PHONY: all
all: clean fmt lint test build ## Clean, format, lint, test, and build

.PHONY: build
build: ## Build the binary for current platform
	@echo "$(GREEN)Building $(BINARY_NAME) $(VERSION)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/die
	@echo "$(GREEN)Binary created: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: install
install: ## Install binary to $GOPATH/bin
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	$(GO) install $(GOFLAGS) ./cmd/die
	@echo "$(GREEN)Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)$(NC)"

.PHONY: run
run: build ## Build and run (example: make run ARGS="--dry 3000")
	@echo "$(GREEN)Running $(BINARY_NAME) $(ARGS)...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

.PHONY: test
test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)Tests complete$(NC)"

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "$(GREEN)Generating coverage report...$(NC)"
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

.PHONY: lint
lint: ## Run linters
	@echo "$(GREEN)Running linters...$(NC)"
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "$(YELLOW)golangci-lint not installed. Installing...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2; \
	}
	golangci-lint run ./...
	@echo "$(GREEN)Linting complete$(NC)"

.PHONY: fmt
fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)Format complete$(NC)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)Vet complete$(NC)"

.PHONY: tidy
tidy: ## Tidy go modules
	@echo "$(GREEN)Tidying modules...$(NC)"
	$(GOMOD) tidy
	@echo "$(GREEN)Modules tidied$(NC)"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning...$(NC)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out coverage.html
	@echo "$(GREEN)Clean complete$(NC)"

.PHONY: dist
dist: clean ## Build binaries for all platforms
	@echo "$(GREEN)Building for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output=$(DIST_DIR)/$(BINARY_NAME)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then \
			output=$$output.exe; \
		fi; \
		echo "$(YELLOW)Building $$os/$$arch...$(NC)"; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(GOFLAGS) -o $$output ./cmd/die; \
	done
	@echo "$(GREEN)Distribution builds created in $(DIST_DIR)/$(NC)"
	@ls -lh $(DIST_DIR)

.PHONY: release
release: dist ## Create release archives (requires dist)
	@echo "$(GREEN)Creating release archives...$(NC)"
	@for file in $(DIST_DIR)/*; do \
		if [ -f "$$file" ]; then \
			filename=$$(basename $$file); \
			echo "$(YELLOW)Archiving $$filename...$(NC)"; \
			cd $(DIST_DIR) && tar -czf $$filename.tar.gz $$filename && cd ..; \
		fi; \
	done
	@echo "$(GREEN)Release archives created in $(DIST_DIR)/$(NC)"
	@ls -lh $(DIST_DIR)/*.tar.gz

.PHONY: dev
dev: clean fmt tidy ## Setup development environment
	@echo "$(GREEN)Setting up development environment...$(NC)"
	$(GO) mod download
	@echo "$(GREEN)Development environment ready$(NC)"

.PHONY: watch
watch: ## Watch for changes and rebuild (requires fswatch)
	@command -v fswatch >/dev/null 2>&1 || { \
		echo "$(RED)fswatch not installed. Install with: brew install fswatch (macOS) or apt install fswatch (Linux)$(NC)"; \
		exit 1; \
	}
	@echo "$(GREEN)Watching for changes...$(NC)"
	@fswatch -o . --exclude .git --exclude build --exclude dist | xargs -n1 -I{} make build

.PHONY: help
help: ## Show this help message
	@echo "$(GREEN)die - Process Assassination Tool$(NC)"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make [target]"
	@echo ""
	@echo "$(YELLOW)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make build                     # Build binary for current platform"
	@echo "  make run ARGS=\"--dry 3000\"   # Build and run with dry-run"
	@echo "  make install                   # Install to GOPATH/bin"
	@echo "  make test                      # Run tests"
	@echo "  make dist                      # Build for all platforms"
	@echo "  make release                   # Create release archives"

# Default target
.DEFAULT_GOAL := help