#!/bin/bash

# reconYa Installation Script
# Wrapper for make install

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

exec make install
