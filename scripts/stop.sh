#!/bin/bash

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PID_FILE="$PROJECT_ROOT/.reconya.pid"
PORT=3008
SILENT=false

# Parse arguments
if [[ "$1" == "--silent" ]]; then
    SILENT=true
fi

log_info() {
    if [[ "$SILENT" == false ]]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    if [[ "$SILENT" == false ]]; then
        echo -e "${GREEN}[SUCCESS]${NC} $1"
    fi
}

log_warning() {
    if [[ "$SILENT" == false ]]; then
        echo -e "${YELLOW}[WARNING]${NC} $1"
    fi
}

log_error() {
    if [[ "$SILENT" == false ]]; then
        echo -e "${RED}[ERROR]${NC} $1"
    fi
}

# Find process using port
find_process_by_port() {
    local port=$1
    if command -v lsof &> /dev/null; then
        lsof -ti :$port 2>/dev/null | head -1
    else
        # Fallback to netstat
        netstat -tlnp 2>/dev/null | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | head -1
    fi
}

# Kill a process gracefully, then force if needed
kill_process() {
    local pid=$1
    local name=$2

    if kill -0 "$pid" 2>/dev/null; then
        log_info "Stopping process $pid ($name)..."
        kill -TERM "$pid" 2>/dev/null

        # Wait up to 5 seconds for graceful shutdown
        for i in {1..10}; do
            if ! kill -0 "$pid" 2>/dev/null; then
                log_success "Process $pid stopped"
                return 0
            fi
            sleep 0.5
        done

        # Force kill if still running
        if kill -0 "$pid" 2>/dev/null; then
            log_warning "Process $pid still running, force killing..."
            kill -9 "$pid" 2>/dev/null
            sleep 1
        fi

        if ! kill -0 "$pid" 2>/dev/null; then
            log_success "Process $pid stopped"
            return 0
        else
            log_error "Failed to kill process $pid"
            return 1
        fi
    fi
    return 0
}

stopped_any=false

# Stop daemon by PID file
if [[ -f "$PID_FILE" ]]; then
    pid=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$pid" ]] && [[ "$pid" =~ ^[0-9]+$ ]]; then
        if kill -0 "$pid" 2>/dev/null; then
            kill_process "$pid" "daemon"
            stopped_any=true
        else
            log_warning "Daemon PID file exists but process $pid not found"
        fi
        rm -f "$PID_FILE"
    fi
fi

# Stop any process on port 3008
port_pid=$(find_process_by_port $PORT)
if [[ -n "$port_pid" ]]; then
    log_warning "Port $PORT is in use by process $port_pid"
    kill_process "$port_pid" "backend"
    stopped_any=true
fi

# Kill any remaining reconya Go processes
pkill -f "go run.*reconya" 2>/dev/null && stopped_any=true
pkill -f "reconya.*cmd" 2>/dev/null && stopped_any=true

# Final verification
sleep 1
final_pid=$(find_process_by_port $PORT)
if [[ -n "$final_pid" ]]; then
    log_error "Warning: Port $PORT is still occupied by process $final_pid"
    log_info "You may need to manually kill this process:"
    log_info "  kill -9 $final_pid"
else
    if [[ "$stopped_any" == true ]]; then
        log_success "reconYa backend stopped"
    else
        log_info "No reconYa backend was running"
    fi
fi
