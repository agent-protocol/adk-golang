package runners

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

// MockAgent implements core.BaseAgent for testing.
type MockAgent struct {
	name           string
	description    string
	instruction    string
	events         []*core.Event
	shouldError    bool
	delay          time.Duration
	beforeCallback core.BeforeAgentCallback
	afterCallback  core.AfterAgentCallback
}

func (m *MockAgent) Name() string                         { return m.name }
func (m *MockAgent) Description() string                  { return m.description }
func (m *MockAgent) Instruction() string                  { return m.instruction }
func (m *MockAgent) SubAgents() []core.BaseAgent          { return nil }
func (m *MockAgent) ParentAgent() core.BaseAgent          { return nil }
func (m *MockAgent) SetParentAgent(parent core.BaseAgent) {}

func (m *MockAgent) RunAsync(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock agent error")
	}

	eventChan := make(chan *core.Event, len(m.events)+1)

	go func() {
		defer close(eventChan)

		// Add delay if specified
		if m.delay > 0 {
			time.Sleep(m.delay)
		}

		// Send all mock events
		for _, event := range m.events {
			select {
			case eventChan <- event:
			case <-invocationCtx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

func (m *MockAgent) Run(invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	stream, err := m.RunAsync(invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range stream {
		events = append(events, event)
	}

	return events, nil
}

func (m *MockAgent) FindAgent(name string) core.BaseAgent {
	if m.name == name {
		return m
	}
	return nil
}

func (m *MockAgent) FindSubAgent(name string) core.BaseAgent { return nil }

func (m *MockAgent) GetBeforeAgentCallback() core.BeforeAgentCallback {
	return m.beforeCallback
}

func (m *MockAgent) SetBeforeAgentCallback(callback core.BeforeAgentCallback) {
	m.beforeCallback = callback
}

func (m *MockAgent) GetAfterAgentCallback() core.AfterAgentCallback {
	return m.afterCallback
}

func (m *MockAgent) SetAfterAgentCallback(callback core.AfterAgentCallback) {
	m.afterCallback = callback
}

func (m *MockAgent) Cleanup(ctx context.Context) error { return nil }

func TestRunnerBasicExecution(t *testing.T) {
	// Create mock agent with test events
	mockEvents := []*core.Event{
		{
			ID:           "event1",
			InvocationID: "inv1",
			Author:       "test-agent",
			Content: &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{Type: "text", Text: ptr.Ptr("Hello from agent")},
				},
			},
			Timestamp: time.Now(),
		},
		{
			ID:           "event2",
			InvocationID: "inv1",
			Author:       "test-agent",
			Content: &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{Type: "text", Text: ptr.Ptr("Processing complete")},
				},
			},
			Timestamp: time.Now(),
		},
	}

	agent := &MockAgent{
		name:        "test-agent",
		description: "Test agent for runner",
		instruction: "Test instruction",
		events:      mockEvents,
	}

	// Create session service
	sessionService := sessions.NewInMemorySessionService()

	// Create runner
	runner := NewRunner("test-app", agent, sessionService)

	// Create run request
	req := &core.RunRequest{
		UserID:    "test-user",
		SessionID: "test-session",
		NewMessage: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{Type: "text", Text: ptr.Ptr("Hello agent")},
			},
		},
	}

	ctx := context.Background()

	// Test RunAsync
	eventStream, err := runner.RunAsync(ctx, req)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	// Collect events from stream
	var receivedEvents []*core.Event
	for event := range eventStream {
		receivedEvents = append(receivedEvents, event)
	}

	// Verify we received the expected events
	if len(receivedEvents) != len(mockEvents) {
		t.Errorf("Expected %d events, got %d", len(mockEvents), len(receivedEvents))
	}

	// Verify event content
	for i, event := range receivedEvents {
		if event.Author != mockEvents[i].Author {
			t.Errorf("Event %d: expected author %s, got %s", i, mockEvents[i].Author, event.Author)
		}
	}
}

func TestRunnerSynchronousExecution(t *testing.T) {
	mockEvents := []*core.Event{
		{
			ID:           "sync1",
			InvocationID: "inv1",
			Author:       "sync-agent",
			Content: &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{Type: "text", Text: ptr.Ptr("Sync response")},
				},
			},
			Timestamp: time.Now(),
		},
	}

	agent := &MockAgent{
		name:   "sync-agent",
		events: mockEvents,
	}

	sessionService := sessions.NewInMemorySessionService()
	runner := NewRunner("test-app", agent, sessionService)

	req := &core.RunRequest{
		UserID:    "test-user",
		SessionID: "test-session-sync",
		NewMessage: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{Type: "text", Text: ptr.Ptr("Sync request")},
			},
		},
	}

	ctx := context.Background()

	// Test Run (synchronous)
	events, err := runner.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(events) != len(mockEvents) {
		t.Errorf("Expected %d events, got %d", len(mockEvents), len(events))
	}
}

func TestRunnerErrorHandling(t *testing.T) {
	agent := &MockAgent{
		name:        "error-agent",
		shouldError: true,
	}

	sessionService := sessions.NewInMemorySessionService()
	runner := NewRunner("test-app", agent, sessionService)

	req := &core.RunRequest{
		UserID:    "test-user",
		SessionID: "test-session-error",
	}

	ctx := context.Background()

	// Test error handling in RunAsync
	eventStream, err := runner.RunAsync(ctx, req)
	if err != nil {
		t.Fatalf("RunAsync should not fail immediately: %v", err)
	}

	// Should receive error event
	var receivedEvents []*core.Event
	for event := range eventStream {
		receivedEvents = append(receivedEvents, event)
	}

	// Should have received an error event
	if len(receivedEvents) == 0 {
		t.Error("Expected to receive error event")
	} else {
		errorEvent := receivedEvents[0]
		if errorEvent.ErrorMessage == nil {
			t.Error("Expected error event to have error message")
		}
	}
}

func TestRunnerCallbacks(t *testing.T) {
	var beforeCalled, afterCalled bool
	var callbackEvents []*core.Event

	mockEvents := []*core.Event{
		{
			ID:           "cb1",
			InvocationID: "inv1",
			Author:       "callback-agent",
			Content: &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{Type: "text", Text: ptr.Ptr("Callback test")},
				},
			},
			Timestamp: time.Now(),
		},
	}

	agent := &MockAgent{
		name:   "callback-agent",
		events: mockEvents,
	}

	// Set callbacks
	agent.SetBeforeAgentCallback(func(invocationCtx *core.InvocationContext) error {
		beforeCalled = true
		return nil
	})

	agent.SetAfterAgentCallback(func(invocationCtx *core.InvocationContext, events []*core.Event) error {
		afterCalled = true
		callbackEvents = events
		return nil
	})

	sessionService := sessions.NewInMemorySessionService()
	runner := NewRunner("test-app", agent, sessionService)

	req := &core.RunRequest{
		UserID:    "test-user",
		SessionID: "test-session-cb",
	}

	ctx := context.Background()

	// Run and collect all events
	eventStream, err := runner.RunAsync(ctx, req)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	var receivedEvents []*core.Event
	for event := range eventStream {
		receivedEvents = append(receivedEvents, event)
	}

	// Verify callbacks were called
	if !beforeCalled {
		t.Error("Before-agent callback was not called")
	}

	if !afterCalled {
		t.Error("After-agent callback was not called")
	}

	if len(callbackEvents) != len(mockEvents) {
		t.Errorf("After-agent callback received %d events, expected %d", len(callbackEvents), len(mockEvents))
	}
}

func TestRunnerConfiguration(t *testing.T) {
	config := &RunnerConfig{
		EventBufferSize:       50,
		EnableEventProcessing: false,
		MaxConcurrentSessions: 10,
		DefaultTimeout:        15 * time.Second,
	}

	agent := &MockAgent{name: "config-agent"}
	sessionService := sessions.NewInMemorySessionService()

	runner := NewRunnerWithConfig("test-app", agent, sessionService, config)

	// Verify configuration is set
	retrievedConfig := runner.GetConfig()
	if retrievedConfig.EventBufferSize != 50 {
		t.Errorf("Expected EventBufferSize 50, got %d", retrievedConfig.EventBufferSize)
	}

	if retrievedConfig.EnableEventProcessing {
		t.Error("Expected EnableEventProcessing to be false")
	}

	if retrievedConfig.MaxConcurrentSessions != 10 {
		t.Errorf("Expected MaxConcurrentSessions 10, got %d", retrievedConfig.MaxConcurrentSessions)
	}

	if retrievedConfig.DefaultTimeout != 15*time.Second {
		t.Errorf("Expected DefaultTimeout 15s, got %v", retrievedConfig.DefaultTimeout)
	}
}

func TestRunnerConcurrentExecution(t *testing.T) {
	mockEvents := []*core.Event{
		{
			ID:           "concurrent1",
			InvocationID: "inv1",
			Author:       "concurrent-agent",
			Content: &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{Type: "text", Text: ptr.Ptr("Concurrent response")},
				},
			},
			Timestamp: time.Now(),
		},
	}

	agent := &MockAgent{
		name:   "concurrent-agent",
		events: mockEvents,
		delay:  50 * time.Millisecond, // Small delay to test concurrency
	}

	sessionService := sessions.NewInMemorySessionService()
	runner := NewRunner("test-app", agent, sessionService)

	ctx := context.Background()
	const numConcurrent = 5

	var wg sync.WaitGroup
	results := make(chan int, numConcurrent)

	// Run multiple concurrent executions
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := &core.RunRequest{
				UserID:    "test-user",
				SessionID: fmt.Sprintf("concurrent-session-%d", id),
			}

			eventStream, err := runner.RunAsync(ctx, req)
			if err != nil {
				t.Errorf("Concurrent execution %d failed: %v", id, err)
				return
			}

			eventCount := 0
			for range eventStream {
				eventCount++
			}

			results <- eventCount
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all executions completed successfully
	completedCount := 0
	for eventCount := range results {
		if eventCount > 0 {
			completedCount++
		}
	}

	if completedCount != numConcurrent {
		t.Errorf("Expected %d successful concurrent executions, got %d", numConcurrent, completedCount)
	}
}

func TestRunnerStateManagement(t *testing.T) {
	// Create event with state delta
	mockEvents := []*core.Event{
		{
			ID:           "state1",
			InvocationID: "inv1",
			Author:       "state-agent",
			Actions: core.EventActions{
				StateDelta: map[string]any{
					"counter": 42,
					"status":  "active",
				},
			},
			Timestamp: time.Now(),
		},
	}

	agent := &MockAgent{
		name:   "state-agent",
		events: mockEvents,
	}

	sessionService := sessions.NewInMemorySessionService()
	runner := NewRunner("test-app", agent, sessionService)

	req := &core.RunRequest{
		UserID:    "test-user",
		SessionID: "state-session",
	}

	ctx := context.Background()

	// Run and process events
	eventStream, err := runner.RunAsync(ctx, req)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	// Consume all events
	for range eventStream {
		// Events are processed automatically
	}

	// Verify state was applied to session
	getReq := &core.GetSessionRequest{
		AppName:   "test-app",
		UserID:    "test-user",
		SessionID: "state-session",
	}

	session, err := sessionService.GetSession(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session.State["counter"] != 42 {
		t.Errorf("Expected counter=42, got %v", session.State["counter"])
	}

	if session.State["status"] != "active" {
		t.Errorf("Expected status=active, got %v", session.State["status"])
	}
}

func TestDefaultRunnerConfig(t *testing.T) {
	config := DefaultRunnerConfig()

	if config.EventBufferSize != 100 {
		t.Errorf("Expected default EventBufferSize 100, got %d", config.EventBufferSize)
	}

	if !config.EnableEventProcessing {
		t.Error("Expected default EnableEventProcessing to be true")
	}

	if config.MaxConcurrentSessions != 0 {
		t.Errorf("Expected default MaxConcurrentSessions 0 (unlimited), got %d", config.MaxConcurrentSessions)
	}

	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default DefaultTimeout 30s, got %v", config.DefaultTimeout)
	}
}
