# A2A API Quick Reference

## Server Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/a2a` | POST | Main A2A JSON-RPC endpoint |
| `/.well-known/agent.json` | GET | Agent discovery |
| `/health` | GET | Health check |
| `/agents` | GET | List available agents |

## JSON-RPC Methods

### tasks/send
Send a message to start a new task:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tasks/send",
  "params": {
    "id": "task-123",
    "message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Hello"}]
    },
    "metadata": {"agent_name": "calculator"}
  }
}
```

### tasks/get
Query task status:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tasks/get",
  "params": {
    "id": "task-123"
  }
}
```

### tasks/cancel
Cancel a running task:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tasks/cancel",
  "params": {
    "id": "task-123"
  }
}
```

### tasks/sendSubscribe
Start streaming task (SSE):
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tasks/sendSubscribe",
  "params": {
    "id": "task-123",
    "message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Stream me results"}]
    }
  }
}
```

## Response Format

Success response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "id": "task-123",
    "status": {
      "state": "working",
      "message": null
    }
  }
}
```

Error response:
```json
{
  "jsonrpc": "2.0", 
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Internal error",
    "data": "Task not found"
  }
}
```

## Task States

- `submitted` - Task received and queued
- `working` - Task is being processed
- `input-required` - Task needs user input
- `completed` - Task finished successfully
- `canceled` - Task was canceled
- `failed` - Task failed with error
- `unknown` - Unknown state

## Client Usage

### Basic Setup
```go
// Discover agent
resolver := a2a.NewAgentCardResolver("http://localhost:8080", nil)
agentCard, err := resolver.GetWellKnownAgentCard(ctx)

// Create client
client, err := a2a.NewClient(agentCard, nil)
defer client.Close()

// Send message
task, err := client.SendMessage(ctx, params)
```

### Streaming
```go
err := client.SendMessageStream(ctx, params, func(response *a2a.SendTaskStreamingResponse) error {
    // Handle streaming events
    return nil
})
```

### Configuration
```go
config := &a2a.ClientConfig{
    Timeout: 30 * time.Second,
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
}
client, err := a2a.NewClient(agentCard, config)
```

## curl Examples

### Discover Agent
```bash
curl http://localhost:8080/.well-known/agent.json
```

### Send Task
```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tasks/send",
    "params": {
      "id": "test-123",
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "Calculate 2+2"}]
      },
      "metadata": {"agent_name": "calculator"}
    }
  }'
```

### Query Task
```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tasks/get",
    "params": {"id": "test-123"}
  }'
```
