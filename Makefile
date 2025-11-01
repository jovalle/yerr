.PHONY: help build test test-verbose test-coverage lint lint-fix lint-go lint-go-fix lint-md lint-md-fix fmt vet clean install run-cpu run-http deps bench check pre-commit release release-check

# Variables
BINARY_NAME=yerr
VERSION?=0.1.0
BUILD_DIR=bin
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

help: ## Display this help screen
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/yerr

install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) ./cmd/yerr

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	$(GOTEST) -v -race -count=1 ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

lint-go: ## Run Go linter
	@echo "Running Go linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

lint-go-fix: ## Run Go linter and fix issues
	@echo "Running Go linter with auto-fix..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run --fix ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint-md: ## Run markdown linter
	@echo "Running markdown linter..."
	@which markdownlint > /dev/null || (echo "markdownlint not installed. Install with: npm install -g markdownlint-cli" && exit 1)
	markdownlint '**/*.md'

lint-md-fix: ## Run markdown linter and fix issues
	@echo "Running markdown linter with auto-fix..."
	@which markdownlint > /dev/null || (echo "markdownlint not installed. Install with: npm install -g markdownlint-cli" && exit 1)
	markdownlint --fix '**/*.md'

lint: lint-go lint-md ## Run all linters (Go + Markdown)

lint-fix: lint-go-fix lint-md-fix ## Fix all linting issues (Go + Markdown)

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "All checks passed!"

pre-commit: fmt vet lint test ## Run pre-commit checks
	@echo "Pre-commit checks passed!"

# Release management
release-check: ## Verify release readiness (runs all checks)
	@echo "🔍 Checking release readiness..."
	@echo ""
	@echo "📋 Running all checks..."
	@$(MAKE) check
	@echo ""
	@echo "📦 Checking git status..."
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "❌ Error: Working directory is not clean. Commit or stash changes first."; \
		exit 1; \
	fi
	@echo "✅ Working directory is clean"
	@echo ""
	@echo "🌿 Checking branch..."
	@BRANCH=$$(git branch --show-current); \
	if [ "$$BRANCH" != "main" ]; then \
		echo "⚠️  Warning: Not on main branch (current: $$BRANCH)"; \
		echo "   Releases are typically created from main branch"; \
	else \
		echo "✅ On main branch"; \
	fi
	@echo ""
	@echo "🔄 Checking if branch is up to date with remote..."
	@git fetch origin main --quiet || true
	@LOCAL=$$(git rev-parse @); \
	REMOTE=$$(git rev-parse @{u} 2>/dev/null || echo ""); \
	if [ -n "$$REMOTE" ] && [ "$$LOCAL" != "$$REMOTE" ]; then \
		echo "⚠️  Warning: Branch is not up to date with remote"; \
		echo "   Run 'git pull' before creating a release"; \
	else \
		echo "✅ Branch is up to date"; \
	fi
	@echo ""
	@echo "✨ Release checks passed! Ready to create a release."
	@echo ""
	@echo "Next steps:"
	@echo "  1. Run: make release VERSION=v1.0.0"
	@echo "  2. Or manually: git tag -a v1.0.0 -m \"Release v1.0.0\" && git push origin v1.0.0"

release: ## Create and push a release tag (usage: make release VERSION=v1.0.0)
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ Error: VERSION is required"; \
		echo "Usage: make release VERSION=v1.0.0"; \
		exit 1; \
	fi
	@echo "🚀 Creating release $(VERSION)..."
	@echo ""
	@echo "📋 Running release checks..."
	@$(MAKE) release-check
	@echo ""
	@echo "🏷️  Creating tag $(VERSION)..."
	@if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "❌ Error: Tag $(VERSION) already exists"; \
		exit 1; \
	fi
	@git tag -a "$(VERSION)" -m "Release $(VERSION)"
	@echo "✅ Tag created: $(VERSION)"
	@echo ""
	@echo "📤 Pushing tag to origin..."
	@git push origin "$(VERSION)"
	@echo ""
	@echo "✅ Release $(VERSION) created successfully!"
	@echo ""
	@echo "📦 GitHub Actions will now:"
	@echo "   • Build binaries for all platforms"
	@echo "   • Create a GitHub release"
	@echo "   • Generate release notes"
	@echo ""
	@echo "🔗 Check progress at: https://github.com/jovalle/yerr/actions"

# Example run targets
run-cpu: build ## Run CPU test example
	@echo "Running CPU load test example..."
	$(BUILD_DIR)/$(BINARY_NAME) cpu -w 4 -d 5

run-http: build ## Run HTTP test example (requires a running server)
	@echo "Running HTTP load test example..."
	$(BUILD_DIR)/$(BINARY_NAME) http https://example.com -w 10 -n 100

# Cross-compilation targets
build-all: build-linux build-darwin build-windows ## Build for all platforms

build-linux: ## Build for Linux
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/yerr

build-darwin: ## Build for macOS
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/yerr
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/yerr

build-windows: ## Build for Windows
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/yerr

.DEFAULT_GOAL := help
