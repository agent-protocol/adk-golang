// Package async provides advanced async tool execution patterns for the ADK framework.
package async

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// ToolResult represents the result of an async tool execution.
type ToolResult struct {
	Result any    `json:"result,omitempty"`
	Error  error  `json:"error,omitempty"`
	Done   bool   `json:"done"`
	ID     string `json:"id"`
}

// ToolProgress represents progress updates from a long-running tool.
type ToolProgress struct {
	ID         string         `json:"id"`
	Progress   float64        `json:"progress"` // 0.0 to 1.0
	Message    string         `json:"message"`  // Human-readable status
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	Cancelable bool           `json:"cancelable"`
}

// ToolStream represents a stream of progress updates and final result.
type ToolStream struct {
	Progress <-chan *ToolProgress `json:"-"`
	Result   <-chan *ToolResult   `json:"-"`
	Cancel   context.CancelFunc   `json:"-"`
}

// AsyncTool extends BaseTool with advanced async capabilities.
type AsyncTool interface {
	core.BaseTool

	// RunStream executes the tool and returns a stream of progress updates and final result.
	// This is the preferred method for long-running operations.
	RunStream(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (*ToolStream, error)

	// CanCancel indicates if the tool supports cancellation during execution.
	CanCancel() bool

	// Cancel cancels a running tool execution by ID.
	Cancel(ctx context.Context, toolID string) error

	// GetStatus returns the current status of a running tool execution.
	GetStatus(ctx context.Context, toolID string) (*ToolProgress, error)
}

// StreamingTool provides a base implementation for streaming tools.
type StreamingTool struct {
	*BaseToolImpl
	maxConcurrency int
	activeTools    map[string]context.CancelFunc
	mu             sync.RWMutex
	executeFunc    func(context.Context, map[string]any, *core.ToolContext, chan<- *ToolProgress, string) (any, error)
}

// BaseToolImpl wraps the original BaseToolImpl to avoid import cycles.
type BaseToolImpl struct {
	name          string
	description   string
	isLongRunning bool
}

// NewStreamingTool creates a new streaming tool with the specified concurrency limit.
func NewStreamingTool(name, description string, maxConcurrency int) *StreamingTool {
	return &StreamingTool{
		BaseToolImpl: &BaseToolImpl{
			name:          name,
			description:   description,
			isLongRunning: true,
		},
		maxConcurrency: maxConcurrency,
		activeTools:    make(map[string]context.CancelFunc),
		executeFunc:    nil, // Will use default implementation
	}
}

// SetExecuteFunc allows setting a custom execution function.
func (t *StreamingTool) SetExecuteFunc(fn func(context.Context, map[string]any, *core.ToolContext, chan<- *ToolProgress, string) (any, error)) {
	t.executeFunc = fn
}

// Name returns the tool's unique identifier.
func (t *BaseToolImpl) Name() string {
	return t.name
}

// Description returns a description of the tool's purpose.
func (t *BaseToolImpl) Description() string {
	return t.description
}

// IsLongRunning indicates if this is a long-running operation.
func (t *BaseToolImpl) IsLongRunning() bool {
	return t.isLongRunning
}

// GetDeclaration returns the function declaration for LLM integration.
func (t *BaseToolImpl) GetDeclaration() *core.FunctionDeclaration {
	return nil
}

// RunAsync implements BaseTool interface by calling RunStream and waiting for result.
func (t *StreamingTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	stream, err := t.RunStream(ctx, args, toolCtx)
	if err != nil {
		return nil, err
	}

	// Wait for the final result
	select {
	case result := <-stream.Result:
		if result.Error != nil {
			return nil, result.Error
		}
		return result.Result, nil
	case <-ctx.Done():
		stream.Cancel()
		return nil, ctx.Err()
	}
}

// RunStream executes the tool and returns a stream of progress updates.
// This is a base implementation that subclasses should override.
func (t *StreamingTool) RunStream(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (*ToolStream, error) {
	toolID := generateToolID()

	// Check concurrency limits
	t.mu.Lock()
	if len(t.activeTools) >= t.maxConcurrency {
		t.mu.Unlock()
		return nil, ErrTooManyActiveTasks
	}
	t.mu.Unlock()

	// Create channels for communication
	progressChan := make(chan *ToolProgress, 10)
	resultChan := make(chan *ToolResult, 1)

	// Create cancellable context
	toolCtx_local, cancel := context.WithCancel(ctx)

	// Track active tool
	t.mu.Lock()
	t.activeTools[toolID] = cancel
	t.mu.Unlock()

	// Start tool execution in goroutine
	go func() {
		defer func() {
			close(progressChan)
			close(resultChan)
			// Remove from active tools
			t.mu.Lock()
			delete(t.activeTools, toolID)
			t.mu.Unlock()
		}()

		// Send initial progress
		progressChan <- &ToolProgress{
			ID:         toolID,
			Progress:   0.0,
			Message:    "Starting tool execution",
			Timestamp:  time.Now(),
			Cancelable: true,
		}

		// Execute using custom function or default implementation
		var result any
		var err error
		if t.executeFunc != nil {
			result, err = t.executeFunc(toolCtx_local, args, toolCtx, progressChan, toolID)
		} else {
			result, err = t.executeInternal(toolCtx_local, args, toolCtx, progressChan, toolID)
		}

		// Send final result
		resultChan <- &ToolResult{
			ID:     toolID,
			Result: result,
			Error:  err,
			Done:   true,
		}
	}()

	return &ToolStream{
		Progress: progressChan,
		Result:   resultChan,
		Cancel:   cancel,
	}, nil
}

// executeInternal is meant to be overridden by concrete implementations.
func (t *StreamingTool) executeInternal(ctx context.Context, args map[string]any, toolCtx *core.ToolContext, progressChan chan<- *ToolProgress, toolID string) (any, error) {
	// Default implementation just sleeps and sends progress updates
	for i := 1; i <= 5; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
			progressChan <- &ToolProgress{
				ID:         toolID,
				Progress:   float64(i) / 5.0,
				Message:    "Processing...",
				Timestamp:  time.Now(),
				Cancelable: true,
			}
		}
	}
	return "Task completed", nil
}

// CanCancel indicates if the tool supports cancellation.
func (t *StreamingTool) CanCancel() bool {
	return true
}

// Cancel cancels a running tool execution by ID.
func (t *StreamingTool) Cancel(ctx context.Context, toolID string) error {
	t.mu.RLock()
	cancelFunc, exists := t.activeTools[toolID]
	t.mu.RUnlock()

	if !exists {
		return ErrToolNotFound
	}

	cancelFunc()
	return nil
}

// GetStatus returns the current status of a running tool execution.
func (t *StreamingTool) GetStatus(ctx context.Context, toolID string) (*ToolProgress, error) {
	t.mu.RLock()
	_, exists := t.activeTools[toolID]
	t.mu.RUnlock()

	if !exists {
		return nil, ErrToolNotFound
	}

	// Return current status - in a real implementation, this would
	// maintain status state for each running tool
	return &ToolProgress{
		ID:         toolID,
		Progress:   0.5, // Placeholder
		Message:    "Running",
		Timestamp:  time.Now(),
		Cancelable: true,
	}, nil
}

// ProcessLLMRequest allows the tool to modify LLM requests.
func (t *StreamingTool) ProcessLLMRequest(ctx context.Context, toolCtx *core.ToolContext, request *core.LLMRequest) error {
	// Default implementation does nothing
	return nil
}

// Utility functions and errors

var (
	ErrTooManyActiveTasks = fmt.Errorf("too many active tool executions")
	ErrToolNotFound       = fmt.Errorf("tool execution not found")
)

func generateToolID() string {
	return "tool_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
