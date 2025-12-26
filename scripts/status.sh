#!/bin/bash

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
GRAY='\033[0;90m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"
PORT=3008

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if command exists and get version
check_dependency() {
    local name=$1
    local cmd=$2
    local version_flag=$3

    if command -v "$cmd" &> /dev/null; then
        version=$("$cmd" $version_flag 2>&1 | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1)
        if [[ -n "$version" ]]; then
            log_success "$name: $version"
        else
            log_warning "$name: installed but version check failed"
        fi
    else
        log_error "$name: not installed"
    fi
}

# Find process using port
find_process_by_port() {
    local port=$1
    if command -v lsof &> /dev/null; then
        lsof -ti :$port 2>/dev/null | head -1
    else
        netstat -tlnp 2>/dev/null | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | head -1
    fi
}

# Test HTTP endpoint
test_http() {
    local url=$1
    if command -v curl &> /dev/null; then
        status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 "$url" 2>/dev/null)
        [[ "$status" -lt 400 ]] && return 0
    fi
    return 1
}

echo ""
log_info "Checking dependencies..."

check_dependency "Go" "go" "version"
check_dependency "nmap" "nmap" "--version"

echo ""
log_info "Checking backend service..."

backend_pid=$(find_process_by_port $PORT)
if [[ -n "$backend_pid" ]]; then
    log_success "Backend: running (PID $backend_pid)"

    # Test web interface
    if test_http "http://localhost:$PORT"; then
        log_success "Backend web interface: accessible"
    else
        log_warning "Backend web interface: not accessible"
    fi

    # Test login endpoint
    if test_http "http://localhost:$PORT/login"; then
        log_success "Backend API: responding"
    else
        log_warning "Backend API: not responding"
    fi
else
    log_error "Backend: not running"
fi

echo ""
log_info "Checking configuration..."

# Check .env file
if [[ -f "$BACKEND_DIR/.env" ]]; then
    log_success "Backend .env: exists"

    # Parse username from .env
    username=$(grep -E "^LOGIN_USERNAME=" "$BACKEND_DIR/.env" 2>/dev/null | cut -d'=' -f2 | tr -d '"')
    if [[ -n "$username" ]]; then
        log_success "Login username: $username"
    else
        log_warning "Login username: not configured"
    fi
else
    log_error "Backend .env: missing"
fi

# Check go.mod
if [[ -f "$BACKEND_DIR/go.mod" ]]; then
    log_success "Backend dependencies: go.mod exists"
else
    log_error "Backend dependencies: go.mod missing"
fi

# Check templates directory
if [[ -d "$BACKEND_DIR/templates" ]]; then
    log_success "HTMX templates: directory exists"
else
    log_error "HTMX templates: directory missing"
fi

echo ""
