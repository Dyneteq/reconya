.PHONY: dev build deps clean test fmt help

# Project paths
PROJECT_ROOT := $(shell pwd)
SRC_DIR := $(PROJECT_ROOT)/src
BINARY := $(PROJECT_ROOT)/reconya

# Colors
GREEN := \033[0;32m
BLUE := \033[0;34m
NC := \033[0m

## Run in development mode
dev:
	@echo "$(BLUE)[INFO]$(NC) Starting reconYa on http://localhost:3008"
	@cd $(SRC_DIR) && go run ./cmd

## Build binary (CGO enabled for SQLite)
build:
	@echo "Building reconYa..."
	@cd $(SRC_DIR) && CGO_ENABLED=1 go build -o $(BINARY) -v ./cmd
	@echo "$(GREEN)[SUCCESS]$(NC) Build complete: $(BINARY)"

## Download dependencies
deps:
	@cd $(SRC_DIR) && go mod download && go mod tidy
	@echo "$(GREEN)[SUCCESS]$(NC) Dependencies updated"

## Clean build artifacts
clean:
	@cd $(SRC_DIR) && go clean
	@rm -f $(BINARY)
	@echo "$(GREEN)[SUCCESS]$(NC) Clean complete"

## Run tests
test:
	@cd $(SRC_DIR) && go test -v ./...

## Format code
fmt:
	@cd $(SRC_DIR) && go fmt ./...

## Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "  dev     Run in development mode"
	@echo "  build   Build binary"
	@echo "  deps    Download dependencies"
	@echo "  clean   Clean build artifacts"
	@echo "  test    Run tests"
	@echo "  fmt     Format code"
	@echo "  help    Show this help"
