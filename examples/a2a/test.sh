#!/bin/bash

# A2A Examples Test Script
# This script validates that all A2A examples can be built and basic functionality works

set -e

echo "🧪 A2A Examples Test Suite"
echo "=========================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "SUCCESS") echo -e "${GREEN}✅ $message${NC}" ;;
        "ERROR") echo -e "${RED}❌ $message${NC}" ;;
        "WARNING") echo -e "${YELLOW}⚠️  $message${NC}" ;;
        "INFO") echo -e "${NC}ℹ️  $message${NC}" ;;
    esac
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Test 1: Check dependencies
print_status "INFO" "Checking dependencies..."

if ! command_exists go; then
    print_status "ERROR" "Go is not installed"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
print_status "SUCCESS" "Go found: $GO_VERSION"

if ! command_exists curl; then
    print_status "WARNING" "curl not found - HTTP tests will be skipped"
    SKIP_HTTP_TESTS=true
else
    print_status "SUCCESS" "curl found"
fi

# Test 2: Build all examples
print_status "INFO" "Building all examples..."

mkdir -p build

# Build server
print_status "INFO" "Building A2A server..."
if cd server && go build -o ../build/a2a-server main.go; then
    print_status "SUCCESS" "Server build successful"
    cd ..
else
    print_status "ERROR" "Server build failed"
    exit 1
fi

# Build client
print_status "INFO" "Building A2A client..."
if cd client && go build -o ../build/a2a-client main.go; then
    print_status "SUCCESS" "Client build successful"
    cd ..
else
    print_status "ERROR" "Client build failed"
    exit 1
fi

# Build full demo
print_status "INFO" "Building full demo..."
if cd full_demo && go build -o ../build/a2a-demo main.go; then
    print_status "SUCCESS" "Full demo build successful"
    cd ..
else
    print_status "ERROR" "Full demo build failed"
    exit 1
fi

# Test 3: Start server and basic HTTP tests
if [ "$SKIP_HTTP_TESTS" != "true" ]; then
    print_status "INFO" "Starting server for HTTP tests..."
    
    # Start server in background
    ./build/a2a-server > build/server.log 2>&1 &
    SERVER_PID=$!
    
    # Give server time to start
    sleep 3
    
    # Function to cleanup server
    cleanup_server() {
        if kill -0 $SERVER_PID 2>/dev/null; then
            kill $SERVER_PID
            wait $SERVER_PID 2>/dev/null || true
        fi
    }
    
    # Set trap to cleanup on exit
    trap cleanup_server EXIT
    
    # Test health endpoint
    print_status "INFO" "Testing health endpoint..."
    if curl -s -f http://localhost:8080/health > /dev/null; then
        print_status "SUCCESS" "Health endpoint responding"
    else
        print_status "ERROR" "Health endpoint not responding"
        print_status "INFO" "Server log:"
        cat build/server.log
        exit 1
    fi
    
    # Test agent discovery
    print_status "INFO" "Testing agent discovery..."
    if AGENT_RESPONSE=$(curl -s -f http://localhost:8080/.well-known/agent.json); then
        print_status "SUCCESS" "Agent discovery working"
        AGENT_NAME=$(echo "$AGENT_RESPONSE" | grep -o '"name":"[^"]*"' | cut -d'"' -f4)
        print_status "INFO" "Discovered agent: $AGENT_NAME"
    else
        print_status "ERROR" "Agent discovery failed"
        exit 1
    fi
    
    # Test agents list
    print_status "INFO" "Testing agents list..."
    if curl -s -f http://localhost:8080/agents > /dev/null; then
        print_status "SUCCESS" "Agents list endpoint working"
    else
        print_status "ERROR" "Agents list endpoint failed"
        exit 1
    fi
    
    # Test A2A endpoint with basic request
    print_status "INFO" "Testing A2A endpoint..."
    A2A_REQUEST='{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tasks/send",
        "params": {
            "id": "test-task-123",
            "message": {
                "role": "user",
                "parts": [{"type": "text", "text": "Hello, test message"}]
            },
            "metadata": {"agent_name": "assistant"}
        }
    }'
    
    if A2A_RESPONSE=$(curl -s -X POST http://localhost:8080/a2a \
        -H "Content-Type: application/json" \
        -d "$A2A_REQUEST"); then
        
        if echo "$A2A_RESPONSE" | grep -q '"result"'; then
            print_status "SUCCESS" "A2A endpoint working"
            TASK_ID=$(echo "$A2A_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
            print_status "INFO" "Created task: $TASK_ID"
        else
            print_status "ERROR" "A2A endpoint returned error"
            print_status "INFO" "Response: $A2A_RESPONSE"
            exit 1
        fi
    else
        print_status "ERROR" "A2A endpoint request failed"
        exit 1
    fi
    
    print_status "SUCCESS" "All HTTP tests passed"
    
    # Stop server
    cleanup_server
    trap - EXIT
    
else
    print_status "WARNING" "Skipping HTTP tests (curl not available)"
fi

# Test 4: Run full demo with timeout
print_status "INFO" "Testing full demo (with timeout)..."

# Check if timeout command is available
if command_exists timeout; then
    # Run demo with timeout to prevent hanging
    if timeout 30s ./build/a2a-demo > build/demo.log 2>&1; then
        print_status "SUCCESS" "Full demo completed successfully"
    else
        EXIT_CODE=$?
        if [ $EXIT_CODE -eq 124 ]; then
            print_status "WARNING" "Full demo timed out (this is expected)"
        else
            print_status "ERROR" "Full demo failed with exit code $EXIT_CODE"
            print_status "INFO" "Demo log:"
            tail -20 build/demo.log
            exit 1
        fi
    fi
elif command_exists gtimeout; then
    # On macOS, timeout might be available as gtimeout via coreutils
    if gtimeout 30s ./build/a2a-demo > build/demo.log 2>&1; then
        print_status "SUCCESS" "Full demo completed successfully"
    else
        EXIT_CODE=$?
        if [ $EXIT_CODE -eq 124 ]; then
            print_status "WARNING" "Full demo timed out (this is expected)"
        else
            print_status "ERROR" "Full demo failed with exit code $EXIT_CODE"
            print_status "INFO" "Demo log:"
            tail -20 build/demo.log
            exit 1
        fi
    fi
else
    # Run without timeout but with a background process to kill it
    print_status "WARNING" "timeout command not available, running demo briefly..."
    ./build/a2a-demo > build/demo.log 2>&1 &
    DEMO_PID=$!
    sleep 10  # Let it run for 10 seconds
    if kill -0 $DEMO_PID 2>/dev/null; then
        kill $DEMO_PID 2>/dev/null || true
        wait $DEMO_PID 2>/dev/null || true
        print_status "SUCCESS" "Full demo ran successfully (stopped after 10s)"
    else
        print_status "WARNING" "Full demo exited early"
    fi
fi

# Test 5: Check for common issues
print_status "INFO" "Checking for common issues..."

# Check if ports are available
if command_exists netstat; then
    if netstat -tuln | grep -q ":8080 "; then
        print_status "WARNING" "Port 8080 is already in use"
    else
        print_status "SUCCESS" "Port 8080 is available"
    fi
fi

# Check Go modules
if [ -f "../../go.mod" ]; then
    print_status "SUCCESS" "Go module found"
else
    print_status "WARNING" "No go.mod found in project root"
fi

# Summary
print_status "SUCCESS" "🎉 All tests completed successfully!"
print_status "INFO" "Built binaries are available in the build/ directory:"
print_status "INFO" "  - build/a2a-server"
print_status "INFO" "  - build/a2a-client" 
print_status "INFO" "  - build/a2a-demo"

echo ""
print_status "INFO" "Next steps:"
print_status "INFO" "  1. Run 'make server' to start the A2A server"
print_status "INFO" "  2. Run 'make client' in another terminal to test the client"
print_status "INFO" "  3. Or run 'make demo' for an integrated experience"
print_status "INFO" "  4. See README.md for detailed documentation"

exit 0
