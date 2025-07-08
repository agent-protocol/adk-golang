# Enhanced Runner Implementation

## Overview

The Runner is the orchestration engine for ADK (Agent Development Kit) that manages agent execution, session state, and provides real-time event streaming. It's designed to be similar to Python's `AsyncGenerator` pattern using Go channels.

## Key Features

### 🔄 **Async Event Streaming**
- Real-time event streaming using Go channels
- Configurable buffer sizes for optimal performance
- Non-blocking event processing
- Cancellation support via `context.Context`

### 📊 **Session Management**  
- Automatic session creation and retrieval
- State persistence across agent interactions
- Event history tracking
- Support for multiple concurrent sessions

### ⚙️ **Event Processing**
- Automatic state delta application
- Artifact version tracking
- Memory service integration
- Agent lifecycle callbacks

### 🚀 **Concurrent Execution**
- Thread-safe operations with proper mutex usage
- Support for multiple concurrent agent executions
- Configurable concurrency limits
- Graceful error handling and recovery

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   RunRequest    │───▶│     Runner      │───▶│  EventStream    │
│                 │    │                 │    │  (channel)      │
│ • UserID        │    │ • Session Mgmt  │    │                 │
│ • SessionID     │    │ • Agent Exec    │    │ ┌─────────────┐ │
│ • NewMessage    │    │ • Event Proc    │    │ │   Event 1   │ │
│ • RunConfig     │    │ • State Mgmt    │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘    │ ┌─────────────┐ │
                                              │ │   Event 2   │ │
                                              │ └─────────────┘ │
                                              │       ...       │
                                              └─────────────────┘
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/agent-protocol/adk-golang/pkg/runners"
    "github.com/agent-protocol/adk-golang/pkg/sessions"
)

func main() {
    // Create session service
    sessionService := sessions.NewInMemorySessionService()
    
    // Create your agent (implement core.BaseAgent)
    agent := &MyAgent{name: "my-agent"}
    
    // Create runner
    runner := runners.NewRunner("my-app", agent, sessionService)
    
    // Create run request
    req := &core.RunRequest{
        UserID:    "user123",
        SessionID: "session456", 
        NewMessage: &core.Content{
            Role: "user",
            Parts: []core.Part{
                {Type: "text", Text: stringPtr("Hello!")},
            },
        },
    }
    
    // Execute asynchronously
    ctx := context.Background()
    eventStream, err := runner.RunAsync(ctx, req)
    if err != nil {
        panic(err)
    }
    
    // Process events in real-time
    for event := range eventStream {
        fmt.Printf("Event from %s: %v\n", event.Author, event.Content)
    }
}
```

### Advanced Configuration

```go
// Custom runner configuration
config := &runners.RunnerConfig{
    EventBufferSize:          100,        // Buffer size for event channel
    EnableEventProcessing:    true,       // Enable automatic event processing
    MaxConcurrentSessions:    50,         // Limit concurrent sessions (0 = unlimited)
    DefaultTimeout:          30 * time.Second,
}

runner := runners.NewRunnerWithConfig("my-app", agent, sessionService, config)

// Add optional services
runner.SetArtifactService(artifactService)
runner.SetMemoryService(memoryService)
runner.SetCredentialService(credentialService)
```

### Event Processing

The Runner automatically processes event actions:

```go
// Events can contain actions that modify session state
event := &core.Event{
    Author: "my-agent",
    Actions: core.EventActions{
        StateDelta: map[string]any{
            "counter": 42,
            "status":  "completed",
        },
        ArtifactDelta: map[string]int{
            "result.json": 1,  // Version 1
        },
        TransferToAgent: stringPtr("other-agent"),
    },
}
```

### Synchronous Execution

For cases where you need all events at once:

```go
events, err := runner.Run(ctx, req)
if err != nil {
    panic(err)
}

fmt.Printf("Received %d events\n", len(events))
for _, event := range events {
    // Process events...
}
```

## Event Streaming Pattern

The Runner implements async event streaming similar to Python's `AsyncGenerator`:

**Python (ADK-Python):**
```python
async def run_async(user_id: str, session_id: str, message: str):
    async for event in runner.run_async(
        user_id=user_id, 
        session_id=session_id, 
        new_message=message
    ):
        print(f"Event: {event.content}")
        yield event
```

**Go (ADK-Golang):**
```go
func RunAsync(userID, sessionID, message string) <-chan *core.Event {
    req := &core.RunRequest{
        UserID: userID,
        SessionID: sessionID,
        NewMessage: createMessage(message),
    }
    
    eventStream, err := runner.RunAsync(ctx, req)
    if err != nil {
        return nil
    }
    
    return eventStream
}

// Usage
for event := range RunAsync("user123", "session456", "Hello!") {
    fmt.Printf("Event: %v\n", event.Content)
}
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `EventBufferSize` | `int` | `100` | Size of the event channel buffer |
| `EnableEventProcessing` | `bool` | `true` | Enable automatic processing of event actions |
| `MaxConcurrentSessions` | `int` | `0` | Maximum concurrent sessions (0 = unlimited) |
| `DefaultTimeout` | `time.Duration` | `30s` | Default timeout for operations |

## Error Handling

The Runner provides comprehensive error handling:

```go
eventStream, err := runner.RunAsync(ctx, req)
if err != nil {
    // Handle immediate errors (session creation, validation, etc.)
    log.Printf("Runner failed to start: %v", err)
    return
}

for event := range eventStream {
    if event.ErrorMessage != nil {
        // Handle runtime errors from agent execution
        log.Printf("Agent error: %s", *event.ErrorMessage)
    }
    
    // Process successful events...
}
```

## Agent Lifecycle Callbacks

The Runner supports agent lifecycle callbacks:

```go
type MyAgent struct {
    // ... agent implementation
}

func (a *MyAgent) SetBeforeAgentCallback(callback core.BeforeAgentCallback) {
    a.beforeCallback = callback
}

func (a *MyAgent) SetAfterAgentCallback(callback core.AfterAgentCallback) {
    a.afterCallback = callback
}

// Usage
agent.SetBeforeAgentCallback(func(ctx context.Context, invocationCtx *core.InvocationContext) error {
    log.Printf("Starting agent execution for session %s", invocationCtx.Session.ID)
    return nil
})

agent.SetAfterAgentCallback(func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
    log.Printf("Agent execution completed with %d events", len(events))
    return nil
})
```

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./pkg/runners/...

# Run tests with coverage
go test -cover ./pkg/runners/...

# Run specific test
go test -run TestRunnerBasicExecution ./pkg/runners/...
```

## Examples

See the `examples/enhanced_runner/` directory for a complete demonstration of:

- Real-time event streaming
- State management
- Concurrent execution
- Error handling
- Session persistence

Run the example:

```bash
cd examples/enhanced_runner
go run main.go
```

## Performance Considerations

### Memory Usage
- Event buffer size affects memory usage
- Large sessions may consume significant memory
- Consider implementing session cleanup policies

### Concurrency
- Use `MaxConcurrentSessions` to limit resource usage
- Monitor goroutine usage in high-load scenarios
- Consider connection pooling for external services

### Event Processing
- Disable `EnableEventProcessing` if you handle events manually
- Large state deltas may impact performance
- Consider batch processing for high-frequency events

## Integration with ADK Components

### Session Services
- In-memory sessions for development
- File-based sessions for persistence
- Database sessions for production

### Artifact Services
- Local file storage
- Cloud storage (GCS, S3)
- In-memory for testing

### Memory Services
- Simple in-memory storage
- Vector databases for semantic search
- External knowledge bases

## Comparison with Python ADK

| Feature | Python ADK | Go ADK |
|---------|------------|--------|
| Event Streaming | `AsyncGenerator` | Go channels |
| Concurrency | `asyncio` | Goroutines |
| Error Handling | Exceptions | Explicit errors |
| Type Safety | Runtime (Pydantic) | Compile-time |
| Performance | Good | Excellent |
| Memory Usage | Higher | Lower |

## Best Practices

1. **Always use context.Context for cancellation**
2. **Configure appropriate buffer sizes for your use case**
3. **Handle errors at both creation and runtime**
4. **Use structured logging for debugging**
5. **Monitor session and memory usage in production**
6. **Implement proper cleanup in agent implementations**
7. **Test concurrent execution scenarios**
8. **Use synchronous execution for batch processing**

## Contributing

When contributing to the Runner implementation:

1. Maintain thread safety
2. Add comprehensive tests
3. Update documentation
4. Follow Go idioms and best practices
5. Ensure compatibility with existing ADK agents

## License

This implementation is part of the ADK (Agent Development Kit) project and follows the same license terms.
