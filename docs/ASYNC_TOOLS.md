# Async Tool System Design

## Overview

The async tool system in ADK Go provides advanced asynchronous execution capabilities using Go's native concurrency features (goroutines and channels) to replicate Python's async/await patterns.

## Core Architecture

### 1. AsyncTool Interface

```go
type AsyncTool interface {
    core.BaseTool
    
    // RunStream executes the tool and returns a stream of progress updates and final result
    RunStream(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (*ToolStream, error)
    
    // CanCancel indicates if the tool supports cancellation during execution
    CanCancel() bool
    
    // Cancel cancels a running tool execution by ID
    Cancel(ctx context.Context, toolID string) error
    
    // GetStatus returns the current status of a running tool execution
    GetStatus(ctx context.Context, toolID string) (*ToolProgress, error)
}
```

### 2. Streaming Components

#### ToolStream
```go
type ToolStream struct {
    Progress <-chan *ToolProgress  // Real-time progress updates
    Result   <-chan *ToolResult    // Final result or error
    Cancel   context.CancelFunc    // Cancellation function
}
```

#### ToolProgress
```go
type ToolProgress struct {
    ID          string         `json:"id"`
    Progress    float64        `json:"progress"`    // 0.0 to 1.0
    Message     string         `json:"message"`     // Human-readable status
    Metadata    map[string]any `json:"metadata,omitempty"`
    Timestamp   time.Time      `json:"timestamp"`
    Cancelable  bool           `json:"cancelable"`
}
```

#### ToolResult
```go
type ToolResult struct {
    Result any    `json:"result,omitempty"`
    Error  error  `json:"error,omitempty"`
    Done   bool   `json:"done"`
    ID     string `json:"id"`
}
```

### 3. StreamingTool Base Implementation

The `StreamingTool` provides a robust foundation for async tool execution:

- **Concurrency Control**: Configurable limits on concurrent executions
- **Progress Tracking**: Real-time progress updates via channels
- **Cancellation Support**: Context-based cancellation with cleanup
- **Error Handling**: Proper error propagation and recovery
- **Resource Management**: Automatic cleanup of channels and goroutines

## Key Features

### 1. Real-time Progress Updates

Tools can send detailed progress updates during execution:

```go
progressChan <- &ToolProgress{
    ID:       toolID,
    Progress: 0.75,
    Message:  "Processing stage 3 of 4",
    Metadata: map[string]any{
        "stage": 3,
        "total": 4,
        "bytes_processed": 1024000,
    },
    Timestamp:  time.Now(),
    Cancelable: true,
}
```

### 2. Cancellation Support

Tools respect context cancellation:

```go
select {
case <-ctx.Done():
    return nil, ctx.Err()
case <-time.After(processingTime):
    // Continue processing
}
```

### 3. Concurrency Control

Built-in limits prevent resource exhaustion:

```go
tool := NewStreamingTool("processor", "File processor", 5) // Max 5 concurrent
```

### 4. Dual Execution Modes

Tools support both sync and async execution:

```go
// Synchronous - blocks until completion
result, err := tool.RunAsync(ctx, args, toolCtx)

// Asynchronous - returns stream immediately
stream, err := tool.RunStream(ctx, args, toolCtx)
```

## Example Implementations

### 1. File Processor Tool

Demonstrates long-running operations with detailed progress:

```go
tool := async.NewFileProcessorTool()
stream, _ := tool.RunStream(ctx, map[string]any{
    "file_path": "/large/file.dat",
    "operation": "analyze",
}, toolCtx)

// Monitor progress
go func() {
    for progress := range stream.Progress {
        fmt.Printf("Progress: %.1f%% - %s\n", 
            progress.Progress*100, progress.Message)
    }
}()

// Get final result
result := <-stream.Result
```

### 2. Web Scraper Tool

Shows concurrent page processing:

```go
tool := async.NewWebScraperTool()
stream, _ := tool.RunStream(ctx, map[string]any{
    "url": "https://example.com",
    "max_pages": 10,
    "selectors": []string{".title", ".content"},
}, toolCtx)
```

## Go-Specific Patterns

### 1. Channel-based Communication

Instead of Python's AsyncGenerator, we use Go channels:

```go
// Python: async def progress_generator():
//             yield progress_update

// Go:
progressChan := make(chan *ToolProgress, 10)
go func() {
    defer close(progressChan)
    for progress := range updates {
        progressChan <- progress
    }
}()
return progressChan
```

### 2. Context Propagation

Context carries cancellation and timeouts throughout the execution:

```go
func (t *Tool) RunStream(ctx context.Context, args map[string]any, toolCtx *ToolContext) (*ToolStream, error) {
    toolCtx_local, cancel := context.WithCancel(ctx)
    
    go func() {
        defer cancel()
        // Tool execution with context checking
        select {
        case <-toolCtx_local.Done():
            return // Cancelled
        case result := <-processingDone:
            // Continue
        }
    }()
}
```

### 3. Goroutine Management

Proper cleanup and resource management:

```go
go func() {
    defer func() {
        close(progressChan)
        close(resultChan)
        // Remove from active tools tracking
        t.mu.Lock()
        delete(t.activeTools, toolID)
        t.mu.Unlock()
    }()
    
    // Tool execution
}()
```

## Usage Patterns

### 1. Fire-and-Forget

```go
stream, _ := tool.RunStream(ctx, args, toolCtx)
// Don't wait for completion, just track ID
```

### 2. Progress Monitoring

```go
stream, _ := tool.RunStream(ctx, args, toolCtx)
for progress := range stream.Progress {
    updateUI(progress)
}
```

### 3. Result Waiting

```go
stream, _ := tool.RunStream(ctx, args, toolCtx)
select {
case result := <-stream.Result:
    return result.Result, result.Error
case <-time.After(timeout):
    stream.Cancel()
    return nil, ErrTimeout
}
```

### 4. Cancellation

```go
stream, _ := tool.RunStream(ctx, args, toolCtx)
// Later...
stream.Cancel() // Immediately cancels execution
```

## Integration with ADK Framework

The async tool system integrates seamlessly with the broader ADK framework:

- **LLM Integration**: Tools can modify LLM requests and provide function declarations
- **Session State**: Progress and results can update session state
- **Event System**: Tool execution generates events for the agent workflow
- **Error Handling**: Consistent error patterns throughout the framework

## Performance Considerations

1. **Memory Usage**: Channels have configurable buffers to prevent blocking
2. **Goroutine Lifecycle**: Proper cleanup prevents goroutine leaks
3. **Concurrency Limits**: Prevents resource exhaustion under load
4. **Context Timeout**: Automatic cleanup for stuck operations

## Testing

Comprehensive test suite covers:

- Basic execution and result types
- Progress update sequences
- Cancellation behavior
- Timeout handling
- Concurrency limits
- Resource cleanup

The async tool system provides a robust, Go-native foundation for building sophisticated AI agent tools with real-time feedback and proper resource management.
