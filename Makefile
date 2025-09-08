# Makefile for Gosinble - Go Automation Library & CLI
# ==================================================

# Project Configuration
PROJECT_NAME := gosinble
CLI_BINARY := gosinble
MODULE := github.com/liliang-cn/gosinble

# Build Configuration
BUILD_DIR := build
DIST_DIR := dist
COVERAGE_DIR := coverage
DOCS_PORT := 6060

# Version Information (injected at build time)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Go Build Flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) \
	-X main.buildTime=$(BUILD_TIME) \
	-X main.gitCommit=$(GIT_COMMIT) \
	-X main.gitBranch=$(GIT_BRANCH) \
	-s -w"

# Go Configuration
GO := go
GOFLAGS := -race -v
GOTAGS := 
GOTEST := $(GO) test $(GOFLAGS)
GOBUILD := $(GO) build $(LDFLAGS)
GOCLEAN := $(GO) clean
GOMOD := $(GO) mod
GOFMT := gofmt
GOVET := $(GO) vet

# Platform Configuration
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 freebsd/amd64
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

# Tool Configuration (auto-detect available tools)
GOLINT := $(shell which golint 2>/dev/null)
STATICCHECK := $(shell which staticcheck 2>/dev/null)
GOSEC := $(shell which gosec 2>/dev/null)
ENTR := $(shell which entr 2>/dev/null)
GODOC := $(shell which godoc 2>/dev/null)

# Colors for output (if terminal supports it)
ifneq ($(TERM),)
	GREEN := \033[32m
	YELLOW := \033[33m
	RED := \033[31m
	BLUE := \033[34m
	RESET := \033[0m
else
	GREEN := 
	YELLOW := 
	RED := 
	BLUE := 
	RESET := 
endif

# Default target
.DEFAULT_GOAL := help

# Phony targets
.PHONY: help info build build-examples build-cross install clean dev-setup \
        test test-unit test-integration test-coverage test-watch benchmark \
        fmt fmt-check tidy vendor lint staticcheck gosec check \
        run-example list-examples docs serve-docs serve-coverage \
        release-build release-checksums tag tools \
        quick ci profile-cpu profile-mem profile-trace docker

##@ General

help: ## Display this help message
	@echo "$(BLUE)Gosinble - Go Automation Library & CLI$(RESET)"
	@echo "=========================================="
	@echo ""
	@echo "$(GREEN)Usage:$(RESET) make <target> [options]"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "\nAvailable targets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-15s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(BLUE)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)Examples:$(RESET)"
	@echo "  make quick                    # Format, test, and build"
	@echo "  make test-coverage           # Run tests with HTML coverage report"
	@echo "  make run-example EXAMPLE=library-usage"
	@echo "  make build-cross            # Cross-compile for all platforms"
	@echo "  make release-build          # Build release binaries"
	@echo ""
	@echo "$(YELLOW)Environment Variables:$(RESET)"
	@echo "  VERSION=v1.0.0              # Override version"
	@echo "  GOTAGS=integration          # Build/test tags"
	@echo "  EXAMPLE=name                # Example to run"

info: ## Show project information and statistics
	@echo "$(BLUE)Project Information$(RESET)"
	@echo "==================="
	@echo "Name:       $(PROJECT_NAME)"
	@echo "Module:     $(MODULE)"
	@echo "Version:    $(VERSION)"
	@echo "Platform:   $(OS)/$(ARCH)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Branch: $(GIT_BRANCH)"
	@echo ""
	@echo "$(BLUE)Code Statistics$(RESET)"
	@echo "==============="
	@echo -n "Go files:      "; find . -name "*.go" | grep -v vendor | wc -l
	@echo -n "Lines of code: "; find . -name "*.go" | grep -v vendor | xargs wc -l | tail -1 | awk '{print $$1}'
	@echo -n "Packages:      "; go list ./... | wc -l
	@echo -n "Examples:      "; find examples/ -name "main.go" | wc -l

##@ Build

build: ## Build the main CLI binary
	@echo "$(GREEN)Building $(CLI_BINARY) for $(OS)/$(ARCH)...$(RESET)"
	$(GOBUILD) -tags "$(GOTAGS)" -o $(BUILD_DIR)/$(CLI_BINARY) ./cmd/$(CLI_BINARY)
	@echo "$(GREEN)✓ Binary built: $(BUILD_DIR)/$(CLI_BINARY)$(RESET)"

build-examples: ## Build all example binaries
	@echo "$(GREEN)Building all examples...$(RESET)"
	@mkdir -p $(BUILD_DIR)/examples
	@for example in $$(find examples/ -name "main.go" -exec dirname {} \;); do \
		name=$$(basename $$example); \
		echo "  Building $$name..."; \
		$(GOBUILD) -tags "$(GOTAGS)" -o $(BUILD_DIR)/examples/$$name ./$$example; \
	done
	@echo "$(GREEN)✓ All examples built in $(BUILD_DIR)/examples/$(RESET)"

build-cross: ## Cross-compile for multiple platforms
	@echo "$(GREEN)Cross-compiling for multiple platforms...$(RESET)"
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		OS_ARCH=$$(echo $$platform | tr "/" "-"); \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		EXT=""; \
		if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
		echo "  Building for $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build $(LDFLAGS) -tags "$(GOTAGS)" \
			-o $(DIST_DIR)/$(CLI_BINARY)-$$OS_ARCH$$EXT ./cmd/$(CLI_BINARY); \
	done
	@echo "$(GREEN)✓ Cross-compilation complete in $(DIST_DIR)/$(RESET)"

install: build ## Install CLI binary to GOPATH/bin
	@echo "$(GREEN)Installing $(CLI_BINARY)...$(RESET)"
	$(GO) install $(LDFLAGS) -tags "$(GOTAGS)" ./cmd/$(CLI_BINARY)
	@echo "$(GREEN)✓ $(CLI_BINARY) installed$(RESET)"

clean: ## Clean build artifacts and caches
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR)
	rm -f *.prof *.trace
	@echo "$(GREEN)✓ Clean complete$(RESET)"

##@ Development

dev-setup: tools ## One-time development environment setup
	@echo "$(GREEN)Setting up development environment...$(RESET)"
	$(GOMOD) download
	$(GOMOD) verify
	@echo "$(GREEN)✓ Development environment ready$(RESET)"

fmt: ## Format all Go code
	@echo "$(GREEN)Formatting code...$(RESET)"
	$(GOFMT) -w -s $$(find . -type f -name "*.go" | grep -v vendor)
	@echo "$(GREEN)✓ Code formatted$(RESET)"

fmt-check: ## Check if code is formatted
	@echo "$(GREEN)Checking code formatting...$(RESET)"
	@unformatted=$$($(GOFMT) -l $$(find . -type f -name "*.go" | grep -v vendor)); \
	if [ -n "$$unformatted" ]; then \
		echo "$(RED)✗ Unformatted files found:$(RESET)"; \
		echo "$$unformatted"; \
		exit 1; \
	else \
		echo "$(GREEN)✓ All files are formatted$(RESET)"; \
	fi

tidy: ## Update and clean dependencies
	@echo "$(GREEN)Tidying dependencies...$(RESET)"
	$(GOMOD) tidy
	$(GOMOD) verify
	@echo "$(GREEN)✓ Dependencies tidied$(RESET)"

vendor: ## Create vendor directory
	@echo "$(GREEN)Creating vendor directory...$(RESET)"
	$(GOMOD) vendor
	@echo "$(GREEN)✓ Vendor directory created$(RESET)"

##@ Testing

test: ## Run all tests with race detection
	@echo "$(GREEN)Running all tests...$(RESET)"
	$(GOTEST) -tags "$(GOTAGS)" ./...
	@echo "$(GREEN)✓ All tests passed$(RESET)"

test-unit: ## Run unit tests only (exclude integration)
	@echo "$(GREEN)Running unit tests...$(RESET)"
	$(GOTEST) -short -tags "$(GOTAGS)" ./...
	@echo "$(GREEN)✓ Unit tests passed$(RESET)"

test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(RESET)"
	$(GOTEST) -tags "integration,$(GOTAGS)" ./tests/...
	@echo "$(GREEN)✓ Integration tests passed$(RESET)"

test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -short -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic -tags "$(GOTAGS)" ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)✓ Coverage report generated: $(COVERAGE_DIR)/coverage.html$(RESET)"

test-watch: ## Continuously run tests on file changes (requires entr)
ifeq ($(ENTR),)
	@echo "$(RED)✗ entr not found. Install with: brew install entr (macOS) or apt-get install entr (Linux)$(RESET)"
	@exit 1
else
	@echo "$(GREEN)Watching for file changes... (Press Ctrl+C to stop)$(RESET)"
	find . -name "*.go" | grep -v vendor | $(ENTR) -c make test-unit
endif

benchmark: ## Run performance benchmarks
	@echo "$(GREEN)Running benchmarks...$(RESET)"
	$(GO) test -bench=. -benchmem -tags "$(GOTAGS)" ./...
	@echo "$(GREEN)✓ Benchmarks complete$(RESET)"

##@ Quality

lint: ## Run go vet and golint
	@echo "$(GREEN)Running linters...$(RESET)"
	$(GOVET) ./...
ifneq ($(GOLINT),)
	$(GOLINT) ./...
else
	@echo "$(YELLOW)⚠ golint not found. Install with: go install golang.org/x/lint/golint@latest$(RESET)"
endif
	@echo "$(GREEN)✓ Linting complete$(RESET)"

staticcheck: ## Run static analysis with staticcheck
ifneq ($(STATICCHECK),)
	@echo "$(GREEN)Running static analysis...$(RESET)"
	$(STATICCHECK) ./...
	@echo "$(GREEN)✓ Static analysis complete$(RESET)"
else
	@echo "$(YELLOW)⚠ staticcheck not found. Install with: go install honnef.co/go/tools/cmd/staticcheck@latest$(RESET)"
endif

gosec: ## Run security scanner
ifneq ($(GOSEC),)
	@echo "$(GREEN)Running security scan...$(RESET)"
	$(GOSEC) ./...
	@echo "$(GREEN)✓ Security scan complete$(RESET)"
else
	@echo "$(YELLOW)⚠ gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest$(RESET)"
endif

check: fmt-check lint staticcheck gosec test-unit ## Run all quality checks
	@echo "$(GREEN)✓ All quality checks passed$(RESET)"

##@ Examples

run-example: ## Run a specific example (usage: make run-example EXAMPLE=library-usage)
ifndef EXAMPLE
	@echo "$(RED)✗ EXAMPLE variable is required$(RESET)"
	@echo "Usage: make run-example EXAMPLE=library-usage"
	@echo "Available examples:"
	@$(MAKE) list-examples
	@exit 1
endif
	@echo "$(GREEN)Running example: $(EXAMPLE)$(RESET)"
	$(GO) run ./examples/$(EXAMPLE)

list-examples: ## List all available examples
	@echo "$(BLUE)Available Examples:$(RESET)"
	@find examples/ -name "main.go" -exec dirname {} \; | sed 's|examples/||' | sort | sed 's/^/  • /'

##@ Documentation

docs: ## Generate Go documentation
	@echo "$(GREEN)Generating documentation...$(RESET)"
	$(GO) doc -all ./... > docs.txt
	@echo "$(GREEN)✓ Documentation saved to docs.txt$(RESET)"

serve-docs: ## Serve documentation locally with godoc
ifneq ($(GODOC),)
	@echo "$(GREEN)Starting documentation server on http://localhost:$(DOCS_PORT)$(RESET)"
	@echo "$(YELLOW)Press Ctrl+C to stop the server$(RESET)"
	$(GODOC) -http=:$(DOCS_PORT)
else
	@echo "$(YELLOW)⚠ godoc not found. Install with: go install golang.org/x/tools/cmd/godoc@latest$(RESET)"
	@echo "Alternative: Use 'go doc' command for inline documentation"
endif

serve-coverage: test-coverage ## Open coverage report in browser
	@if [ -f $(COVERAGE_DIR)/coverage.html ]; then \
		echo "$(GREEN)Opening coverage report...$(RESET)"; \
		case "$(OS)" in \
			darwin) open $(COVERAGE_DIR)/coverage.html ;; \
			linux) xdg-open $(COVERAGE_DIR)/coverage.html ;; \
			*) echo "$(YELLOW)Coverage report: $(COVERAGE_DIR)/coverage.html$(RESET)" ;; \
		esac \
	else \
		echo "$(RED)✗ Coverage report not found. Run 'make test-coverage' first$(RESET)"; \
	fi

##@ Release

release-build: clean build-cross ## Build release binaries for all platforms
	@echo "$(GREEN)Creating release packages...$(RESET)"
	@cd $(DIST_DIR) && for binary in $(CLI_BINARY)-*; do \
		if [ -f "$$binary" ]; then \
			echo "  Packaging $$binary..."; \
			tar czf "$$binary.tar.gz" "$$binary"; \
		fi \
	done
	@echo "$(GREEN)✓ Release packages created in $(DIST_DIR)/$(RESET)"

release-checksums: ## Generate SHA256 checksums for release files
	@echo "$(GREEN)Generating checksums...$(RESET)"
	@cd $(DIST_DIR) && sha256sum *.tar.gz > SHA256SUMS
	@echo "$(GREEN)✓ Checksums generated: $(DIST_DIR)/SHA256SUMS$(RESET)"

tag: ## Create and push git tag (usage: make tag VERSION=v1.0.0)
ifndef VERSION
	@echo "$(RED)✗ VERSION is required$(RESET)"
	@echo "Usage: make tag VERSION=v1.0.0"
	@exit 1
endif
	@echo "$(GREEN)Creating tag $(VERSION)...$(RESET)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "$(GREEN)✓ Tag $(VERSION) created and pushed$(RESET)"

##@ Tools

tools: ## Install/update development tools
	@echo "$(GREEN)Installing development tools...$(RESET)"
	$(GO) install golang.org/x/lint/golint@latest
	$(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	$(GO) install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	$(GO) install golang.org/x/tools/cmd/godoc@latest
	@echo "$(GREEN)✓ Development tools installed$(RESET)"

##@ Shortcuts

quick: fmt test build ## Quick development workflow: format, test, and build
	@echo "$(GREEN)✓ Quick workflow complete$(RESET)"

ci: fmt-check lint staticcheck test-unit ## Continuous integration workflow
	@echo "$(GREEN)✓ CI workflow complete$(RESET)"

##@ Profiling

profile-cpu: ## Run CPU profiling
	@echo "$(GREEN)Running CPU profiling...$(RESET)"
	$(GO) test -cpuprofile=cpu.prof -bench=. ./...
	$(GO) tool pprof cpu.prof
	@echo "$(GREEN)✓ CPU profiling complete$(RESET)"

profile-mem: ## Run memory profiling
	@echo "$(GREEN)Running memory profiling...$(RESET)"
	$(GO) test -memprofile=mem.prof -bench=. ./...
	$(GO) tool pprof mem.prof
	@echo "$(GREEN)✓ Memory profiling complete$(RESET)"

profile-trace: ## Run execution tracing
	@echo "$(GREEN)Running execution tracing...$(RESET)"
	$(GO) test -trace=trace.out -bench=. ./...
	$(GO) tool trace trace.out
	@echo "$(GREEN)✓ Execution tracing complete$(RESET)"

##@ Docker (if Dockerfile exists)

docker: ## Build Docker image
	@if [ -f Dockerfile ]; then \
		echo "$(GREEN)Building Docker image...$(RESET)"; \
		docker build -t $(PROJECT_NAME):$(VERSION) .; \
		echo "$(GREEN)✓ Docker image built: $(PROJECT_NAME):$(VERSION)$(RESET)"; \
	else \
		echo "$(YELLOW)⚠ Dockerfile not found$(RESET)"; \
	fi