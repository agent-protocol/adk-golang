package executor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/a2a/converters"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// A2aAgentExecutorConfig contains configuration for the A2aAgentExecutor.
type A2aAgentExecutorConfig struct {
	// EnableDebugLogging enables detailed logging for debugging
	EnableDebugLogging bool
	// Timeout is the maximum time to wait for agent execution
	Timeout time.Duration
	// MaxConcurrentRequests limits the number of concurrent requests
	MaxConcurrentRequests int
}

// DefaultA2aAgentExecutorConfig returns default configuration for A2aAgentExecutor.
func DefaultA2aAgentExecutorConfig() *A2aAgentExecutorConfig {
	return &A2aAgentExecutorConfig{
		EnableDebugLogging:    false,
		Timeout:               5 * time.Minute,
		MaxConcurrentRequests: 10,
	}
}

// A2aAgentExecutor is responsible for handling A2A requests and converting them
// to ADK agent calls. It acts as a bridge between the A2A protocol and ADK agents.
type A2aAgentExecutor struct {
	runner         core.Runner
	runnerFactory  func() (core.Runner, error)
	config         *A2aAgentExecutorConfig
	resolvedRunner core.Runner
	mutex          sync.RWMutex
}

// NewA2aAgentExecutor creates a new A2aAgentExecutor with a direct runner instance.
func NewA2aAgentExecutor(runner core.Runner, config *A2aAgentExecutorConfig) *A2aAgentExecutor {
	if config == nil {
		config = DefaultA2aAgentExecutorConfig()
	}
	return &A2aAgentExecutor{
		runner: runner,
		config: config,
	}
}

// NewA2aAgentExecutorWithFactory creates a new A2aAgentExecutor with a runner factory function.
// This allows for lazy initialization of the runner.
func NewA2aAgentExecutorWithFactory(runnerFactory func() (core.Runner, error), config *A2aAgentExecutorConfig) *A2aAgentExecutor {
	if config == nil {
		config = DefaultA2aAgentExecutorConfig()
	}
	return &A2aAgentExecutor{
		runnerFactory: runnerFactory,
		config:        config,
	}
}

// RequestContext represents the context of an A2A request.
type RequestContext struct {
	TaskID      string
	ContextID   string
	Message     *a2a.Message
	CurrentTask *a2a.Task
	SessionID   string
	UserID      string
	Metadata    map[string]interface{}
}

// EventQueue interface for publishing A2A events.
type EventQueue interface {
	// EnqueueEvent adds an event to the queue for processing
	EnqueueEvent(ctx context.Context, event interface{}) error
	// Close closes the event queue
	Close() error
}

// SimpleEventQueue is a basic implementation of EventQueue for testing and simple use cases.
type SimpleEventQueue struct {
	events chan interface{}
	closed bool
	mutex  sync.RWMutex
}

// NewSimpleEventQueue creates a new SimpleEventQueue with specified buffer size.
func NewSimpleEventQueue(bufferSize int) *SimpleEventQueue {
	return &SimpleEventQueue{
		events: make(chan interface{}, bufferSize),
	}
}

// EnqueueEvent adds an event to the queue.
func (eq *SimpleEventQueue) EnqueueEvent(ctx context.Context, event interface{}) error {
	eq.mutex.RLock()
	defer eq.mutex.RUnlock()

	if eq.closed {
		return fmt.Errorf("event queue is closed")
	}

	select {
	case eq.events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("event queue is full")
	}
}

// Close closes the event queue.
func (eq *SimpleEventQueue) Close() error {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()

	if !eq.closed {
		close(eq.events)
		eq.closed = true
	}
	return nil
}

// Events returns a channel to receive events from the queue.
func (eq *SimpleEventQueue) Events() <-chan interface{} {
	return eq.events
}

// resolveRunner resolves the runner instance, handling both direct instances and factory functions.
func (executor *A2aAgentExecutor) resolveRunner() (core.Runner, error) {
	executor.mutex.Lock()
	defer executor.mutex.Unlock()

	// Return cached runner if available
	if executor.resolvedRunner != nil {
		return executor.resolvedRunner, nil
	}

	// If we have a direct runner, use it
	if executor.runner != nil {
		executor.resolvedRunner = executor.runner
		return executor.resolvedRunner, nil
	}

	// If we have a factory, call it
	if executor.runnerFactory != nil {
		runner, err := executor.runnerFactory()
		if err != nil {
			return nil, fmt.Errorf("failed to create runner: %w", err)
		}
		executor.resolvedRunner = runner
		return executor.resolvedRunner, nil
	}

	return nil, fmt.Errorf("no runner or runner factory provided")
}

// Execute handles an A2A request and publishes updates to the event queue.
// It converts A2A requests to ADK format, executes the agent, and streams back A2A events.
func (executor *A2aAgentExecutor) Execute(ctx context.Context, requestCtx *RequestContext, eventQueue EventQueue) error {
	if requestCtx.Message == nil {
		return fmt.Errorf("A2A request must have a message")
	}

	// Set timeout context
	if executor.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, executor.config.Timeout)
		defer cancel()
	}

	// Create task submitted event for new tasks
	if requestCtx.CurrentTask == nil {
		submitEvent := &a2a.TaskStatusUpdateEvent{
			ID: requestCtx.TaskID,
			Status: a2a.TaskStatus{
				State:     a2a.TaskStateSubmitted,
				Message:   requestCtx.Message,
				Timestamp: ptr.Ptr(time.Now()),
			},
			Final: false,
		}
		if err := eventQueue.EnqueueEvent(ctx, submitEvent); err != nil {
			return fmt.Errorf("failed to enqueue submitted event: %w", err)
		}
	}

	// Handle the request
	if err := executor.handleRequest(ctx, requestCtx, eventQueue); err != nil {
		// Publish failure event
		failureEvent := &a2a.TaskStatusUpdateEvent{
			ID: requestCtx.TaskID,
			Status: a2a.TaskStatus{
				State:     a2a.TaskStateFailed,
				Message:   createErrorMessage(err.Error()),
				Timestamp: ptr.Ptr(time.Now()),
			},
			Final: true,
		}
		if enqueueErr := eventQueue.EnqueueEvent(ctx, failureEvent); enqueueErr != nil {
			log.Printf("Failed to enqueue failure event: %v", enqueueErr)
		}
		return fmt.Errorf("failed to handle A2A request: %w", err)
	}

	return nil
}

// Cancel handles cancellation of a running task.
func (executor *A2aAgentExecutor) Cancel(ctx context.Context, requestCtx *RequestContext, eventQueue EventQueue) error {
	// TODO: Implement proper cancellation logic
	// This should interact with the runner to cancel the ongoing task
	cancelEvent := &a2a.TaskStatusUpdateEvent{
		ID: requestCtx.TaskID,
		Status: a2a.TaskStatus{
			State:     a2a.TaskStateCanceled,
			Timestamp: ptr.Ptr(time.Now()),
		},
		Final: true,
	}
	return eventQueue.EnqueueEvent(ctx, cancelEvent)
}

// handleRequest processes the A2A request by converting it to ADK format and executing the agent.
func (executor *A2aAgentExecutor) handleRequest(ctx context.Context, requestCtx *RequestContext, eventQueue EventQueue) error {
	// Resolve the runner instance
	runner, err := executor.resolveRunner()
	if err != nil {
		return fmt.Errorf("failed to resolve runner: %w", err)
	}

	// Convert A2A request to ADK run arguments
	runArgs, err := converters.ConvertA2ARequestToADKRunArgs(&converters.RequestContext{
		TaskID:      requestCtx.TaskID,
		ContextID:   requestCtx.ContextID,
		Message:     requestCtx.Message,
		CurrentTask: requestCtx.CurrentTask,
		SessionID:   requestCtx.SessionID,
		UserID:      requestCtx.UserID,
		Metadata:    requestCtx.Metadata,
	})
	if err != nil {
		return fmt.Errorf("failed to convert A2A request to ADK run args: %w", err)
	}

	// Ensure session exists
	session, err := executor.prepareSession(ctx, requestCtx, runArgs, runner)
	if err != nil {
		return fmt.Errorf("failed to prepare session: %w", err)
	}

	// Publish working event
	workingEvent := &a2a.TaskStatusUpdateEvent{
		ID: requestCtx.TaskID,
		Status: a2a.TaskStatus{
			State:     a2a.TaskStateWorking,
			Timestamp: ptr.Ptr(time.Now()),
		},
		Final:    false,
		Metadata: createExecutionMetadata(runner, runArgs),
	}
	if err := eventQueue.EnqueueEvent(ctx, workingEvent); err != nil {
		return fmt.Errorf("failed to enqueue working event: %w", err)
	}

	// Execute the agent and stream events
	runRequest := &core.RunRequest{
		UserID:     runArgs.UserID,
		SessionID:  runArgs.SessionID,
		NewMessage: runArgs.NewMessage,
		RunConfig:  runArgs.RunConfig,
	}
	eventStream, err := runner.RunAsync(ctx, runRequest)
	if err != nil {
		return fmt.Errorf("failed to start agent execution: %w", err)
	}

	// Process events from the agent and convert to A2A events
	taskCompleted := false
	for adkEvent := range eventStream {
		// Convert ADK event to A2A events
		a2aEvents, err := converters.ConvertEventToA2AEvents(adkEvent, session, requestCtx.TaskID, requestCtx.ContextID)
		if err != nil {
			log.Printf("Failed to convert ADK event to A2A events: %v", err)
			continue
		}

		// Enqueue all converted events
		for _, a2aEvent := range a2aEvents {
			if err := eventQueue.EnqueueEvent(ctx, a2aEvent); err != nil {
				log.Printf("Failed to enqueue A2A event: %v", err)
			}
		}

		// Check if task should be marked as completed
		if adkEvent.Actions.Escalate != nil && *adkEvent.Actions.Escalate {
			taskCompleted = true
		}
	}

	// Publish final completion event if not already completed
	if !taskCompleted {
		completionEvent := &a2a.TaskStatusUpdateEvent{
			ID: requestCtx.TaskID,
			Status: a2a.TaskStatus{
				State:     a2a.TaskStateCompleted,
				Timestamp: ptr.Ptr(time.Now()),
			},
			Final: true,
		}
		if err := eventQueue.EnqueueEvent(ctx, completionEvent); err != nil {
			return fmt.Errorf("failed to enqueue completion event: %w", err)
		}
	}

	return nil
}

// prepareSession ensures that a session exists for the request.
// For now, this is a simplified implementation that creates a basic session.
func (executor *A2aAgentExecutor) prepareSession(ctx context.Context, requestCtx *RequestContext, runArgs *converters.ADKRunArgs, runner core.Runner) (*core.Session, error) {
	// Create a basic session for the request
	// In a full implementation, this would use the runner's session service
	session := &core.Session{
		ID:      runArgs.SessionID,
		UserID:  runArgs.UserID,
		AppName: "a2a-agent", // Default app name
		State:   make(map[string]interface{}),
		Events:  make([]*core.Event, 0),
	}

	return session, nil
}

func createErrorMessage(errorText string) *a2a.Message {
	return &a2a.Message{
		Role: "agent",
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: &errorText,
			},
		},
	}
}

func createExecutionMetadata(runner core.Runner, runArgs *converters.ADKRunArgs) map[string]interface{} {
	return map[string]interface{}{
		"adk:app_name":   "a2a-agent", // Default app name
		"adk:user_id":    runArgs.UserID,
		"adk:session_id": runArgs.SessionID,
	}
}
