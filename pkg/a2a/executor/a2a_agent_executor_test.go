package executor

import (
	"context"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/core"
)

// MockRunner implements core.Runner for testing
type MockRunner struct {
	events []*core.Event
}

func (m *MockRunner) RunAsync(ctx context.Context, req *core.RunRequest) (core.EventStream, error) {
	eventChan := make(chan *core.Event, len(m.events))

	go func() {
		defer close(eventChan)
		for _, event := range m.events {
			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

func (m *MockRunner) Run(ctx context.Context, req *core.RunRequest) ([]*core.Event, error) {
	eventStream, err := m.RunAsync(ctx, req)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
	}

	return events, nil
}

func (m *MockRunner) Close(ctx context.Context) error {
	return nil
}

func TestA2aAgentExecutor_Execute(t *testing.T) {
	// Create a mock runner with some test events
	mockRunner := &MockRunner{
		events: []*core.Event{
			{
				ID:           "event1",
				InvocationID: "inv1",
				Author:       "test-agent",
				Content: &core.Content{
					Role: "agent",
					Parts: []core.Part{
						{
							Type: "text",
							Text: stringPtr("Hello from test agent"),
						},
					},
				},
				Timestamp: time.Now(),
				Actions:   core.EventActions{},
			},
		},
	}

	// Create executor
	executor := NewA2aAgentExecutor(mockRunner, nil)

	// Create test request context
	requestCtx := &RequestContext{
		TaskID:    "task-123",
		ContextID: "ctx-456",
		Message: &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr("Hello, agent!"),
				},
			},
		},
		UserID: "user-123",
	}

	// Create test event queue
	eventQueue := NewSimpleEventQueue(10)
	defer eventQueue.Close()

	// Execute the request
	ctx := context.Background()
	err := executor.Execute(ctx, requestCtx, eventQueue)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify events were queued
	select {
	case event := <-eventQueue.Events():
		if statusEvent, ok := event.(*a2a.TaskStatusUpdateEvent); ok {
			if statusEvent.Status.State != a2a.TaskStateSubmitted {
				t.Errorf("Expected first event to have state %s, got %s", a2a.TaskStateSubmitted, statusEvent.Status.State)
			}
		} else {
			t.Errorf("Expected first event to be TaskStatusUpdateEvent, got %T", event)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for first event")
	}
}

func TestA2aAgentExecutor_ExecuteWithError(t *testing.T) {
	// Create executor with nil runner to force error
	executor := NewA2aAgentExecutor(nil, nil)

	// Create test request context
	requestCtx := &RequestContext{
		TaskID:    "task-123",
		ContextID: "ctx-456",
		Message: &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr("Hello, agent!"),
				},
			},
		},
		UserID: "user-123",
	}

	// Create test event queue
	eventQueue := NewSimpleEventQueue(10)
	defer eventQueue.Close()

	// Execute the request
	ctx := context.Background()
	err := executor.Execute(ctx, requestCtx, eventQueue)

	if err == nil {
		t.Error("Expected Execute to fail with nil runner")
	}
}

func TestA2aAgentExecutor_Cancel(t *testing.T) {
	// Create a mock runner
	mockRunner := &MockRunner{}

	// Create executor
	executor := NewA2aAgentExecutor(mockRunner, nil)

	// Create test request context
	requestCtx := &RequestContext{
		TaskID:    "task-123",
		ContextID: "ctx-456",
		UserID:    "user-123",
	}

	// Create test event queue
	eventQueue := NewSimpleEventQueue(10)
	defer eventQueue.Close()

	// Cancel the request
	ctx := context.Background()
	err := executor.Cancel(ctx, requestCtx, eventQueue)

	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	// Verify cancel event was queued
	select {
	case event := <-eventQueue.Events():
		if statusEvent, ok := event.(*a2a.TaskStatusUpdateEvent); ok {
			if statusEvent.Status.State != a2a.TaskStateCanceled {
				t.Errorf("Expected cancel event to have state %s, got %s", a2a.TaskStateCanceled, statusEvent.Status.State)
			}
		} else {
			t.Errorf("Expected cancel event to be TaskStatusUpdateEvent, got %T", event)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for cancel event")
	}
}

func TestSimpleEventQueue(t *testing.T) {
	queue := NewSimpleEventQueue(2)
	defer queue.Close()

	ctx := context.Background()

	// Test enqueue
	err := queue.EnqueueEvent(ctx, "event1")
	if err != nil {
		t.Fatalf("Failed to enqueue event: %v", err)
	}

	err = queue.EnqueueEvent(ctx, "event2")
	if err != nil {
		t.Fatalf("Failed to enqueue second event: %v", err)
	}

	// Test that queue is full
	err = queue.EnqueueEvent(ctx, "event3")
	if err == nil {
		t.Error("Expected error when queue is full")
	}

	// Test receive
	select {
	case event := <-queue.Events():
		if event != "event1" {
			t.Errorf("Expected 'event1', got %v", event)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event")
	}

	// Test close
	err = queue.Close()
	if err != nil {
		t.Fatalf("Failed to close queue: %v", err)
	}

	// Test enqueue after close
	err = queue.EnqueueEvent(ctx, "event4")
	if err == nil {
		t.Error("Expected error when enqueueing to closed queue")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
