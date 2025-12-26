#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Test configuration
export TEST_ENV="test"
export LOG_LEVEL="error"

echo -e "${GREEN}reconYa - Test Suite${NC}"
echo "=============================="

# Function to run tests
run_tests() {
    local test_type=$1
    local packages=$2
    
    echo -e "\n${YELLOW}Running $test_type tests...${NC}"
    
    if go test -v -timeout 30s $packages; then
        echo -e "${GREEN}✅ $test_type tests passed${NC}"
        return 0
    else
        echo -e "${RED}❌ $test_type tests failed${NC}"
        return 1
    fi
}

# Function to run tests with coverage
run_coverage() {
    echo -e "\n${YELLOW}Running tests with coverage...${NC}"
    
    if go test -v -timeout 30s -coverprofile=coverage.out ./...; then
        echo -e "${GREEN}✅ Tests completed${NC}"
        
        # Generate HTML coverage report
        go tool cover -html=coverage.out -o coverage.html
        echo -e "${GREEN}📊 Coverage report generated: coverage.html${NC}"
        
        # Show coverage summary
        echo -e "\n${YELLOW}Coverage Summary:${NC}"
        go tool cover -func=coverage.out | tail -1
        
        return 0
    else
        echo -e "${RED}❌ Tests failed${NC}"
        return 1
    fi
}

# Parse command line arguments
case "${1:-all}" in
    "unit")
        run_tests "Unit" "./models/... ./internal/..."
        ;;
    "integration")
        run_tests "Integration" "./tests/integration/..."
        ;;
    "coverage")
        run_coverage
        ;;
    "all")
        echo -e "${YELLOW}Running all tests...${NC}"
        
        success=true
        
        run_tests "Unit" "./models/... ./internal/..." || success=false
        run_tests "Integration" "./tests/integration/..." || success=false
        
        if [ "$success" = true ]; then
            echo -e "\n${GREEN}🎉 All tests passed!${NC}"
            exit 0
        else
            echo -e "\n${RED}💥 Some tests failed!${NC}"
            exit 1
        fi
        ;;
    "watch")
        echo -e "${YELLOW}Watching for file changes...${NC}"
        echo "Press Ctrl+C to stop"
        
        if command -v entr &> /dev/null; then
            find . -name "*.go" | entr -c ./test.sh unit
        else
            echo -e "${RED}Error: 'entr' command not found. Install it with: brew install entr${NC}"
            exit 1
        fi
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [COMMAND]"
        echo ""
        echo "Commands:"
        echo "  all          Run all tests (default)"
        echo "  unit         Run unit tests only"
        echo "  integration  Run integration tests only"
        echo "  coverage     Run tests with coverage report"
        echo "  watch        Watch for changes and run unit tests"
        echo "  help         Show this help message"
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac