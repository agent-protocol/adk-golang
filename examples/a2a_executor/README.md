# A2A Agent Executor

The A2A Agent Executor provides a bridge between the A2A (Agent-to-Agent) protocol and ADK agents, enabling seamless integration of ADK agents into A2A workflows.

## Overview

The A2A Agent Executor handles:

1. **A2A Request Processing**: Converts incoming A2A requests to ADK format
2. **Agent Execution**: Runs ADK agents with proper session management  
3. **Event Conversion**: Transforms ADK events back to A2A protocol messages
4. **Real-time Streaming**: Provides real-time event streaming for A2A clients

## Key Components

### A2aAgentExecutor

The main executor class that orchestrates the conversion and execution process:

```go
// Create with direct runner instance
executor := executor.NewA2aAgentExecutor(runner, config)

// Or create with runner factory for lazy initialization
executor := executor.NewA2aAgentExecutorWithFactory(runnerFactory, config)
```

### Event Converters

Located in `pkg/a2a/converters/`, these handle bidirectional conversion between A2A and ADK formats:

- **Request Converter**: Converts A2A requests to ADK run arguments
- **Event Converter**: Converts ADK events to A2A status updates and artifact events

### Configuration

```go
config := &executor.A2aAgentExecutorConfig{
    EnableDebugLogging:    true,
    Timeout:              30 * time.Second,
    MaxConcurrentRequests: 5,
}
```

## Usage Example

### Basic Usage

```go
// Create ADK agent and runner
agent := agents.NewLlmAgent("my-agent", config)
runner := runners.NewRunner("my-app", agent, sessionService)

// Create A2A executor
a2aExecutor := executor.NewA2aAgentExecutor(runner, nil)

// Create A2A request context
requestCtx := &executor.RequestContext{
    TaskID:    "task-123",
    ContextID: "context-456", 
    Message: &a2a.Message{
        Role: "user",
        Parts: []a2a.Part{
            {
                Type: "text",
                Text: stringPtr("Hello, agent!"),
            },
        },
    },
    UserID: "user-789",
}

// Create event queue for A2A events
eventQueue := executor.NewSimpleEventQueue(10)
defer eventQueue.Close()

// Execute the request
ctx := context.Background()
err := a2aExecutor.Execute(ctx, requestCtx, eventQueue)
if err != nil {
    log.Fatalf("Execution failed: %v", err)
}

// Process A2A events
for event := range eventQueue.Events() {
    switch e := event.(type) {
    case *a2a.TaskStatusUpdateEvent:
        fmt.Printf("Task status: %s\n", e.Status.State)
    case *a2a.TaskArtifactUpdateEvent:
        fmt.Printf("Artifact: %s\n", *e.Artifact.Name)
    }
}
```

### Integration with A2A Server

The executor can be integrated with an A2A server to expose ADK agents as A2A endpoints:

```go
// Create A2A server
server := a2a.NewA2AServer(a2a.A2AServerConfig{})

// Register agent with executor
server.RegisterAgentWithExecutor("my-agent", a2aExecutor, agentCard)

// Start HTTP server
http.ListenAndServe(":8080", server)
```

## Event Flow

1. **A2A Request** → **Request Converter** → **ADK Run Args**
2. **ADK Run Args** → **Runner.RunAsync()** → **ADK Event Stream**
3. **ADK Events** → **Event Converter** → **A2A Events**
4. **A2A Events** → **Event Queue** → **A2A Client**

## Supported A2A Features

### Message Types
- ✅ Text messages
- ✅ Function calls (via data parts)
- ✅ Function responses
- ⚠️ File attachments (basic support)

### Task States
- ✅ `submitted` - Task has been received
- ✅ `working` - Agent is processing
- ✅ `input-required` - Long-running tool needs input
- ✅ `completed` - Task finished successfully
- ✅ `failed` - Task encountered an error
- ✅ `canceled` - Task was canceled

### Event Types
- ✅ `TaskStatusUpdateEvent` - Status changes and messages
- ✅ `TaskArtifactUpdateEvent` - Artifact updates
- ✅ Real-time streaming via event queue

## Error Handling

The executor provides comprehensive error handling:

- **Conversion Errors**: Detailed errors when A2A/ADK conversion fails
- **Execution Errors**: Proper error events when agent execution fails  
- **Timeout Handling**: Configurable timeouts for long-running operations
- **Graceful Degradation**: Continues processing when non-critical errors occur

## Testing

Run the tests:

```bash
go test ./pkg/a2a/executor/...
go test ./pkg/a2a/converters/...
```

Run the example:

```bash
go run examples/a2a_executor/main.go
```

## Performance Considerations

- **Event Buffering**: Configurable buffer sizes for event queues
- **Concurrent Requests**: Limits on concurrent A2A request processing
- **Memory Management**: Proper cleanup of goroutines and channels
- **Streaming**: Non-blocking event streaming for real-time updates

## Future Enhancements

- [ ] Enhanced file handling with proper content management
- [ ] Websocket support for bi-directional communication  
- [ ] Authentication and authorization integration
- [ ] Metrics and observability features
- [ ] Advanced session management with persistence
- [ ] Support for A2A push notifications

## Contributing

When contributing to the A2A executor:

1. Ensure all conversions preserve semantic meaning
2. Add comprehensive tests for new features
3. Update documentation for any API changes
4. Follow ADK and A2A protocol specifications
5. Test with real A2A clients when possible
