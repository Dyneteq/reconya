.PHONY: test test-unit test-integration test-coverage clean build run

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=reconya
COVERAGE_FILE=coverage.out

# Test parameters
TEST_TIMEOUT=30s
TEST_PACKAGES=./...
UNIT_TEST_PACKAGES=./models/... ./internal/...
INTEGRATION_TEST_PACKAGES=./tests/integration/...

# Build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd

# Build for Windows (with CGO enabled for SQLite)
build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME).exe -v ./cmd

# Build with CGO enabled (required for SQLite)
build-cgo:
	CGO_ENABLED=1 $(GOBUILD) -o $(BINARY_NAME) -v ./cmd

# Run application
run:
	$(GOCMD) run ./cmd

# Clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE)

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run all tests
test: test-unit test-integration

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) $(UNIT_TEST_PACKAGES)

# Run integration tests only
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) $(INTEGRATION_TEST_PACKAGES)

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) -coverprofile=$(COVERAGE_FILE) $(TEST_PACKAGES)
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests in watch mode (requires entr)
test-watch:
	@echo "Watching for changes and running tests..."
	find . -name "*.go" | entr -c make test-unit

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -v -race -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem $(TEST_PACKAGES)

# Lint code (requires golangci-lint)
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	$(GOCMD) fmt $(TEST_PACKAGES)

# Vet code
vet:
	$(GOCMD) vet $(TEST_PACKAGES)

# Check for security issues (requires gosec)
security:
	@echo "Running security check..."
	gosec ./...

# Run all quality checks
quality: fmt vet lint security test-coverage

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  build-windows - Build for Windows with CGO enabled"
	@echo "  build-cgo     - Build with CGO enabled (required for SQLite)"
	@echo "  run           - Run the application"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  test          - Run all tests"
	@echo "  test-unit     - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-watch    - Watch for changes and run tests"
	@echo "  test-race     - Run tests with race detection"
	@echo "  bench         - Run benchmarks"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code"
	@echo "  security      - Run security check"
	@echo "  quality       - Run all quality checks"
	@echo "  help          - Show this help"