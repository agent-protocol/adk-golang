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
- **Tools**: `BaseTool`, `FunctionTool`, `GoogleSearchTool`, `EnhancedAgentTool`
- **Runner**: Orchestrates agent execution with real-time event streaming
- **Sessions**: Advanced session management with scoped state and persistence
- **Events**: Communication units between agents with streaming support
- **A2A Integration**: Complete A2A protocol implementation for remote agents
- **CLI**: Comprehensive command-line interface for all operations
- **API Server**: HTTP API with Web UI for testing and production deployment

## Project Structure

```
adk-golang/
├── cmd/
│   └── adk/              # CLI application with create, run, web, eval commands
├── pkg/
│   ├── agents/           # Agent implementations (Base, LLM, Sequential)
│   ├── tools/            # Tool system (Function, Google Search, Agent tools)
│   ├── sessions/         # Session management (Memory, File, State, Handlers)
│   ├── runners/          # Execution orchestration with event streaming
│   ├── a2a/              # A2A protocol implementation and converters
│   ├── api/              # HTTP API server and Web UI
│   └── cli/              # CLI command implementations
├── internal/
│   ├── core/             # Core types and interfaces
│   ├── llm/              # LLM integrations (interface ready)
│   └── utils/            # Utilities and helpers
├── examples/             # Comprehensive examples and demos
└── docs/                 # Detailed implementation documentation
```

## Core Interfaces

### BaseAgent
Provides the foundation for all agent types with hierarchy support, lifecycle callbacks, and execution patterns:

```go
type BaseAgent interface {
    // Basic Properties
    Name() string
    Description() string
    Instruction() string
    
    // Hierarchy Management
    SubAgents() []BaseAgent
    ParentAgent() BaseAgent
    SetParentAgent(parent BaseAgent)
    FindAgent(name string) BaseAgent
    FindSubAgent(name string) BaseAgent
    
    // Execution Methods
    RunAsync(ctx context.Context, invocationCtx *InvocationContext) (EventStream, error)
    Run(ctx context.Context, invocationCtx *InvocationContext) ([]*Event, error)
    
    // Lifecycle Callbacks
    GetBeforeAgentCallback() BeforeAgentCallback
    SetBeforeAgentCallback(callback BeforeAgentCallback)
    GetAfterAgentCallback() AfterAgentCallback
    SetAfterAgentCallback(callback AfterAgentCallback)
    
    // Cleanup
    Cleanup(ctx context.Context) error
}
```

### BaseTool
Interface for tools that agents can use, including built-in tools and custom functions:

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
Real-time communication units with comprehensive metadata and actions:

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
    Partial             *bool         `json:"partial,omitempty"`
    ErrorMessage        *string       `json:"error_message,omitempty"`
}

type EventStream <-chan *Event
```

### Enhanced Session Management
Comprehensive session system with scoped state, event handlers, and persistence:

```go
type SessionService interface {
    CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error)
    GetSession(ctx context.Context, req *GetSessionRequest) (*Session, error)
    AppendEvent(ctx context.Context, session *Session, event *Event) error
    DeleteSession(ctx context.Context, req *DeleteSessionRequest) error
    ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error)
    UpdateSessionState(ctx context.Context, session *Session, stateDelta map[string]any) error
    GetSessionMetadata(ctx context.Context, appName, userID, sessionID string) (*SessionMetadata, error)
    // ... additional session management methods
}
```

### Runner (Orchestration Engine)
Manages agent execution with real-time event streaming and state management:

```go
type Runner interface {
    RunAsync(ctx context.Context, req *RunRequest) (EventStream, error)
    Run(ctx context.Context, req *RunRequest) ([]*Event, error)
    Close(ctx context.Context) error
}
```

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/agent-protocol/adk-golang.git
cd adk-golang

# Build the CLI
go build -o bin/adk ./cmd/adk

# Or install to system PATH
go install ./cmd/adk
```

### 1. Create Your First Agent

```bash
# Create a new agent with template
./bin/adk create my-assistant --model gemini-2.0-flash

cd my-assistant
```

This creates a basic agent structure:
```
my-assistant/
├── agent.go        # Main agent implementation
├── go.mod         # Go module file
├── .env           # Environment variables
└── README.md      # Agent documentation
```

### 2. Run Your Agent

```bash
# Interactive mode - chat with your agent
./bin/adk run my-assistant

# Web UI mode - browser-based testing
./bin/adk web my-assistant --port 8080
# Visit http://localhost:8080

# API server mode - for production deployment
./bin/adk api-server my-assistant --port 8000
```

### 3. Basic Agent Example

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

### 4. Advanced Multi-Agent Example

```go
// Create specialized agents
researchAgent := agents.NewLLMAgent("researcher", "Conducts research", "gemini-2.0-flash")
analysisAgent := agents.NewLLMAgent("analyst", "Analyzes data", "gemini-2.0-flash")
writerAgent := agents.NewLLMAgent("writer", "Creates content", "gemini-2.0-flash")

// Create tools from agents for agent-to-agent communication
researchTool := tools.NewEnhancedAgentTool(researchAgent)
analysisTool := tools.NewEnhancedAgentTool(analysisAgent)
writingTool := tools.NewEnhancedAgentTool(writerAgent)

// Create coordinator agent that uses specialist agents as tools
coordinator := agents.NewLLMAgent("coordinator", "Coordinates multi-agent workflow", "gemini-2.0-flash")
coordinator.AddTool(researchTool)
coordinator.AddTool(analysisTool)
coordinator.AddTool(writingTool)

// Add Google Search capability
coordinator.AddTool(tools.GlobalGoogleSearchTool)

// Use coordinator as root agent
runner := runners.NewRunner("multi_agent_app", coordinator, sessionService)
```

### 5. Event Streaming Example

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
    
    // Handle different event types
    if event.Content != nil {
        for _, part := range event.Content.Parts {
            switch part.Type {
            case "text":
                if part.Text != nil {
                    fmt.Printf("Text: %s\n", *part.Text)
                }
            case "function_call":
                if part.FunctionCall != nil {
                    fmt.Printf("Calling function: %s\n", part.FunctionCall.Name)
                }
            case "function_response":
                if part.FunctionResponse != nil {
                    fmt.Printf("Function result: %v\n", part.FunctionResponse.Response)
                }
            }
        }
    }
    
    // Check for errors
    if event.ErrorMessage != nil {
        fmt.Printf("Error: %s\n", *event.ErrorMessage)
    }
}
```

## Advanced Features

### 🔧 Comprehensive Tool System

#### Google Search Tool
Built-in Google Search integration optimized for Gemini models:
```go
// Add Google Search capability to any agent
agent.AddTool(tools.GlobalGoogleSearchTool)

// Model-aware configuration (Gemini 1.x vs 2.x+ handling)
// Automatic constraint enforcement
// Zero local execution (operates as built-in model capability)
```

#### Enhanced Agent Tool
Enable agent-to-agent communication via tool interface:
```go
// Wrap any agent as a tool for multi-agent workflows
mathAgent := agents.NewLLMAgent("math_specialist", "Solves math problems", "gemini-2.0-flash")
mathTool := tools.NewEnhancedAgentTool(mathAgent)

// Advanced configuration options
config := &tools.AgentToolConfig{
    Timeout:           30 * time.Second,
    IsolateState:      true,  // Optional state isolation
    ErrorStrategy:     tools.ErrorStrategyReturnError,
    CustomInstruction: "Focus on step-by-step solutions",
}
```

#### Enhanced Function Tool
Automatic Go function wrapping with advanced features:
```go
// Automatic parameter extraction and JSON schema generation
func Calculate(operation string, a, b float64) (float64, error) {
    switch operation {
    case "add": return a + b, nil
    case "multiply": return a * b, nil
    default: return 0, fmt.Errorf("unsupported operation: %s", operation)
    }
}

calculatorTool, _ := tools.NewFunctionTool("calculator", "Mathematical operations", Calculate)
agent.AddTool(calculatorTool)
```

### 🗃️ Advanced Session Management

#### Scoped State Management
```go
// Four types of state scope with automatic inheritance
session.SetState("session_key", "value")           // Session-specific
session.SetState("user:preference", "dark_mode")   // User-wide
session.SetState("app:version", "1.0.0")          // Application-wide  
session.SetState("temp:processing", true)          // Temporary (not persisted)
```

#### Session Event Handlers
```go
// Pluggable event system for monitoring and metrics
logger := sessions.NewLoggingEventHandler()
metrics := sessions.NewMetricsEventHandler()
validator := sessions.NewValidationEventHandler()

compositeHandler := sessions.NewCompositeEventHandler(logger, metrics, validator)
sessionService.AddEventHandler(compositeHandler)
```

#### Session Utilities and Helpers
```go
helper := core.NewSessionStateHelper(session)

// Type-safe operations with automatic conversions
helper.SetString("username", "Alice")
helper.SetInt("score", 100)
newScore, _ := helper.Increment("score", 25)
helper.Toggle("premium_user")

// Collection operations
helper.AppendToSlice("tags", "premium")
helper.SetJSON("profile", userProfile)
```

### 🌐 HTTP API Server & Web UI

#### Interactive Web Interface
```bash
# Start web server with interactive UI
./bin/adk web agents/ --port 8080

# Visit http://localhost:8080 for browser-based testing
```

#### Complete API Endpoints
- **POST `/run`** - Synchronous agent execution
- **POST `/run_sse`** - Server-Sent Events streaming
- **WebSocket `/run_live`** - Live bidirectional communication
- **POST `/a2a`** - A2A protocol endpoint
- **Session Management API** - Full CRUD operations

#### Production Configuration
```bash
./bin/adk api-server agents/ \
  --port 8000 \
  --session-service-uri "sqlite://sessions.db" \
  --artifact-service-uri "gs://my-bucket" \
  --allow-origins "https://myapp.com" \
  --a2a
```

### 🔄 A2A Protocol Integration

Complete Agent-to-Agent communication protocol implementation:

#### A2A Agent Executor
```go
// Convert any ADK agent to A2A protocol
executor := a2a.NewA2aAgentExecutor(runner)

// Process A2A requests with real-time event streaming
a2aRequest := &a2a.A2aRequest{
    AgentName: "my-agent",
    Message: &a2a.Message{
        Parts: []a2a.Part{{Text: "Hello from A2A!"}},
    },
}

eventQueue := a2a.NewSimpleEventQueue(100)
err := executor.Execute(ctx, a2aRequest, eventQueue)
```

#### Event Conversion
Bidirectional conversion between ADK events and A2A protocol messages:
- Text messages with metadata preservation
- Function calls via data parts with proper metadata
- Long-running tool support with `input-required` state
- Intelligent task state mapping (working, completed, failed)

### 🚀 Enhanced Runner with Event Streaming

Real-time agent execution with comprehensive orchestration:

```go
// Configure advanced runner options
config := &runners.RunnerConfig{
    EventBufferSize:          100,
    EnableEventProcessing:    true,
    MaxConcurrentSessions:    50,
    DefaultTimeout:          30 * time.Second,
}

runner := runners.NewRunnerWithConfig("my-app", agent, sessionService, config)

// Real-time streaming with context cancellation
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

eventStream, err := runner.RunAsync(ctx, request)
for event := range eventStream {
    // Events arrive in real-time as agent processes them
    processEvent(event)
}
```

### 🛠️ Comprehensive CLI

Full command-line interface matching Python ADK functionality:

```bash
# Agent lifecycle management
./bin/adk create my-agent --model gemini-2.0-flash
./bin/adk run my-agent --save-session --session-id interactive-123

# Web services
./bin/adk web agents/ --port 8080 --a2a
./bin/adk api-server agents/ --port 8000

# Evaluation and deployment (planned)
./bin/adk eval my-agent eval-set.json
./bin/adk deploy cloud-run my-agent --project my-project
```

### 🧪 Async Tool System

Advanced asynchronous tool execution with progress tracking:

```go
type AsyncTool interface {
    core.BaseTool
    RunStream(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (*ToolStream, error)
    CanCancel() bool
    Cancel(ctx context.Context, toolID string) error
    GetStatus(ctx context.Context, toolID string) (*ToolProgress, error)
}

// Real-time progress updates via channels
stream, _ := asyncTool.RunStream(ctx, args, toolCtx)
for progress := range stream.Progress {
    fmt.Printf("Progress: %.1f%% - %s\n", progress.Progress*100, progress.Message)
}
```

## Implementation Status

This Go implementation provides comprehensive feature parity with the Python ADK plus additional enhancements:

### ✅ Completed Features

- **Core Framework**: Complete interfaces and base implementations
- **Agent System**: BaseAgent, LLMAgent, SequentialAgent with full hierarchy support
- **Advanced Tool System**: 
  - FunctionTool with automatic Go function wrapping
  - GoogleSearchTool with built-in Gemini integration
  - EnhancedAgentTool for multi-agent workflows
- **Comprehensive Session Management**: 
  - Multiple backends (memory, file)
  - Scoped state management (app, user, session, temp)
  - Event handlers and utilities
  - Backup/restore and session analytics
- **Enhanced Runner**: Real-time event streaming with configurable orchestration
- **Complete A2A Integration**: 
  - A2aAgentExecutor with event conversion
  - Bidirectional protocol conversion
  - Request/response handling
- **HTTP API Server**: 
  - Complete REST API endpoints
  - Server-Sent Events streaming
  - WebSocket support
  - Interactive Web UI
- **CLI Application**: Full command-line interface with all commands
- **Testing**: Comprehensive test suites throughout
- **Documentation**: Extensive documentation and examples

### 🔄 In Progress
- **LLM Provider Integrations**: Interface defined, implementations needed for specific providers
- **Advanced Evaluation Framework**: CLI structure ready, evaluation logic needed
- **Cloud Deployment**: CLI commands ready, deployment automation needed

### 📋 Planned Enhancements
- **Database Persistence**: Additional session backends (PostgreSQL, MongoDB)
- **Advanced Toolsets**: MCP (Model Context Protocol) integration, OpenAPI tools
- **Performance Optimizations**: Connection pooling, caching layers
- **Observability**: Metrics, tracing, and monitoring integrations

### Architectural Enhancements

1. **Better Concurrency**: Native goroutines provide superior concurrent execution
2. **Real-time Streaming**: Go channels enable efficient real-time event processing
3. **Type Safety**: Compile-time validation prevents runtime errors
4. **Resource Efficiency**: Lower memory usage and faster execution
5. **Deployment Simplicity**: Single binary deployment with no dependencies
6. **Context Propagation**: Built-in cancellation and timeout support

## Testing

### Run All Tests
```bash
# Run entire test suite
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Links

- **ADK Documentation**: https://google.github.io/adk-docs/
- **A2A Protocol**: https://a2aproject.github.io/A2A/latest/specification/
- **Python ADK**: https://google.github.io/adk-docs/api-reference/python/
- **Java ADK**: https://google.github.io/adk-docs/api-reference/java/
- **Examples Repository**: https://github.com/agent-protocol/adk-samples