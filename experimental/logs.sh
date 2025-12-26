#!/bin/bash
set -e

# Colors for pretty output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check for docker compose v2 or docker-compose
if command -v docker compose &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    echo -e "${RED}Docker Compose is not installed. Please install Docker Compose first.${NC}"
    exit 1
fi

# Parse arguments
FOLLOW=false
SERVICE=""

while [[ $# -gt 0 ]]; do
  case $1 in
    -f|--follow)
      FOLLOW=true
      shift
      ;;
    reconya)
      SERVICE=$1
      shift
      ;;
    *)
      echo "Unknown argument: $1"
      echo "Usage: ./logs.sh [-f|--follow] [reconya]"
      exit 1
      ;;
  esac
done

# Construct command
CMD="$COMPOSE_CMD logs"
if $FOLLOW; then
  CMD="$CMD -f"
fi
if [[ -n $SERVICE ]]; then
  CMD="$CMD $SERVICE"
fi

# Display logs
echo -e "${YELLOW}Displaying logs for Reconya...${NC}"
echo -e "Press Ctrl+C to exit if following logs\n"
eval $CMD