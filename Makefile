.PHONY: start start-dev stop status logs logs-follow logs-errors logs-clear build build-cgo deps clean help install setup-nmap bump-version

# Project paths
PROJECT_ROOT := $(shell pwd)
BACKEND_DIR := $(PROJECT_ROOT)/backend
SCRIPTS_DIR := $(PROJECT_ROOT)/scripts
LOGS_DIR := $(PROJECT_ROOT)/logs
PID_FILE := $(PROJECT_ROOT)/.reconya.pid
LOG_FILE := $(LOGS_DIR)/reconya.log
ERROR_LOG := $(LOGS_DIR)/reconya.error.log

# Default port
PORT ?= 3008

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
BLUE := \033[0;34m
NC := \033[0m

#-----------------------------------------------------------------------
# Main targets
#-----------------------------------------------------------------------

## Start reconYa backend as daemon
start:
	@echo "=========================================="
	@echo "         Starting reconYa Backend         "
	@echo "=========================================="
	@mkdir -p $(LOGS_DIR)
	@$(MAKE) -s stop-silent
	@echo "$(BLUE)[INFO]$(NC) Starting backend as daemon..."
	@cd $(BACKEND_DIR) && nohup go run ./cmd > $(LOG_FILE) 2> $(ERROR_LOG) & echo $$! > $(PID_FILE)
	@sleep 2
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(GREEN)[SUCCESS]$(NC) reconYa daemon started with PID: $$(cat $(PID_FILE))"; \
		echo ""; \
		echo "Access reconYa at: http://localhost:$(PORT)"; \
		echo "Default login: admin / password"; \
		echo ""; \
		echo "Logs: $(LOG_FILE)"; \
		echo "Errors: $(ERROR_LOG)"; \
	else \
		echo "$(RED)[ERROR]$(NC) Failed to start daemon"; \
		exit 1; \
	fi

## Start reconYa backend in foreground (dev mode)
start-dev:
	@echo "=========================================="
	@echo "      Starting reconYa Backend (dev)      "
	@echo "=========================================="
	@$(MAKE) -s stop-silent
	@echo "$(BLUE)[INFO]$(NC) reconYa backend will run on: http://localhost:$(PORT)"
	@echo "$(BLUE)[INFO]$(NC) Press Ctrl+C to stop the service"
	@echo ""
	@cd $(BACKEND_DIR) && go run ./cmd

## Stop reconYa backend
stop:
	@echo "=========================================="
	@echo "         Stopping reconYa Backend         "
	@echo "=========================================="
	@$(SCRIPTS_DIR)/stop.sh

stop-silent:
	@$(SCRIPTS_DIR)/stop.sh --silent 2>/dev/null || true

## Check reconYa service status
status:
	@echo "=========================================="
	@echo "          reconYa Service Status          "
	@echo "=========================================="
	@$(SCRIPTS_DIR)/status.sh

## View daemon logs
logs:
	@if [ -f $(LOG_FILE) ]; then \
		cat $(LOG_FILE); \
	else \
		echo "$(YELLOW)[WARNING]$(NC) No log file found. Service may not be running in daemon mode."; \
	fi

## Follow daemon logs (tail -f)
logs-follow:
	@echo "$(BLUE)[INFO]$(NC) Following logs (Press Ctrl+C to exit)..."
	@if [ -f $(LOG_FILE) ]; then \
		tail -f $(LOG_FILE); \
	else \
		echo "$(YELLOW)[WARNING]$(NC) No log file found. Creating and waiting..."; \
		mkdir -p $(LOGS_DIR); \
		touch $(LOG_FILE); \
		tail -f $(LOG_FILE); \
	fi

## View error logs
logs-errors:
	@if [ -f $(ERROR_LOG) ]; then \
		cat $(ERROR_LOG); \
	else \
		echo "$(YELLOW)[WARNING]$(NC) No error log file found."; \
	fi

## Clear all log files
logs-clear:
	@echo "Clearing daemon logs..."
	@if [ -f $(LOG_FILE) ]; then > $(LOG_FILE); echo "$(GREEN)[SUCCESS]$(NC) Application logs cleared"; fi
	@if [ -f $(ERROR_LOG) ]; then > $(ERROR_LOG); echo "$(GREEN)[SUCCESS]$(NC) Error logs cleared"; fi
	@echo "$(GREEN)[SUCCESS]$(NC) All logs cleared"

#-----------------------------------------------------------------------
# Build targets
#-----------------------------------------------------------------------

## Build the backend binary
build:
	@echo "Building reconYa backend..."
	@cd $(BACKEND_DIR) && go build -o reconya -v ./cmd
	@echo "$(GREEN)[SUCCESS]$(NC) Build complete: $(BACKEND_DIR)/reconya"

## Build with CGO enabled (required for SQLite)
build-cgo:
	@echo "Building reconYa backend with CGO..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=1 go build -o reconya -v ./cmd
	@echo "$(GREEN)[SUCCESS]$(NC) Build complete: $(BACKEND_DIR)/reconya"

## Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	@cd $(BACKEND_DIR) && go mod download && go mod tidy
	@echo "$(GREEN)[SUCCESS]$(NC) Dependencies updated"

## Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@cd $(BACKEND_DIR) && go clean
	@rm -f $(BACKEND_DIR)/reconya
	@rm -f $(PID_FILE)
	@echo "$(GREEN)[SUCCESS]$(NC) Clean complete"

#-----------------------------------------------------------------------
# Setup targets
#-----------------------------------------------------------------------

## Initial setup - create .env file if needed
install:
	@echo "=========================================="
	@echo "          reconYa Installation            "
	@echo "=========================================="
	@if [ ! -f $(BACKEND_DIR)/.env ]; then \
		if [ -f $(BACKEND_DIR)/.env.example ]; then \
			echo "$(BLUE)[INFO]$(NC) Creating .env from example..."; \
			cp $(BACKEND_DIR)/.env.example $(BACKEND_DIR)/.env; \
		else \
			echo "$(BLUE)[INFO]$(NC) Creating default .env..."; \
			echo 'LOGIN_USERNAME=admin' > $(BACKEND_DIR)/.env; \
			echo 'LOGIN_PASSWORD=password' >> $(BACKEND_DIR)/.env; \
			echo 'DATABASE_NAME="reconya-dev"' >> $(BACKEND_DIR)/.env; \
			echo "JWT_SECRET_KEY=\"$$(openssl rand -base64 32)\"" >> $(BACKEND_DIR)/.env; \
			echo 'SQLITE_PATH="data/reconya-dev.db"' >> $(BACKEND_DIR)/.env; \
		fi; \
		echo "$(GREEN)[SUCCESS]$(NC) .env file created"; \
	else \
		echo "$(GREEN)[SUCCESS]$(NC) .env file already exists"; \
	fi
	@$(MAKE) deps
	@echo ""
	@echo "$(GREEN)[SUCCESS]$(NC) Installation complete!"
	@echo "Run 'make start' to start reconYa"

## Setup nmap permissions for MAC address detection
setup-nmap:
	@echo "Setting up nmap permissions..."
	@NMAP_PATH=$$(which nmap) && \
	if [ -n "$$NMAP_PATH" ]; then \
		echo "$(BLUE)[INFO]$(NC) Found nmap at: $$NMAP_PATH"; \
		sudo chown root:wheel "$$NMAP_PATH" && \
		sudo chmod u+s "$$NMAP_PATH" && \
		echo "$(GREEN)[SUCCESS]$(NC) nmap permissions configured"; \
	else \
		echo "$(RED)[ERROR]$(NC) nmap not found. Please install nmap first."; \
		exit 1; \
	fi

## Bump version
bump-version:
	@$(SCRIPTS_DIR)/bump-version.sh

#-----------------------------------------------------------------------
# Help
#-----------------------------------------------------------------------

## Show this help
help:
	@echo "reconYa Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Main targets:"
	@echo "  start        Start reconYa backend as daemon"
	@echo "  start-dev    Start in foreground (development mode)"
	@echo "  stop         Stop reconYa backend"
	@echo "  status       Check service status"
	@echo ""
	@echo "Log targets:"
	@echo "  logs         View daemon logs"
	@echo "  logs-follow  Follow logs (tail -f)"
	@echo "  logs-errors  View error logs"
	@echo "  logs-clear   Clear all log files"
	@echo ""
	@echo "Build targets:"
	@echo "  build        Build the backend binary"
	@echo "  build-cgo    Build with CGO (for SQLite)"
	@echo "  deps         Download dependencies"
	@echo "  clean        Clean build artifacts"
	@echo ""
	@echo "Setup targets:"
	@echo "  install      Initial setup"
	@echo "  setup-nmap   Configure nmap permissions"
	@echo "  bump-version Bump project version"
