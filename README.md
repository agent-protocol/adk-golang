# ADK for Go

Agent Development Kit (ADK) is a framework for building AI agents, and we're implementing ADK with Go. This project provides Go interfaces and implementations that mirror the Python ADK's architecture while following Go idioms and best practices.

## Overview

ADK consists of two main parts:
- **Agent2Agent (A2A) protocol**: Inter-agent communication protocol. See the [specification](https://a2aproject.github.io/A2A/latest/specification/)
- **ADK implementation**: The core framework for building agents. See [ADK documentation](https://google.github.io/adk-docs/)
  - [Python API](https://google.github.io/adk-docs/api-reference/python/)
  - [Java API](https://google.github.io/adk-docs/api-reference/java/)
  - **Go API** (this project)

## Core Architecture

The Go implementation follows the same architectural principles as the Python ADK but adapts them for Go's strengths:

### Core Components

- **Agents**: `BaseAgent`, `LlmAgent`, `SequentialAgent`, `RemoteA2aAgent`
- **Tools**: `BaseTool`, `FunctionTool`, `AgentTool`
- **Runner**: Orchestrates agent execution and manages workflows
- **Sessions**: Session management and state persistence
- **Events**: Communication units between agents with streaming support
- **A2A Integration**: A2A protocol implementation for remote agents

### Key Design Considerations for Go

- **Concurrency**: Use goroutines and channels instead of Python's asyncio
- **Error Handling**: Explicit error returns instead of exceptions
- **Context**: Use `context.Context` for cancellation and timeouts
- **Interfaces**: Define small, focused interfaces following Go idioms
- **Streaming**: Event streams using Go channels
- **JSON**: Proper struct tags for JSON marshaling/unmarshaling
- **Modularity**: Native Go approach for multi-agent projects

## Project Structure

```
adk-golang/
├── cmd/
│   └── adk/              # CLI application
├── pkg/
│   ├── agents/           # Agent implementations
│   ├── tools/            # Tool system
│   ├── sessions/         # Session management
│   ├── runners/          # Execution orchestration
│   └── a2a/              # A2A protocol implementation
├── internal/
│   ├── core/             # Core types and interfaces
│   ├── llm/              # LLM integrations
│   └── utils/            # Utilities
├── examples/             # Example agents and usage
└── tests/                # Test suites
```

## Core Interfaces

### BaseAgent

```go
type BaseAgent interface {
    Name() string
    Description() string
    Instruction() string
    
    SubAgents() []BaseAgent
    ParentAgent() BaseAgent
    SetParentAgent(parent BaseAgent)
    
    RunAsync(ctx context.Context, invocationCtx *InvocationContext) (EventStream, error)
    
    FindAgent(name string) BaseAgent
    FindSubAgent(name string) BaseAgent
    Cleanup(ctx context.Context) error
}
```

### BaseTool

```go
type BaseTool interface {
    Name() string
    Description() string
    IsLongRunning() bool
    
    GetDeclaration() *FunctionDeclaration
    RunAsync(ctx context.Context, args map[string]any, toolCtx *ToolContext) (any, error)
    ProcessLLMRequest(ctx context.Context, toolCtx *ToolContext, request *LLMRequest) error
}
```

### Event System

```go
type Event struct {
    ID                  string        `json:"id"`
    InvocationID        string        `json:"invocation_id"`
    Author              string        `json:"author"`
    Content             *Content      `json:"content,omitempty"`
    Actions             EventActions  `json:"actions"`
    Branch              *string       `json:"branch,omitempty"`
    Timestamp           time.Time     `json:"timestamp"`
    LongRunningToolIDs  []string      `json:"long_running_tool_ids,omitempty"`
    // ... additional fields
}

type EventStream <-chan *Event
```

### Session Management

```go
type SessionService interface {
    CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error)
    GetSession(ctx context.Context, req *GetSessionRequest) (*Session, error)
    AppendEvent(ctx context.Context, session *Session, event *Event) error
    DeleteSession(ctx context.Context, req *DeleteSessionRequest) error
    ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error)
}
```

### Runner (Orchestration)

```go
type Runner interface {
    RunAsync(ctx context.Context, req *RunRequest) (EventStream, error)
    Run(ctx context.Context, req *RunRequest) ([]*Event, error)
    Close(ctx context.Context) error
}
```

## Quick Start

### Basic Agent Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/agent-protocol/adk-golang/pkg/agents"
    "github.com/agent-protocol/adk-golang/pkg/runners"
    "github.com/agent-protocol/adk-golang/pkg/sessions"
    "github.com/agent-protocol/adk-golang/pkg/tools"
    "github.com/agent-protocol/adk-golang/pkg/core"
)

func main() {
    ctx := context.Background()

    // Create a simple tool
    greetingTool, err := tools.NewFunctionTool(
        "greeting",
        "Generates a greeting message",
        func(name string) string {
            return fmt.Sprintf("Hello, %s! Welcome to ADK for Go!", name)
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create an LLM agent
    agent := agents.NewLLMAgent(
        "assistant",
        "A helpful assistant",
        "gemini-2.0-flash",
    )
    agent.AddTool(greetingTool)

    // Create services
    sessionService := sessions.NewInMemorySessionService()
    runner := runners.NewRunner("my_app", agent, sessionService)

    // Run the agent
    request := &core.RunRequest{
        UserID:    "user123",
        SessionID: "session123",
        NewMessage: &core.Content{
            Role: "user",
            Parts: []core.Part{
                {
                    Type: "text",
                    Text: stringPtr("Hello! Please greet me."),
                },
            },
        },
    }

    events, err := runner.Run(ctx, request)
    if err != nil {
        log.Fatal(err)
    }

    // Process events
    for _, event := range events {
        fmt.Printf("Event from %s: %v\n", event.Author, event.Content)
    }
}

func stringPtr(s string) *string { return &s }
```

### Multi-Agent System

```go
// Create specialized agents
greeter := agents.NewBaseAgent("greeter", "Handles greetings")
taskExecutor := agents.NewLLMAgent("executor", "Executes tasks", "gemini-2.0-flash")

// Create coordinator agent
coordinator := agents.NewSequentialAgent("coordinator", "Coordinates multiple agents")
coordinator.AddSubAgent(greeter)
coordinator.AddSubAgent(taskExecutor)

// Use coordinator as root agent
runner := runners.NewRunner("multi_agent_app", coordinator, sessionService)
```

### Event Streaming

```go
// Stream events in real-time
eventStream, err := runner.RunAsync(ctx, request)
if err != nil {
    log.Fatal(err)
}

for event := range eventStream {
    // Process events as they arrive
    fmt.Printf("Real-time event: %s from %s\n", 
        getEventType(event), event.Author)
    
    // Check for function calls, responses, errors, etc.
    if event.ErrorMessage != nil {
        fmt.Printf("Error: %s\n", *event.ErrorMessage)
    }
}
```

## Features

✅ **Event-driven Architecture**: Streaming events with Go channels  
✅ **Multi-agent Systems**: Hierarchical agent composition  
✅ **Tool Integration**: Function tools with automatic schema generation  
✅ **Session Management**: Comprehensive session persistence and state management  
✅ **Context Propagation**: Proper `context.Context` usage throughout  
✅ **Error Handling**: Go-native error handling patterns  
✅ **Concurrency**: Goroutine-based async execution  
✅ **Type Safety**: Strong typing with interfaces and structs  

## Running the Example

```bash
cd examples/basic
go run main.go
```

You can also run the session management example:

```bash
cd examples/sessions  
go run main.go
```

This will demonstrate:
- Creating agents and tools
- Session management with multiple backends
- State management with scoped keys
- Event streaming
- Event handlers and lifecycle management
- Session utilities and backup/restore
- Agent hierarchy
- Tool execution

## Development Status

This Go implementation provides the core interfaces and basic implementations to demonstrate the ADK architecture. Key components implemented:

- ✅ Core interfaces (BaseAgent, BaseTool, Runner, etc.)
- ✅ Basic agent types (BaseAgent, LLMAgent, SequentialAgent)
- ✅ Tool system (FunctionTool, AgentTool)
- ✅ Event system with streaming support
- ✅ Comprehensive session management (memory and file persistence)
- ✅ State management with scoped keys (app, user, session, temp)
- ✅ Session event handlers (logging, metrics, validation)
- ✅ Session utilities (backup/restore, filtering, merging)
- ✅ Runner orchestration
- ⏳ LLM integrations (interface defined, implementations needed)
- ⏳ A2A protocol integration
- ⏳ Advanced toolsets (MCP, OpenAPI, etc.)
- ⏳ Database persistence options
- ⏳ CLI application

## Contributing

This project follows Go best practices and aims for compatibility with the Python ADK architecture. Contributions are welcome!

## License

Licensed under the Apache License, Version 2.0.