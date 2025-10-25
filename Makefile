.PHONY: build run clean test fmt lint build-all install

# Binary name
BINARY=claude-code-proxy
CMD_PATH=cmd/$(BINARY)/main.go

# Build directory
BUILD_DIR=dist

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Build flags
LDFLAGS=-ldflags "-s -w"

# Default target
all: build

# Build the binary
build:
	@echo "🔨 Building $(BINARY)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY) $(CMD_PATH)
	@echo "✅ Build complete: ./$(BINARY)"

# Run the proxy
run: build
	@echo "🚀 Starting proxy..."
	./$(BINARY)

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean complete"

# Run tests
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

# Run benchmarks
bench:
	@echo "⚡ Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./internal/converter

# Format code
fmt:
	@echo "📝 Formatting code..."
	$(GOFMT) ./...

# Lint (requires golangci-lint)
lint:
	@echo "🔍 Linting..."
	golangci-lint run || echo "⚠️  Install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

# Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	$(GOMOD) download
	@echo "✅ Dependencies downloaded"

# Build for all platforms
build-all: clean
	@echo "🔨 Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 $(CMD_PATH)

	# macOS AMD64 (Intel)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 $(CMD_PATH)

	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(CMD_PATH)

	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 $(CMD_PATH)

	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(CMD_PATH)

	@echo "✅ Built binaries:"
	@ls -lh $(BUILD_DIR)/

# Install to system
install: build
	@echo "📦 Installing $(BINARY) to /usr/local/bin..."
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
	sudo cp scripts/ccp /usr/local/bin/ccp
	sudo chmod +x /usr/local/bin/ccp
	@echo "✅ Installed:"
	@echo "   - /usr/local/bin/$(BINARY)"
	@echo "   - /usr/local/bin/ccp (wrapper script)"
	@echo ""
	@echo "📋 Next steps:"
	@echo "  1. Create config: mkdir -p ~/.claude && cp .env.example ~/.claude/proxy.env"
	@echo "  2. Edit config: nano ~/.claude/proxy.env"
	@echo "  3. Run: ccp chat (or claude-code-proxy)"
