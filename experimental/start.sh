#!/bin/bash
set -e

# Colors for pretty output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting reconYa...${NC}"

# Check if docker and docker-compose are installed
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed. Please install Docker first.${NC}"
    echo "Visit https://docs.docker.com/engine/install/ for installation instructions."
    exit 1
fi

# Check for docker compose v2 or docker-compose
if command -v docker compose &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    echo -e "${RED}Docker Compose is not installed. Please install Docker Compose first.${NC}"
    echo "Visit https://docs.docker.com/compose/install/ for installation instructions."
    exit 1
fi

# Check if .env file exists
if [ ! -f .env ]; then
    echo -e "${YELLOW}No .env file found. Run ./setup.sh first or create an .env file manually.${NC}"
    exit 1
fi

# Load environment variables
source .env

# Check if we need to rebuild by comparing source code modification times with image creation times
REBUILD_FLAG=""

# Check if images exist and compare with source file modification times
if ! docker image inspect reconya &> /dev/null; then
    echo -e "${YELLOW}Image not found, will rebuild...${NC}"
    REBUILD_FLAG="--build"
else
    # Get the creation time of the backend image
    IMAGE_TIME=$(docker image inspect reconya --format='{{.Created}}' 2>/dev/null || echo "1970-01-01T00:00:00Z")
    
    # Check if any backend source files are newer than the image
    if find src -name "*.go" -newer <(date -d "$IMAGE_TIME" +"%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "1970-01-01 00:00:00") 2>/dev/null | grep -q .; then
        echo -e "${YELLOW}Source files changed, will rebuild...${NC}"
        REBUILD_FLAG="--build"
    # Check if docker-compose.yml or Dockerfile changed
    elif [ "docker-compose.yml" -nt <(date -d "$IMAGE_TIME" +"%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "1970-01-01 00:00:00") ] || 
         [ "src/Dockerfile" -nt <(date -d "$IMAGE_TIME" +"%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "1970-01-01 00:00:00") ]; then
        echo -e "${YELLOW}Docker configuration changed, will rebuild...${NC}"
        REBUILD_FLAG="--build"
    else
        echo -e "${GREEN}No source changes detected, using existing images...${NC}"
    fi
fi

# Start the application
echo -e "${YELLOW}Starting containers...${NC}"

# Try with rebuild flag, fallback to no-build if it fails
if ! $COMPOSE_CMD up -d $REBUILD_FLAG; then
    echo -e "${RED}Build failed (likely network/proxy issue), trying with existing images...${NC}"
    $COMPOSE_CMD up -d --no-build
fi

echo -e "\n${GREEN}reconYa is now running!${NC}"
echo -e "Access the application at: ${YELLOW}http://localhost:${PORT:-3008}${NC}"
echo
echo -e "To view logs, run: ${YELLOW}./logs.sh${NC}"
echo -e "To stop the application, run: ${YELLOW}./stop.sh${NC}"