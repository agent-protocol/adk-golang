# ADK HTTP API Server & Web UI

This package provides a complete HTTP API server implementation equivalent to Python's `fast_api.py`, with additional Web UI support for interactive agent testing.

## Features

### 🚀 HTTP API Endpoints

- **POST `/run`** - Execute agents synchronously
- **POST `/run_sse`** - Execute agents with Server-Sent Events streaming  
- **WebSocket `/run_live`** - Live bidirectional agent interactions
- **POST `/a2a`** - A2A (Agent-to-Agent) protocol endpoint
- **GET `/list-apps`** - List available agents
- **GET `/health`** - Health check endpoint

### 🌐 Web UI

- **Interactive chat interface** - Test agents directly in your browser
- **Real-time streaming** - See agent responses as they arrive
- **Agent selection** - Switch between different agents
- **Session management** - Automatic session creation and management

### 📊 Session Management API

- **GET `/apps/{app}/users/{user}/sessions`** - List user sessions
- **POST `/apps/{app}/users/{user}/sessions`** - Create new session
- **GET `/apps/{app}/users/{user}/sessions/{id}`** - Get specific session
- **DELETE `/apps/{app}/users/{user}/sessions/{id}`** - Delete session

## Quick Start

### 1. Start the Web UI Server

```bash
# Start with web UI (includes all API endpoints)
./bin/adk web --port 8000 examples/test-agents

# Visit http://localhost:8000 in your browser
```

### 2. Start API-Only Server

```bash
# Start API server without web UI
./bin/adk api-server --port 8000 examples/test-agents
```

### 3. Configuration Options

```bash
./bin/adk web \
  --host 0.0.0.0 \
  --port 8080 \
  --allow-origins "http://localhost:3000,https://myapp.com" \
  --a2a \
  --session-service-uri "sqlite://sessions.db" \
  examples/test-agents
```

## API Usage Examples

### Execute Agent Synchronously

```bash
curl -X POST http://localhost:8000/run \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "echo-agent",
    "user_id": "test-user",
    "session_id": "test-session",
    "new_message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Hello, agent!"}]
    }
  }'
```

### Execute Agent with Streaming

```bash
curl -X POST http://localhost:8000/run_sse \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "echo-agent",
    "user_id": "test-user", 
    "session_id": "test-session",
    "new_message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Hello, streaming agent!"}]
    },
    "streaming": true
  }'
```

### WebSocket Connection

```javascript
const ws = new WebSocket('ws://localhost:8000/run_live?app_name=echo-agent&user_id=test-user&session_id=test-session');

ws.onopen = () => {
  ws.send(JSON.stringify({
    text: "Hello via WebSocket!"
  }));
};

ws.onmessage = (event) => {
  const response = JSON.parse(event.data);
  console.log('Agent response:', response);
};
```

### A2A Protocol

```bash
curl -X POST http://localhost:8000/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "echo-agent",
    "message": {
      "parts": [{"text": "Hello from A2A!"}]
    }
  }'
```

## Web UI Features

### Agent Selection
- Automatically discovers available agents from the agents directory
- Click any agent to start a new chat session
- Visual indication of the currently selected agent

### Real-time Chat
- Send messages by typing and pressing Enter or clicking Send
- See agent responses stream in real-time via Server-Sent Events
- Messages are visually distinguished between user and agent

### Session Management
- Automatic session creation when selecting an agent
- Session state persists during the chat
- Clean session management with proper cleanup

## Architecture

### Server Components

- **`Server`** - Main HTTP server with middleware and routing
- **`WebUIHandler`** - Serves the interactive web interface
- **Service Integration** - Session, Artifact, and Memory services
- **Agent Loader** - Dynamic agent discovery and loading
- **Runner Cache** - Efficient agent execution with caching

### Supported Agent Formats

- **Go Source** - `main.go` with `RootAgent` variable
- **YAML Config** - `agent.yml` declarative configuration  
- **Executable** - Compiled agent binaries
- **Go Plugins** - `agent.so` shared libraries

### CORS & Security

- Configurable CORS origins
- WebSocket origin validation
- Request validation and sanitization
- Error handling with proper HTTP status codes

## Configuration

### Service URIs

```bash
# Session storage options
--session-service-uri "sqlite://path/to/sessions.db"
--session-service-uri "agentengine://resource_id"

# Artifact storage options  
--artifact-service-uri "gs://bucket-name"

# Memory service options
--memory-service-uri "rag://corpus_id"
--memory-service-uri "agentengine://resource_id"
```

### Server Options

```bash
--host 127.0.0.1           # Bind address
--port 8000                # Port number
--allow-origins "url1,url2" # CORS origins
--a2a                      # Enable A2A endpoint
--trace-to-cloud           # Enable cloud tracing
--log-level debug          # Logging level
```

## Development

### Adding New Endpoints

```go
// In server.go setupRoutes()
s.router.HandleFunc("/my-endpoint", s.handleMyEndpoint)

// Implement handler
func (s *Server) handleMyEndpoint(w http.ResponseWriter, r *http.Request) {
    // Handle request
}
```

### Extending WebSocket Support

```go
// In webui.go
func (s *Server) handleWebSocketSession(conn *websocket.Conn, runner *runners.RunnerImpl, session *core.Session) {
    // Add custom WebSocket message handling
}
```

### Custom Service Integration

```go
// In server.go
func createCustomService(uri string) (CustomService, error) {
    // Implement custom service creation
}
```

## Error Handling

The server provides comprehensive error handling:

- **400 Bad Request** - Invalid request format or parameters
- **404 Not Found** - Session, agent, or resource not found  
- **405 Method Not Allowed** - Unsupported HTTP method
- **500 Internal Server Error** - Server-side execution errors

Error responses include descriptive messages to aid debugging.

## Monitoring & Debugging

### Health Checks

```bash
curl http://localhost:8000/health
# Returns: {"status": "healthy", "time": "2024-01-01T12:00:00Z"}
```

### Agent Discovery

```bash
curl http://localhost:8000/list-apps
# Returns: ["echo-agent", "chat-agent", "search-agent"]
```

### Session Inspection

```bash
# List sessions
curl http://localhost:8000/apps/echo-agent/users/test-user/sessions

# Get specific session  
curl http://localhost:8000/apps/echo-agent/users/test-user/sessions/session-123
```

This implementation provides a complete, production-ready HTTP API server for the ADK framework with an intuitive web interface for testing and development.
