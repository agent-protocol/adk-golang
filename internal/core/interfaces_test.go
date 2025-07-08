package core

import (
	"context"
	"testing"
	"time"
)

// Mock implementations for testing

type mockAgent struct {
	name        string
	description string
	instruction string
	subAgents   []BaseAgent
	parent      BaseAgent
}

func (m *mockAgent) Name() string                       { return m.name }
func (m *mockAgent) Description() string                { return m.description }
func (m *mockAgent) Instruction() string                { return m.instruction }
func (m *mockAgent) SubAgents() []BaseAgent             { return m.subAgents }
func (m *mockAgent) ParentAgent() BaseAgent             { return m.parent }
func (m *mockAgent) SetParentAgent(parent BaseAgent)    { m.parent = parent }
func (m *mockAgent) FindAgent(name string) BaseAgent    { return nil }
func (m *mockAgent) FindSubAgent(name string) BaseAgent { return nil }
func (m *mockAgent) Cleanup(ctx context.Context) error  { return nil }

func (m *mockAgent) RunAsync(ctx context.Context, invocationCtx *InvocationContext) (EventStream, error) {
	eventChan := make(chan *Event, 1)
	go func() {
		defer close(eventChan)
		event := &Event{
			ID:           "test-event",
			InvocationID: invocationCtx.InvocationID,
			Author:       m.name,
			Content: &Content{
				Role: "assistant",
				Parts: []Part{
					{Type: "text", Text: stringPtr("Hello from mock agent")},
				},
			},
			Actions:   EventActions{},
			Timestamp: time.Now(),
		}
		eventChan <- event
	}()
	return eventChan, nil
}

type mockTool struct {
	name        string
	description string
	longRunning bool
	declaration *FunctionDeclaration
}

func (m *mockTool) Name() string                         { return m.name }
func (m *mockTool) Description() string                  { return m.description }
func (m *mockTool) IsLongRunning() bool                  { return m.longRunning }
func (m *mockTool) GetDeclaration() *FunctionDeclaration { return m.declaration }

func (m *mockTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *ToolContext) (any, error) {
	return "mock result", nil
}

func (m *mockTool) ProcessLLMRequest(ctx context.Context, toolCtx *ToolContext, request *LLMRequest) error {
	return nil
}

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func TestBaseAgentInterface(t *testing.T) {
	agent := &mockAgent{
		name:        "test-agent",
		description: "A test agent",
		instruction: "Test instruction",
	}

	// Test basic properties
	if agent.Name() != "test-agent" {
		t.Errorf("Expected name 'test-agent', got %s", agent.Name())
	}

	if agent.Description() != "A test agent" {
		t.Errorf("Expected description 'A test agent', got %s", agent.Description())
	}

	// Test agent hierarchy
	subAgent := &mockAgent{name: "sub-agent"}
	agent.subAgents = []BaseAgent{subAgent}
	subAgent.SetParentAgent(agent)

	if len(agent.SubAgents()) != 1 {
		t.Errorf("Expected 1 sub-agent, got %d", len(agent.SubAgents()))
	}

	if subAgent.ParentAgent() != agent {
		t.Error("Sub-agent parent should be set to main agent")
	}
}

func TestBaseToolInterface(t *testing.T) {
	tool := &mockTool{
		name:        "test-tool",
		description: "A test tool",
		longRunning: false,
		declaration: &FunctionDeclaration{
			Name:        "test-tool",
			Description: "A test tool",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"input": map[string]any{
						"type":        "string",
						"description": "Test input",
					},
				},
				"required": []string{"input"},
			},
		},
	}

	// Test basic properties
	if tool.Name() != "test-tool" {
		t.Errorf("Expected name 'test-tool', got %s", tool.Name())
	}

	if tool.IsLongRunning() != false {
		t.Error("Expected tool to not be long-running")
	}

	// Test function declaration
	decl := tool.GetDeclaration()
	if decl == nil {
		t.Fatal("Expected function declaration, got nil")
	}

	if decl.Name != "test-tool" {
		t.Errorf("Expected declaration name 'test-tool', got %s", decl.Name)
	}

	// Test tool execution
	ctx := context.Background()
	toolCtx := &ToolContext{}
	args := map[string]any{"input": "test"}

	result, err := tool.RunAsync(ctx, args, toolCtx)
	if err != nil {
		t.Errorf("Tool execution failed: %v", err)
	}

	if result != "mock result" {
		t.Errorf("Expected 'mock result', got %v", result)
	}
}

func TestEventStructure(t *testing.T) {
	event := &Event{
		ID:           "event-123",
		InvocationID: "invocation-456",
		Author:       "test-agent",
		Content: &Content{
			Role: "assistant",
			Parts: []Part{
				{Type: "text", Text: stringPtr("Test message")},
			},
		},
		Actions: EventActions{
			StateDelta: map[string]any{
				"key": "value",
			},
		},
		Timestamp: time.Now(),
	}

	// Test event properties
	if event.ID != "event-123" {
		t.Errorf("Expected ID 'event-123', got %s", event.ID)
	}

	if event.Author != "test-agent" {
		t.Errorf("Expected author 'test-agent', got %s", event.Author)
	}

	// Test content structure
	if event.Content.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %s", event.Content.Role)
	}

	if len(event.Content.Parts) != 1 {
		t.Errorf("Expected 1 content part, got %d", len(event.Content.Parts))
	}

	part := event.Content.Parts[0]
	if part.Type != "text" {
		t.Errorf("Expected part type 'text', got %s", part.Type)
	}

	if *part.Text != "Test message" {
		t.Errorf("Expected text 'Test message', got %s", *part.Text)
	}

	// Test actions
	if len(event.Actions.StateDelta) != 1 {
		t.Errorf("Expected 1 state delta entry, got %d", len(event.Actions.StateDelta))
	}

	if event.Actions.StateDelta["key"] != "value" {
		t.Errorf("Expected state delta value 'value', got %v", event.Actions.StateDelta["key"])
	}
}

func TestStateManagement(t *testing.T) {
	state := NewState()

	// Test setting and getting values
	state.Set("test_key", "test_value")
	value, exists := state.Get("test_key")
	if !exists {
		t.Error("Expected key to exist after setting")
	}
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}

	// Test scoped keys
	state.Set("app:config", "app_value")
	state.Set("user:preference", "user_value")
	state.Set("temp:cache", "temp_value")

	appValue, _ := state.Get("app:config")
	if appValue != "app_value" {
		t.Errorf("Expected 'app_value', got %v", appValue)
	}

	// Test applying deltas
	delta := map[string]any{
		"new_key":  "new_value",
		"test_key": "updated_value",
	}

	state.Update(delta)

	// Check updates
	if value, _ := state.Get("new_key"); value != "new_value" {
		t.Errorf("Expected 'new_value', got %v", value)
	}

	if value, _ := state.Get("test_key"); value != "updated_value" {
		t.Errorf("Expected 'updated_value', got %v", value)
	}
}

func TestInvocationContext(t *testing.T) {
	session := &Session{
		ID:      "session-123",
		AppName: "test-app",
		UserID:  "user-456",
		State:   make(map[string]any),
	}

	agent := &mockAgent{name: "test-agent"}

	ctx := &InvocationContext{
		InvocationID: "invocation-789",
		Agent:        agent,
		Session:      session,
		UserContent: &Content{
			Role: "user",
			Parts: []Part{
				{Type: "text", Text: stringPtr("Hello")},
			},
		},
		RunConfig: &RunConfig{
			MaxTurns: intPtr(10),
		},
	}

	// Test context properties
	if ctx.InvocationID != "invocation-789" {
		t.Errorf("Expected invocation ID 'invocation-789', got %s", ctx.InvocationID)
	}

	if ctx.Agent.Name() != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got %s", ctx.Agent.Name())
	}

	if ctx.Session.ID != "session-123" {
		t.Errorf("Expected session ID 'session-123', got %s", ctx.Session.ID)
	}

	// Test user content
	if ctx.UserContent.Role != "user" {
		t.Errorf("Expected user role, got %s", ctx.UserContent.Role)
	}
}

func TestEventStream(t *testing.T) {
	ctx := context.Background()
	agent := &mockAgent{name: "test-agent"}

	invocationCtx := &InvocationContext{
		InvocationID: "test-invocation",
		Agent:        agent,
		Session:      &Session{ID: "test-session"},
	}

	// Test event streaming
	eventStream, err := agent.RunAsync(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	// Collect events from stream
	var events []*Event
	for event := range eventStream {
		events = append(events, event)
	}

	// Verify we got the expected event
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Author != "test-agent" {
		t.Errorf("Expected author 'test-agent', got %s", event.Author)
	}

	if event.InvocationID != "test-invocation" {
		t.Errorf("Expected invocation ID 'test-invocation', got %s", event.InvocationID)
	}
}
