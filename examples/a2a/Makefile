# Makefile for A2A Examples

.PHONY: help server client demo clean test

# Default target
help:
	@echo "A2A Examples - Available commands:"
	@echo ""
	@echo "  make server    - Start the A2A server (port 8080)"
	@echo "  make client    - Run the A2A client demo (requires server to be running)"
	@echo "  make demo      - Run the complete integrated demo"
	@echo "  make test      - Test all examples"
	@echo "  make clean     - Clean build artifacts"
	@echo ""
	@echo "Quick start:"
	@echo "  1. Run 'make server' in one terminal"
	@echo "  2. Run 'make client' in another terminal"
	@echo "  Or run 'make demo' for an integrated experience"

# Start the A2A server
server:
	@echo "🚀 Starting A2A Server on http://localhost:8080..."
	@echo "Press Ctrl+C to stop"
	cd server && go run main.go

# Run the client demo (requires server to be running)
client:
	@echo "🤖 Running A2A Client Demo..."
	@echo "Make sure the server is running first (make server)"
	cd client && go run main.go

# Run the complete integrated demo
demo:
	@echo "🌟 Running Complete A2A Demo..."
	cd full_demo && go run main.go

# Test all examples (build only)
test:
	@echo "🧪 Testing A2A Examples..."
	@echo "Building server..."
	cd server && go build -o ../build/a2a-server main.go
	@echo "Building client..."
	cd client && go build -o ../build/a2a-client main.go
	@echo "Building full demo..."
	cd full_demo && go build -o ../build/a2a-demo main.go
	@echo "✅ All examples build successfully!"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf build/
	@echo "✅ Cleaned!"

# Create build directory
build/:
	mkdir -p build

# Advanced targets for development

# Run server in background
server-bg: build/
	@echo "🚀 Starting A2A Server in background..."
	cd server && go build -o ../build/a2a-server main.go
	./build/a2a-server &
	@echo "Server PID: $$!"
	@echo "Use 'pkill a2a-server' to stop"

# Quick test - start server, run client, stop server
quick-test: build/
	@echo "⚡ Running quick integration test..."
	cd server && go build -o ../build/a2a-server main.go && \
	cd ../client && go build -o ../build/a2a-client main.go && \
	echo "Starting server..." && \
	../build/a2a-server > ../build/server.log 2>&1 & \
	SERVER_PID=$$! && \
	sleep 3 && \
	echo "Running client..." && \
	../build/a2a-client && \
	echo "Stopping server..." && \
	kill $$SERVER_PID || true && \
	echo "✅ Quick test completed!"

# Development mode - auto-restart server on changes
dev-server:
	@echo "🔄 Starting development server (auto-restart)..."
	@echo "Install 'air' for auto-reload: go install github.com/cosmtrek/air@latest"
	cd server && air -c ../.air.toml || echo "Install air with: go install github.com/cosmtrek/air@latest"

# Show example endpoints
endpoints:
	@echo "📡 A2A Server Endpoints:"
	@echo ""
	@echo "  Main A2A endpoint:     http://localhost:8080/a2a"
	@echo "  Agent discovery:       http://localhost:8080/.well-known/agent.json"
	@echo "  Health check:          http://localhost:8080/health"
	@echo "  Available agents:      http://localhost:8080/agents"
	@echo ""
	@echo "Example curl commands:"
	@echo ""
	@echo "  # Discover agent"
	@echo "  curl http://localhost:8080/.well-known/agent.json"
	@echo ""
	@echo "  # Send task"
	@echo "  curl -X POST http://localhost:8080/a2a \\"
	@echo "    -H 'Content-Type: application/json' \\"
	@echo "    -d '{"
	@echo "      \"jsonrpc\": \"2.0\","
	@echo "      \"id\": 1,"
	@echo "      \"method\": \"tasks/send\","
	@echo "      \"params\": {"
	@echo "        \"id\": \"test-123\","
	@echo "        \"message\": {"
	@echo "          \"role\": \"user\","
	@echo "          \"parts\": [{\"type\": \"text\", \"text\": \"Calculate 2+2\"}]"
	@echo "        },"
	@echo "        \"metadata\": {\"agent_name\": \"calculator\"}"
	@echo "      }"
	@echo "    }'"

# Check if dependencies are available
deps:
	@echo "🔍 Checking dependencies..."
	@go version || echo "❌ Go not installed"
	@curl --version > /dev/null 2>&1 && echo "✅ curl available" || echo "⚠️  curl not available (optional)"
	@air -v > /dev/null 2>&1 && echo "✅ air available" || echo "⚠️  air not available (optional, for dev-server)"

# Install optional development tools
install-dev-tools:
	@echo "🛠️  Installing development tools..."
	go install github.com/cosmtrek/air@latest
	@echo "✅ Development tools installed!"
