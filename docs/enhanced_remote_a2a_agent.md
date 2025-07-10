# Enhanced RemoteA2aAgent Configuration and Usage Guide

This document provides a comprehensive guide for configuring and using the RemoteA2aAgent in ADK Golang, with focus on task waiting, streaming, and agent card inspection.

## Overview

The RemoteA2aAgent enables communication with remote A2A (Agent-to-Agent) agents. The enhanced version provides sophisticated task handling capabilities including:

- **Automatic streaming detection** based on agent capabilities
- **Task polling** for long-running operations
- **Configurable waiting strategies** 
- **Agent card inspection** to understand remote agent capabilities
- **Error handling and retries**

## Key Improvements

### 1. Task Waiting Strategies

The enhanced RemoteA2aAgent supports multiple strategies for waiting for task completion:

```go
type TaskWaitingStrategy int

const (
    TaskWaitingNone   // Don't wait, return immediately
    TaskWaitingPoll   // Poll for task completion using GetTask
    TaskWaitingStream // Use streaming if supported by agent
    TaskWaitingAuto   // Automatically choose based on agent capabilities
)
```

### 2. Agent Card Capability Detection

The agent automatically inspects the remote agent's capabilities from the agent card:

```go
// Check if remote agent supports streaming
func (r *RemoteA2aAgent) shouldUseStreaming() bool {
    if !r.config.PreferStreaming {
        return false
    }
    
    if r.agentCard != nil && r.agentCard.Capabilities.Streaming {
        return true
    }
    
    return false
}
```

### 3. Enhanced Configuration Options

```go
type RemoteA2aAgentConfig struct {
    // Basic configuration
    Timeout    time.Duration
    HTTPClient *http.Client
    Headers    map[string]string
    
    // Task configuration
    TaskPollingEnabled   bool
    TaskPollingInterval  time.Duration
    TaskPollingTimeout   time.Duration
    
    // Streaming configuration
    PreferStreaming bool
}

// Enhanced configuration with more options
type EnhancedRemoteA2aAgentConfig struct {
    // All basic options plus:
    TaskWaitingStrategy  TaskWaitingStrategy
    TaskPollingInterval  time.Duration
    TaskPollingTimeout   time.Duration
    MaxTaskPollingTries  int
    
    ForceStreaming      bool
    StreamingTimeout    time.Duration
    StreamingBufferSize int
    
    MaxRetries       int
    RetryBackoff     time.Duration
    RetryableErrors  []string
}
```

## Usage Examples

### Basic Usage with Automatic Configuration

```go
// Create agent with default configuration
agent, err := agents.NewRemoteA2aAgentFromURL(
    "my_agent",
    "http://localhost:8001/.well-known/agent.json",
    nil, // Use defaults
)
if err != nil {
    log.Fatal(err)
}
defer agent.Close()

// The agent will automatically:
// - Fetch and validate the agent card
// - Choose streaming vs polling based on capabilities
// - Handle task waiting appropriately
```

### Custom Task Polling Configuration

```go
config := agents.DefaultRemoteA2aAgentConfig()
config.TaskPollingEnabled = true
config.TaskPollingInterval = 1 * time.Second  // Poll every second
config.TaskPollingTimeout = 60 * time.Second  // Wait up to 1 minute
config.PreferStreaming = false                // Force polling over streaming

agent, err := agents.NewRemoteA2aAgentFromURL("agent", url, config)
```

### Enhanced Agent with Advanced Features

```go
config := agents.DefaultEnhancedRemoteA2aAgentConfig()
config.TaskWaitingStrategy = agents.TaskWaitingAuto
config.TaskPollingTimeout = 300 * time.Second
config.MaxRetries = 3
config.RetryBackoff = 2 * time.Second

agent, err := agents.NewEnhancedRemoteA2aAgentFromURL("agent", url, config)
```

### Streaming-Only Configuration

```go
config := agents.DefaultEnhancedRemoteA2aAgentConfig()
config.TaskWaitingStrategy = agents.TaskWaitingStream
config.ForceStreaming = true                   // Use streaming even if not advertised
config.StreamingTimeout = 60 * time.Second
config.StreamingBufferSize = 100

agent, err := agents.NewEnhancedRemoteA2aAgentFromURL("agent", url, config)
```

## Agent Card Inspection

You can inspect remote agent capabilities before using them:

```go
// Create and resolve agent
agent, err := agents.NewRemoteA2aAgentFromURL("agent", url, nil)
if err != nil {
    log.Fatal(err)
}

// Ensure agent card is resolved
ctx := context.Background()
if err := agent.EnsureResolved(ctx); err != nil {
    log.Fatal(err)
}

// Inspect capabilities
card := agent.GetAgentCard()
if card != nil {
    fmt.Printf("Agent: %s\n", card.Name)
    fmt.Printf("Streaming supported: %t\n", card.Capabilities.Streaming)
    fmt.Printf("Push notifications: %t\n", card.Capabilities.PushNotifications)
    
    // List available skills
    for _, skill := range card.Skills {
        fmt.Printf("Skill: %s - %s\n", skill.Name, 
            func() string {
                if skill.Description != nil {
                    return *skill.Description
                }
                return "No description"
            }())
    }
}
```

## Task State Handling

The agent properly handles different task states according to the A2A specification:

```go
// Terminal states that end task polling
- TaskStateCompleted  // Task finished successfully
- TaskStateFailed     // Task failed with error
- TaskStateCanceled   // Task was canceled

// Non-terminal states that continue polling
- TaskStateSubmitted     // Task received but not started
- TaskStateWorking      // Task is being processed
- TaskStateInputRequired // Task needs user input
```

## Error Handling and Retries

The enhanced agent includes robust error handling:

```go
config := agents.DefaultEnhancedRemoteA2aAgentConfig()
config.MaxRetries = 3
config.RetryBackoff = 1 * time.Second
config.RetryableErrors = []string{
    "timeout",
    "connection refused", 
    "temporary failure",
}
```

## Best Practices

### 1. Choose the Right Strategy

- **TaskWaitingAuto**: Best default choice, adapts to agent capabilities
- **TaskWaitingStream**: Use for real-time updates and long-running tasks
- **TaskWaitingPoll**: Use when streaming is not available or desired
- **TaskWaitingNone**: Use for fire-and-forget operations

### 2. Configure Timeouts Appropriately

```go
// For quick operations
config.TaskPollingTimeout = 30 * time.Second
config.TaskPollingInterval = 1 * time.Second

// For long-running operations  
config.TaskPollingTimeout = 600 * time.Second  // 10 minutes
config.TaskPollingInterval = 5 * time.Second   // Poll every 5 seconds
```

### 3. Handle Different Response Types

```go
eventChan, err := agent.RunAsync(invocationCtx)
if err != nil {
    log.Fatal(err)
}

for event := range eventChan {
    if event.Content != nil && len(event.Content.Parts) > 0 {
        for _, part := range event.Content.Parts {
            if part.Text != nil {
                fmt.Printf("Response: %s\n", *part.Text)
            }
            // Handle other part types (files, data, etc.)
        }
    }
}
```

### 4. Monitor Agent Performance

```go
start := time.Now()
eventChan, err := agent.RunAsync(invocationCtx)
// ... process events ...
duration := time.Since(start)
fmt.Printf("Task completed in %v\n", duration)
```

## Troubleshooting

### Common Issues

1. **Agent card resolution fails**
   - Check URL accessibility
   - Verify JSON format
   - Ensure proper authentication if required

2. **Task polling timeout** 
   - Increase `TaskPollingTimeout`
   - Reduce `TaskPollingInterval` for more frequent checks
   - Check if task is actually long-running

3. **Streaming not working**
   - Verify agent supports streaming in capabilities
   - Check if streaming endpoint is accessible
   - Try forcing streaming with `ForceStreaming = true`

4. **Connection issues**
   - Configure retry settings
   - Check network connectivity
   - Verify agent server is running

### Debug Configuration

```go
config := agents.DefaultEnhancedRemoteA2aAgentConfig()
config.TaskPollingInterval = 1 * time.Second  // More frequent polling
config.MaxRetries = 5                         // More retry attempts
config.RetryBackoff = 500 * time.Millisecond  // Faster retries

// Enable verbose logging (if available)
config.VerboseLogging = true
```

## Migration from Basic to Enhanced Agent

Existing code using `RemoteA2aAgent` can be easily upgraded:

```go
// Before (basic agent)
agent, err := agents.NewRemoteA2aAgentFromURL("agent", url, basicConfig)

// After (enhanced agent) 
enhancedConfig := convertToEnhancedConfig(basicConfig)
agent, err := agents.NewEnhancedRemoteA2aAgentFromURL("agent", url, enhancedConfig)
```

The enhanced agent maintains backward compatibility while providing additional features.

## Complete Example

See `examples/enhanced_remote_a2a_agent/main.go` for a complete working example that demonstrates:
- Basic usage with auto-detection
- Custom polling configuration  
- Forced streaming
- Agent capability inspection
- Error handling patterns

This enhanced RemoteA2aAgent provides a robust foundation for building reliable multi-agent systems using the A2A protocol.
