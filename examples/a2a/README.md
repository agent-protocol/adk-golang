# A2A (Agent-to-Agent) Communication Examples

This directory contains comprehensive examples demonstrating how to use the A2A (Agent-to-Agent) communication protocol in the Go ADK. The A2A protocol enables agents to communicate with each other remotely using JSON-RPC over HTTP.

## Overview

The A2A protocol allows:
- **Remote Agent Communication**: Agents can invoke other agents running on different servers
- **Standardized Interface**: JSON-RPC based protocol for consistent communication
- **Agent Discovery**: Well-known endpoints for discovering agent capabilities
- **Task Management**: Complete lifecycle management of agent tasks
- **Streaming Support**: Real-time event streaming for long-running tasks

## Examples Structure

```
examples/a2a/
├── README.md              # This file
├── server/                # A2A Server examples
│   └── main.go           # Complete server implementation
├── client/               # A2A Client examples  
│   └── main.go           # Comprehensive client usage
└── full_demo/            # Complete end-to-end demo
    └── main.go           # Server + Client integrated demo
```

## Quick Start

### 1. Server Example

The server example shows how to expose local agents as A2A services:

```bash
cd examples/a2a/server
go run main.go
```

This starts an A2A server on `http://localhost:8080` with:
- **Multiple Agents**: Calculator, Weather, Greeter, and Multi-tool agents
- **Well-known Discovery**: `/.well-known/agent.json` endpoint
- **Health Checks**: `/health` endpoint for monitoring
- **Agent Listing**: `/agents` endpoint for available agents

**Key Features Demonstrated:**
- Agent registration and management
- Agent cards (metadata) creation
- HTTP endpoint setup
- JSON-RPC request handling
- Tool integration with agents

### 2. Client Example

The client example demonstrates how to communicate with remote agents:

```bash
# Make sure the server is running first
cd examples/a2a/client
go run main.go
```

**Key Features Demonstrated:**
- Agent discovery using well-known endpoints
- Basic message sending
- Streaming communication
- Task lifecycle management
- Error handling and timeouts
- Polling for task completion

### 3. Full Integration Demo

The full demo shows both server and client working together:

```bash
cd examples/a2a/full_demo
go run main.go
```

This comprehensive demo includes:
- Automatic server startup
- Multiple client interaction patterns
- Concurrent request handling
- Complex multi-step tasks
- Error scenario testing
- Graceful shutdown

## Core Concepts

### Agent Cards

Agent cards provide metadata about agent capabilities:

```go
agentCard := &a2a.AgentCard{
    Name:        "calculator",
    Description: stringPtr("Mathematical calculation specialist"),
    URL:         "http://localhost:8080/a2a",
    Version:     "1.0.0",
    Capabilities: a2a.AgentCapabilities{
        Streaming:              true,
        PushNotifications:      false,
        StateTransitionHistory: true,
    },
    Skills: []a2a.AgentSkill{
        {
            ID:          "calculation",
            Name:        "Calculator",
            Description: stringPtr("Perform mathematical calculations"),
            Examples:    []string{"Calculate 2+2", "What is 15*7?"},
        },
    },
}
```

### Client Usage

Basic client creation and usage:

```go
// Create client from agent card
client, err := a2a.NewClient(agentCard, nil)
if err != nil {
    return err
}
defer client.Close()

// Send a message
message := &a2a.Message{
    Role: "user",
    Parts: []a2a.Part{{
        Type: "text",
        Text: stringPtr("Hello, please calculate 15 * 23"),
    }},
}

params := &a2a.TaskSendParams{
    ID:      generateTaskID(),
    Message: *message,
    Metadata: map[string]any{
        "agent_name": "calculator",
    },
}

task, err := client.SendMessage(ctx, params)
```

### Server Setup

Setting up an A2A server:

```go
// Create agents and tools
calculatorTool := createCalculatorTool()
calcAgent := agents.NewLLMAgent("calculator", "Math specialist", "gemini-2.0-flash")
calcAgent.AddTool(calculatorTool)

// Create server
agentMap := map[string]core.BaseAgent{
    "calculator": calcAgent,
}

agentCards := map[string]*a2a.AgentCard{
    "calculator": createCalculatorCard(),
}

a2aServer := server.NewA2AServer(server.A2AServerConfig{
    Agents:     agentMap,
    AgentCards: agentCards,
})

// Set up HTTP routes
mux := http.NewServeMux()
mux.Handle("/a2a", a2aServer)
mux.HandleFunc("/.well-known/agent.json", wellKnownHandler)

// Start server
srv := &http.Server{Addr: ":8080", Handler: mux}
srv.ListenAndServe()
```

## Communication Patterns

### 1. Basic Request-Response

Simple one-time message sending:

```go
task, err := client.SendMessage(ctx, params)
// Poll for completion or handle response
```

### 2. Streaming Communication

Real-time event streaming:

```go
err := client.SendMessageStream(ctx, params, func(response *a2a.SendTaskStreamingResponse) error {
    // Handle streaming events
    if response.Error != nil {
        return response.Error
    }
    
    // Process status updates, artifacts, etc.
    return nil
})
```

### 3. Task Management

Complete task lifecycle:

```go
// Create task
task, err := client.SendMessage(ctx, params)

// Query status
status, err := client.GetTask(ctx, &a2a.TaskQueryParams{ID: task.ID})

// Cancel if needed
canceled, err := client.CancelTask(ctx, &a2a.TaskIdParams{ID: task.ID})
```

## Agent Discovery

### Well-known Endpoint

Agents can be discovered using standard endpoints:

```go
resolver := a2a.NewAgentCardResolver("http://localhost:8080", nil)
agentCard, err := resolver.GetWellKnownAgentCard(ctx)
```

### Custom Discovery

You can also implement custom discovery mechanisms:

```go
agentCard, err := resolver.GetAgentCard(ctx, "/custom/agent/path")
```

## Error Handling

The examples demonstrate various error scenarios:

- **Network errors**: Timeouts, connection failures
- **Protocol errors**: Invalid JSON-RPC requests
- **Agent errors**: Task not found, cancellation failures
- **Validation errors**: Invalid parameters, malformed messages

```go
// Configure client with timeout
config := &a2a.ClientConfig{
    Timeout: 30 * time.Second,
    Headers: map[string]string{
        "User-Agent": "A2A-Client/1.0",
    },
}

client, err := a2a.NewClient(agentCard, config)
```

## Advanced Features

### Push Notifications

Set up push notifications for task updates:

```go
pushConfig := &a2a.TaskPushNotificationConfig{
    ID: taskID,
    PushNotificationConfig: a2a.PushNotificationConfig{
        URL:   "https://my-app.com/webhooks/a2a",
        Token: stringPtr("webhook-secret"),
    },
}

result, err := client.SetTaskPushNotification(ctx, pushConfig)
```

### Custom Headers

Add custom headers for authentication or routing:

```go
config := &a2a.ClientConfig{
    Headers: map[string]string{
        "Authorization": "Bearer " + token,
        "X-Request-ID":  requestID,
    },
}
```

### Concurrent Operations

Handle multiple concurrent requests:

```go
var wg sync.WaitGroup
results := make(chan *a2a.Task, numRequests)

for i := 0; i < numRequests; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        task, err := client.SendMessage(ctx, createParams(id))
        if err != nil {
            log.Printf("Request %d failed: %v", id, err)
            return
        }
        results <- task
    }(i)
}

wg.Wait()
close(results)
```

## Testing and Development

### Running the Examples

1. **Start the server**:
   ```bash
   cd examples/a2a/server
   go run main.go
   ```

2. **Test with client** (in another terminal):
   ```bash
   cd examples/a2a/client  
   go run main.go
   ```

3. **Run full demo** (integrated):
   ```bash
   cd examples/a2a/full_demo
   go run main.go
   ```

### Manual Testing

You can also test the A2A server manually using curl:

```bash
# Discover agent
curl http://localhost:8080/.well-known/agent.json

# Send a task
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tasks/send",
    "params": {
      "id": "test-task-123",
      "message": {
        "role": "user",
        "parts": [{"type": "text", "text": "Calculate 2+2"}]
      },
      "metadata": {"agent_name": "calculator"}
    }
  }'

# Query task status
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0", 
    "id": 2,
    "method": "tasks/get",
    "params": {"id": "test-task-123"}
  }'
```

## Best Practices

1. **Error Handling**: Always handle errors gracefully and provide meaningful messages
2. **Timeouts**: Set appropriate timeouts for your use case
3. **Resource Cleanup**: Always close clients when done
4. **Agent Discovery**: Cache agent cards to avoid repeated discovery calls
5. **Task IDs**: Use unique, meaningful task IDs for tracking
6. **Metadata**: Use task metadata for routing and debugging
7. **Monitoring**: Implement health checks and logging
8. **Security**: Use authentication headers in production

## Troubleshooting

### Common Issues

1. **Connection Refused**: Make sure the server is running on the correct port
2. **Task Not Found**: Check that task IDs are correct and tasks haven't expired
3. **Timeout Errors**: Adjust client timeout settings or check server performance
4. **JSON Parse Errors**: Verify message format and encoding
5. **Agent Not Found**: Check agent registration and metadata configuration

### Debug Tips

- Enable verbose logging in both client and server
- Use the `/health` endpoint to verify server status
- Check the `/agents` endpoint to see available agents
- Monitor HTTP response codes and JSON-RPC error codes
- Use network tools to inspect HTTP traffic

## Integration with Other Systems

The A2A protocol can be integrated with:

- **Web Applications**: Expose agents as REST APIs
- **Message Queues**: Use A2A for async agent communication  
- **Microservices**: Agent communication across service boundaries
- **Load Balancers**: Distribute agent requests across instances
- **Monitoring Systems**: Track agent performance and availability

This completes the comprehensive A2A examples. These examples provide a solid foundation for building agent-to-agent communication systems using the Go ADK.
