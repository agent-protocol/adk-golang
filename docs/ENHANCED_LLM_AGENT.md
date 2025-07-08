# Enhanced LLM Agent with Tool Execution Pipeline

## Overview

The Enhanced LLM Agent is a sophisticated implementation of an LLM-based agent in Go that provides comprehensive tool execution capabilities, conversation flow management, and error handling. It extends the base agent framework with advanced features for building intelligent, tool-enabled conversational AI systems.

## Key Features

### 🧠 Advanced LLM Integration
- **Configurable Model Parameters**: Temperature, max tokens, top-p, top-k
- **Retry Logic**: Automatic retry with exponential backoff for failed requests
- **Streaming Support**: Real-time response streaming with partial updates
- **Function Calling**: Native support for LLM function calling

### 🔧 Comprehensive Tool Execution Pipeline
- **Multi-Tool Support**: Execute multiple tools in sequence
- **Tool Timeout Management**: Configurable timeouts for tool execution
- **Error Handling**: Robust error handling with graceful degradation
- **Long-Running Tools**: Support for background/long-running operations
- **Tool Context**: Rich context passing between tools and agents

### 💬 Conversation Flow Management
- **Multi-Turn Conversations**: Maintain context across multiple turns
- **State Management**: Persistent state across conversation turns
- **Event Streaming**: Real-time event streaming for UI integration
- **Turn Completion Detection**: Automatic detection of conversation completion

### 📊 Monitoring & Callbacks
- **Lifecycle Callbacks**: Before/after hooks for model and tool execution
- **Event Tracking**: Comprehensive event logging and tracking
- **Performance Monitoring**: Built-in timing and performance metrics

## Architecture

### Core Components

```
EnhancedLlmAgent
├── Configuration (LlmAgentConfig)
├── Tools ([]BaseTool)
├── LLM Connection (LLMConnection)
├── Callbacks (LlmAgentCallbacks)
└── Execution Pipeline
    ├── Conversation Flow Manager
    ├── Tool Execution Engine
    ├── Retry Manager
    └── Event Streamer
```

### Tool Execution Pipeline

1. **Request Processing**: Parse user input and build LLM request
2. **LLM Call**: Send request to language model with retry logic
3. **Function Call Detection**: Identify and extract function calls
4. **Tool Execution**: Execute tools with timeout and error handling
5. **Response Integration**: Integrate tool responses back into conversation
6. **Final Response**: Generate final response to user

## Configuration

### LlmAgentConfig

```go
type LlmAgentConfig struct {
    Model             string        // LLM model name (e.g., "gpt-4", "gemini-1.5-pro")
    Temperature       *float32      // Randomness (0.0-1.0)
    MaxTokens         *int          // Maximum response tokens
    TopP              *float32      // Top-p sampling (0.0-1.0)
    TopK              *int          // Top-k sampling
    SystemInstruction *string       // System prompt/instruction
    MaxToolCalls      int           // Maximum tool calls per turn
    ToolCallTimeout   time.Duration // Timeout for individual tool calls
    RetryAttempts     int           // Number of retry attempts for failed LLM calls
    StreamingEnabled  bool          // Enable response streaming
}
```

### Default Configuration

```go
config := agents.DefaultLlmAgentConfig()
// Returns:
// - Model: "gemini-1.5-pro"
// - Temperature: 0.7
// - MaxTokens: 4096
// - MaxToolCalls: 10
// - ToolCallTimeout: 30 seconds
// - RetryAttempts: 3
// - StreamingEnabled: false
```

## Usage Examples

### Basic Setup

```go
// Create agent with default configuration
agent := agents.NewEnhancedLlmAgent(
    "my-assistant",
    "A helpful AI assistant",
    nil, // Uses default config
)

// Set system instruction
agent.SetInstruction("You are a helpful assistant with access to various tools.")

// Set LLM connection
agent.SetLLMConnection(myLLMConnection)

// Add tools
agent.AddTool(NewCalculatorTool())
agent.AddTool(NewWeatherTool())
```

### Custom Configuration

```go
config := &agents.LlmAgentConfig{
    Model:            "gpt-4",
    Temperature:      floatPtr(0.5),
    MaxTokens:        intPtr(2048),
    MaxToolCalls:     5,
    ToolCallTimeout:  15 * time.Second,
    RetryAttempts:    2,
    StreamingEnabled: true,
}

agent := agents.NewEnhancedLlmAgent("custom-agent", "Custom assistant", config)
```

### Tool Management

```go
// Add tools
calculator := NewCalculatorTool()
weather := NewWeatherTool()
search := NewWebSearchTool()

agent.AddTool(calculator)
agent.AddTool(weather)
agent.AddTool(search)

// Get tool by name
if tool, exists := agent.GetTool("calculator"); exists {
    fmt.Printf("Found tool: %s", tool.Name())
}

// Remove tool
removed := agent.RemoveTool("search")
if removed {
    fmt.Println("Search tool removed")
}

// List all tools
tools := agent.Tools()
fmt.Printf("Agent has %d tools", len(tools))
```

### Conversation Execution

```go
// Create session and context
session := core.NewSession("session-1", "my-app", "user-123")
invocationCtx := core.NewInvocationContext("inv-1", agent, session, sessionService)

// Set user input
invocationCtx.UserContent = &core.Content{
    Role: "user",
    Parts: []core.Part{
        {
            Type: "text",
            Text: stringPtr("Calculate the square root of 16"),
        },
    },
}

// Execute agent (synchronous)
ctx := context.Background()
events, err := agent.Run(ctx, invocationCtx)
if err != nil {
    log.Fatalf("Agent execution failed: %v", err)
}

// Process events
for _, event := range events {
    fmt.Printf("Event from %s: %s\n", event.Author, getEventText(event))
}
```

### Asynchronous Execution with Streaming

```go
// Execute agent asynchronously
eventStream, err := agent.RunAsync(ctx, invocationCtx)
if err != nil {
    log.Fatalf("Failed to start agent: %v", err)
}

// Process events as they arrive
for event := range eventStream {
    if event.Content != nil {
        // Handle different event types
        if event.Partial != nil && *event.Partial {
            fmt.Print("Partial response...")
        } else {
            fmt.Println("Complete response received")
        }
        
        // Process function calls
        for _, part := range event.Content.Parts {
            switch part.Type {
            case "text":
                if part.Text != nil {
                    fmt.Printf("Text: %s\n", *part.Text)
                }
            case "function_call":
                if part.FunctionCall != nil {
                    fmt.Printf("Calling tool: %s\n", part.FunctionCall.Name)
                }
            case "function_response":
                if part.FunctionResponse != nil {
                    fmt.Printf("Tool response from: %s\n", part.FunctionResponse.Name)
                }
            }
        }
    }
    
    if event.ErrorMessage != nil {
        fmt.Printf("Error: %s\n", *event.ErrorMessage)
    }
}
```

### Callbacks and Monitoring

```go
callbacks := &agents.LlmAgentCallbacks{
    BeforeModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
        log.Println("About to call LLM")
        return nil
    },
    AfterModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
        log.Printf("LLM call completed with %d events", len(events))
        return nil
    },
    BeforeToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
        log.Println("About to execute tool")
        return nil
    },
    AfterToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
        log.Println("Tool execution completed")
        return nil
    },
}

agent.SetCallbacks(callbacks)
```

## Creating Custom Tools

### Basic Tool Implementation

```go
type MyCustomTool struct {
    *tools.BaseToolImpl
}

func NewMyCustomTool() *MyCustomTool {
    return &MyCustomTool{
        BaseToolImpl: tools.NewBaseTool("my_tool", "Description of my tool"),
    }
}

func (t *MyCustomTool) GetDeclaration() *core.FunctionDeclaration {
    return &core.FunctionDeclaration{
        Name:        "my_tool",
        Description: "Description of what this tool does",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "param1": map[string]interface{}{
                    "type":        "string",
                    "description": "Description of param1",
                },
                "param2": map[string]interface{}{
                    "type":        "integer",
                    "description": "Description of param2",
                },
            },
            "required": []string{"param1"},
        },
    }
}

func (t *MyCustomTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
    param1, ok := args["param1"].(string)
    if !ok {
        return nil, fmt.Errorf("param1 must be a string")
    }
    
    param2, _ := args["param2"].(float64) // Optional parameter
    
    // Implement your tool logic here
    result := fmt.Sprintf("Processed %s with %f", param1, param2)
    
    // Optionally update state
    toolCtx.Actions.StateDelta = map[string]any{
        "last_tool_result": result,
    }
    
    return result, nil
}
```

### Long-Running Tool

```go
type LongRunningTool struct {
    *tools.BaseToolImpl
}

func NewLongRunningTool() *LongRunningTool {
    tool := &LongRunningTool{
        BaseToolImpl: tools.NewBaseTool("long_task", "A tool that takes time to complete"),
    }
    tool.SetLongRunning(true) // Mark as long-running
    return tool
}

func (t *LongRunningTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
    // Simulate long-running operation
    select {
    case <-time.After(5 * time.Second):
        return "Long task completed", nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

## Streaming Agent

For real-time response streaming:

```go
// Create streaming agent
streamingAgent := agents.NewStreamingLlmAgent("stream-agent", "Streaming assistant", config)
streamingAgent.SetLLMConnection(streamingLLMConnection)

// Execute with streaming
eventStream, err := streamingAgent.RunAsync(ctx, invocationCtx)
if err != nil {
    log.Fatalf("Streaming failed: %v", err)
}

// Handle streaming events
for event := range eventStream {
    if event.Partial != nil && *event.Partial {
        // Handle partial response (e.g., update UI progressively)
        updatePartialResponse(event)
    } else {
        // Handle complete response
        displayFinalResponse(event)
    }
}
```

## Error Handling

The Enhanced LLM Agent provides comprehensive error handling:

### Retry Logic
- Automatic retry for transient failures
- Exponential backoff between retries
- Configurable retry attempts

### Tool Execution Errors
- Individual tool failures don't stop the conversation
- Error responses are passed back to the LLM
- Timeout protection for long-running tools

### LLM Connection Errors
- Connection failures are retried automatically
- Graceful degradation when LLM is unavailable
- Detailed error messages for debugging

## Performance Considerations

### Tool Timeout Management
```go
config := &agents.LlmAgentConfig{
    ToolCallTimeout: 30 * time.Second, // Adjust based on your tools
}
```

### Conversation Turn Limits
```go
runConfig := &core.RunConfig{
    MaxTurns: 20, // Prevent infinite loops
}
invocationCtx.RunConfig = runConfig
```

### Memory Management
- Session events are stored in memory
- Consider implementing session cleanup for long conversations
- Use artifact services for large data storage

## Integration with A2A Protocol

The Enhanced LLM Agent is fully compatible with the A2A (Agent-to-Agent) protocol:

```go
// Wrap agent for A2A exposure
a2aAgent := a2a.NewA2AAgent(agent)

// Start A2A server
server := a2a.NewServer(a2aAgent)
server.Listen(":8080")
```

## Best Practices

### 1. Tool Design
- Keep tools focused and single-purpose
- Provide clear parameter descriptions
- Handle errors gracefully
- Use appropriate timeouts

### 2. System Instructions
- Be specific about tool usage
- Provide context about available capabilities
- Include error handling instructions

### 3. Configuration
- Start with default config and adjust as needed
- Monitor performance and adjust timeouts
- Use streaming for better user experience

### 4. Error Handling
- Always check for errors in tool implementations
- Provide meaningful error messages
- Log errors for debugging

### 5. Testing
- Use mock LLM connections for testing
- Test tool execution independently
- Verify conversation flow with different scenarios

## Complete Example

See `examples/enhanced_llm_agent/main.go` for a complete working example that demonstrates:

- Setting up an enhanced LLM agent
- Adding multiple types of tools (calculator, weather, web search)
- Handling conversation flow
- Processing function calls and responses
- Managing agent lifecycle with callbacks

## Troubleshooting

### Common Issues

1. **Tool not being called**: Check tool declaration and ensure it's properly registered
2. **Timeout errors**: Increase `ToolCallTimeout` in configuration
3. **LLM connection failures**: Verify LLM connection setup and credentials
4. **Memory issues**: Implement session cleanup for long conversations

### Debugging

Enable verbose logging and use callbacks to monitor execution:

```go
callbacks := &agents.LlmAgentCallbacks{
    BeforeModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
        log.Printf("LLM Request: %+v", buildDebugRequest(invocationCtx))
        return nil
    },
    AfterToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
        log.Printf("Tool completed with %d events", len(events))
        return nil
    },
}
```

This comprehensive implementation provides a solid foundation for building sophisticated LLM-based agents with tool execution capabilities in Go.
