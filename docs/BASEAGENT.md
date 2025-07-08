# BaseAgent Interface and InvocationContext Implementation

## Overview

This document describes the implementation of the BaseAgent interface and InvocationContext in the ADK Go framework, which provides the core abstractions for building AI agents with proper context management and cancellation support.

## BaseAgent Interface

The `BaseAgent` interface defines the contract that all agents must implement. It provides a complete agent lifecycle with hierarchy support, callback mechanisms, and both synchronous and asynchronous execution patterns.

### Interface Definition

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
    
    // Agent Discovery
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

### Key Features

#### 1. Agent Hierarchy
- Agents can have parent-child relationships
- `FindAgent()` searches recursively through the entire hierarchy
- `FindSubAgent()` searches only direct children
- Automatic parent-child linking when adding sub-agents

#### 2. Execution Patterns
- **Asynchronous**: `RunAsync()` returns an event stream for real-time processing
- **Synchronous**: `Run()` collects all events and returns them at once
- Built-in context.Context support for cancellation and timeouts

#### 3. Lifecycle Callbacks
- **BeforeAgentCallback**: Called before agent execution starts
- **AfterAgentCallback**: Called after agent execution completes
- Enables cross-cutting concerns like logging, metrics, and validation

#### 4. Cancellation Support
All agent operations respect Go's context.Context for:
- Request timeouts
- Graceful cancellation
- Deadline propagation
- Resource cleanup

## InvocationContext

The `InvocationContext` provides comprehensive context for agent execution, including session management, service access, and hierarchical branching.

### Structure

```go
type InvocationContext struct {
    InvocationID      string
    Agent             BaseAgent
    Session           *Session
    SessionService    SessionService
    ArtifactService   ArtifactService
    MemoryService     MemoryService
    CredentialService CredentialService
    UserContent       *Content
    Branch            *string
    RunConfig         *RunConfig
    EndInvocation     bool
}
```

### Builder Pattern

The InvocationContext supports fluent configuration:

```go
invocationCtx := core.NewInvocationContext(invocationID, agent, session, sessionService).
    WithUserContent(userContent).
    WithBranch("main.sub_branch").
    WithRunConfig(&core.RunConfig{MaxTurns: intPtr(10)}).
    WithArtifactService(artifactService)
```

### Context Hierarchy

InvocationContext supports hierarchical branching for multi-agent workflows:

```go
// Create sub-context for a sub-agent
subCtx := invocationCtx.CreateSubContext(subAgent, "processing")
// Branch path becomes: "main.sub_branch.processing"
```

### Service Integration

The context provides optional service integrations:

- **SessionService**: Required for session management
- **ArtifactService**: Optional for file/blob storage
- **MemoryService**: Optional for long-term memory across sessions
- **CredentialService**: Optional for secure credential management

## Implementation Examples

### 1. Basic Agent

```go
// Create a simple agent
agent := agents.NewBaseAgent("helper", "A helpful assistant")
agent.SetInstruction("You are a helpful AI assistant")

// Set up callbacks
agent.SetBeforeAgentCallback(func(ctx context.Context, invocationCtx *core.InvocationContext) error {
    log.Printf("Starting agent: %s", invocationCtx.Agent.Name())
    return nil
})

// Create session and context
session, _ := sessionService.CreateSession(ctx, &core.CreateSessionRequest{
    AppName: "my_app",
    UserID:  "user123",
})

invocationCtx := core.NewInvocationContext("inv_001", agent, session, sessionService)

// Run the agent
events, err := agent.Run(ctx, invocationCtx)
```

### 2. Sequential Agent with Sub-agents

```go
// Create main agent
mainAgent := agents.NewSequentialAgent("workflow", "Multi-step workflow")

// Create sub-agents
dataAgent := agents.NewBaseAgent("data_processor", "Processes data")
analysisAgent := agents.NewBaseAgent("analyzer", "Analyzes results")

// Build hierarchy
mainAgent.AddSubAgent(dataAgent)
mainAgent.AddSubAgent(analysisAgent)

// Execute with automatic sub-agent coordination
stream, err := mainAgent.RunAsync(ctx, invocationCtx)
for event := range stream {
    fmt.Printf("Event from %s: %v\n", event.Author, event.Content)
}
```

### 3. Agent Discovery

```go
// Find agents in hierarchy
dataAgent := mainAgent.FindAgent("data_processor")  // Finds anywhere in hierarchy
directSub := mainAgent.FindSubAgent("analyzer")     // Finds only direct children

if dataAgent != nil {
    fmt.Printf("Found agent: %s\n", dataAgent.Name())
}
```

### 4. Context Propagation

```go
// Create hierarchical contexts for complex workflows
mainCtx := core.NewInvocationContext("main_001", mainAgent, session, sessionService).
    WithBranch("main")

// Each sub-agent gets its own context with inherited services
dataCtx := mainCtx.CreateSubContext(dataAgent, "data_processing")
// Branch becomes: "main.data_processing"

analysisCtx := mainCtx.CreateSubContext(analysisAgent, "analysis")  
// Branch becomes: "main.analysis"
```

## Concurrency and Safety

### Thread Safety
- InvocationContext is designed for single-threaded use within an agent execution
- Session services handle concurrent access internally
- Agent hierarchy is read-only during execution

### Cancellation
All operations respect context cancellation:

```go
// Set timeout for agent execution
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Agent will respect the timeout
events, err := agent.Run(ctx, invocationCtx)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Agent execution timed out")
}
```

### Resource Management
```go
// Always cleanup agents when done
defer func() {
    if err := agent.Cleanup(ctx); err != nil {
        log.Printf("Cleanup error: %v", err)
    }
}()
```

## Testing

The implementation includes comprehensive tests covering:

- Basic agent interface compliance
- Hierarchy management and agent discovery
- Context creation and propagation
- Callback execution
- Cancellation behavior
- Resource cleanup

Run tests with:
```bash
go test ./pkg/agents/ -v
```

## Example Application

See `examples/agent_demo/main.go` for a complete working example that demonstrates:

- Basic agent creation and execution
- Sequential agent coordination
- Agent hierarchy and discovery
- Callback usage
- Context management

Run the example:
```bash
cd examples/agent_demo && go run main.go
```

## Integration with ADK Framework

This BaseAgent implementation integrates seamlessly with other ADK components:

- **Tools**: Agents can use tools through the ToolContext
- **Sessions**: Full session management through SessionService
- **Events**: Event-driven communication between agents
- **A2A Protocol**: Remote agent communication
- **Runners**: Orchestrated execution environments

## Next Steps

With BaseAgent and InvocationContext implemented, the next development priorities are:

1. **Tool System**: Implement tool execution framework
2. **LLM Integration**: Enhanced LLM agent with tool calling
3. **A2A Protocol**: Remote agent communication
4. **Runner Implementation**: Orchestration layer
5. **CLI and API**: User interfaces

This implementation provides a solid foundation for the complete ADK Go framework.
