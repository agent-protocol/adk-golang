package agents

import (
	"context"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// MockLLMConnection is a mock implementation for testing.
type MockLLMConnection struct {
	responses []*core.LLMResponse
	callCount int
}

func NewMockLLMConnection(responses ...*core.LLMResponse) *MockLLMConnection {
	return &MockLLMConnection{
		responses: responses,
	}
}

func (m *MockLLMConnection) GenerateContent(ctx context.Context, request *core.LLMRequest) (*core.LLMResponse, error) {
	if m.callCount >= len(m.responses) {
		return &core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr("Default response"),
					},
				},
			},
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++
	return response, nil
}

func (m *MockLLMConnection) GenerateContentStream(ctx context.Context, request *core.LLMRequest) (<-chan *core.LLMResponse, error) {
	stream := make(chan *core.LLMResponse, len(m.responses))

	go func() {
		defer close(stream)
		for _, response := range m.responses {
			select {
			case stream <- response:
			case <-ctx.Done():
				return
			}
		}
	}()

	return stream, nil
}

func (m *MockLLMConnection) Close(ctx context.Context) error {
	return nil
}

// MockTool is a simple tool for testing.
type MockTool struct {
	name      string
	response  interface{}
	callCount int
}

func NewMockTool(name string, response interface{}) *MockTool {
	return &MockTool{
		name:     name,
		response: response,
	}
}

func (t *MockTool) Name() string {
	return t.name
}

func (t *MockTool) Description() string {
	return "Mock tool for testing"
}

func (t *MockTool) IsLongRunning() bool {
	return false
}

func (t *MockTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        t.name,
		Description: "Mock tool for testing",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input parameter",
				},
			},
			"required": []string{"input"},
		},
	}
}

func (t *MockTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	t.callCount++
	return t.response, nil
}

func (t *MockTool) ProcessLLMRequest(ctx context.Context, toolCtx *core.ToolContext, request *core.LLMRequest) error {
	return nil
}

func TestEnhancedLlmAgent_BasicFunctionality(t *testing.T) {
	// Create test configuration
	config := &LlmAgentConfig{
		Model:           "test-model",
		Temperature:     llmFloatPtr(0.7),
		MaxTokens:       llmIntPtr(1000),
		MaxToolCalls:    5,
		ToolCallTimeout: 10 * time.Second,
		RetryAttempts:   1,
	}

	// Create agent
	agent := NewEnhancedLlmAgent("test-agent", "A test agent", config)

	// Verify configuration
	if agent.Model() != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", agent.Model())
	}

	if agent.Config().MaxToolCalls != 5 {
		t.Errorf("Expected MaxToolCalls 5, got %d", agent.Config().MaxToolCalls)
	}
}

func TestEnhancedLlmAgent_ToolManagement(t *testing.T) {
	agent := NewEnhancedLlmAgent("test-agent", "A test agent", nil)

	// Test adding tools
	tool1 := NewMockTool("tool1", "response1")
	tool2 := NewMockTool("tool2", "response2")

	agent.AddTool(tool1)
	agent.AddTool(tool2)

	// Verify tools were added
	tools := agent.Tools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// Test getting tool by name
	retrievedTool, exists := agent.GetTool("tool1")
	if !exists {
		t.Error("Expected tool1 to exist")
	}
	if retrievedTool.Name() != "tool1" {
		t.Errorf("Expected tool name 'tool1', got '%s'", retrievedTool.Name())
	}

	// Test removing tool
	removed := agent.RemoveTool("tool1")
	if !removed {
		t.Error("Expected tool1 to be removed")
	}

	tools = agent.Tools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after removal, got %d", len(tools))
	}

	// Test getting non-existent tool
	_, exists = agent.GetTool("tool1")
	if exists {
		t.Error("Expected tool1 to not exist after removal")
	}
}

func TestEnhancedLlmAgent_SimpleConversation(t *testing.T) {
	// Create mock LLM connection with a simple response
	mockLLM := NewMockLLMConnection(
		&core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr("Hello! How can I help you today?"),
					},
				},
			},
		},
	)

	// Create agent and set LLM connection
	agent := NewEnhancedLlmAgent("test-agent", "A helpful assistant", nil)
	agent.SetLLMConnection(mockLLM)

	// Create test session and context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("Hello"),
			},
		},
	}

	// Run the agent
	ctx := context.Background()
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify events
	if len(events) == 0 {
		t.Error("Expected at least one event")
	}

	// Check the response content
	lastEvent := events[len(events)-1]
	if lastEvent.Content == nil {
		t.Error("Expected event to have content")
	}

	if len(lastEvent.Content.Parts) == 0 {
		t.Error("Expected event content to have parts")
	}

	firstPart := lastEvent.Content.Parts[0]
	if firstPart.Text == nil || *firstPart.Text != "Hello! How can I help you today?" {
		t.Errorf("Unexpected response text: %v", firstPart.Text)
	}
}

func TestEnhancedLlmAgent_ToolExecution(t *testing.T) {
	// Create mock LLM connection with function call response
	mockLLM := NewMockLLMConnection(
		// First response with function call
		&core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_1",
							Name: "test_tool",
							Args: map[string]any{
								"input": "test input",
							},
						},
					},
				},
			},
		},
		// Second response after tool execution
		&core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr("I used the test tool and got: test result"),
					},
				},
			},
		},
	)

	// Create agent and set LLM connection
	agent := NewEnhancedLlmAgent("test-agent", "A tool-using assistant", nil)
	agent.SetLLMConnection(mockLLM)

	// Add mock tool
	mockTool := NewMockTool("test_tool", "test result")
	agent.AddTool(mockTool)

	// Create test session and context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("Use the test tool"),
			},
		},
	}

	// Run the agent
	ctx := context.Background()
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify events
	if len(events) < 3 {
		t.Errorf("Expected at least 3 events (function call, function response, final response), got %d", len(events))
	}

	// Verify tool was called
	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called once, was called %d times", mockTool.callCount)
	}

	// Check function call event
	firstEvent := events[0]
	functionCalls := firstEvent.GetFunctionCalls()
	if len(functionCalls) != 1 {
		t.Errorf("Expected 1 function call, got %d", len(functionCalls))
	}
	if functionCalls[0].Name != "test_tool" {
		t.Errorf("Expected function call to 'test_tool', got '%s'", functionCalls[0].Name)
	}

	// Check function response event
	secondEvent := events[1]
	functionResponses := secondEvent.GetFunctionResponses()
	if len(functionResponses) != 1 {
		t.Errorf("Expected 1 function response, got %d", len(functionResponses))
	}
	if functionResponses[0].Name != "test_tool" {
		t.Errorf("Expected function response from 'test_tool', got '%s'", functionResponses[0].Name)
	}

	// Check final response
	lastEvent := events[len(events)-1]
	if lastEvent.Content == nil || len(lastEvent.Content.Parts) == 0 {
		t.Error("Expected final event to have content")
	}
}

func TestStreamingLlmAgent(t *testing.T) {
	// Create mock LLM responses for streaming
	responses := []*core.LLMResponse{
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr("Hello"),
					},
				},
			},
			Partial: llmBoolPtr(true),
		},
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr(" there!"),
					},
				},
			},
			Partial: llmBoolPtr(false),
		},
	}

	mockLLM := NewMockLLMConnection(responses...)

	// Create streaming agent
	agent := NewStreamingLlmAgent("streaming-agent", "A streaming assistant", nil)
	agent.SetLLMConnection(mockLLM)

	// Create test session and context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("Hello"),
			},
		},
	}

	// Run the streaming agent
	ctx := context.Background()
	eventStream, err := agent.RunAsync(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Streaming agent run failed: %v", err)
	}

	// Collect events from stream
	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
	}

	// Verify we got multiple events for streaming
	if len(events) < 2 {
		t.Errorf("Expected at least 2 streaming events, got %d", len(events))
	}

	// Verify first event is partial
	if events[0].Partial == nil || !*events[0].Partial {
		t.Error("Expected first event to be partial")
	}

	// Verify last event is complete
	lastEvent := events[len(events)-1]
	if lastEvent.Partial != nil && *lastEvent.Partial {
		t.Error("Expected last event to be complete (not partial)")
	}
}

func TestLlmAgentConfig(t *testing.T) {
	// Test default config
	defaultConfig := DefaultLlmAgentConfig()
	if defaultConfig.Model != "gemini-1.5-pro" {
		t.Errorf("Expected default model 'gemini-1.5-pro', got '%s'", defaultConfig.Model)
	}

	if defaultConfig.MaxToolCalls != 10 {
		t.Errorf("Expected default MaxToolCalls 10, got %d", defaultConfig.MaxToolCalls)
	}

	if defaultConfig.ToolCallTimeout != 30*time.Second {
		t.Errorf("Expected default ToolCallTimeout 30s, got %v", defaultConfig.ToolCallTimeout)
	}

	// Test custom config
	customConfig := &LlmAgentConfig{
		Model:            "custom-model",
		Temperature:      llmFloatPtr(0.5),
		MaxTokens:        llmIntPtr(2000),
		MaxToolCalls:     15,
		ToolCallTimeout:  45 * time.Second,
		RetryAttempts:    5,
		StreamingEnabled: true,
	}

	agent := NewEnhancedLlmAgent("test-agent", "Test agent", customConfig)

	if agent.Config().Model != "custom-model" {
		t.Errorf("Expected custom model 'custom-model', got '%s'", agent.Config().Model)
	}

	if *agent.Config().Temperature != 0.5 {
		t.Errorf("Expected custom temperature 0.5, got %f", *agent.Config().Temperature)
	}

	if agent.Config().StreamingEnabled != true {
		t.Error("Expected streaming to be enabled")
	}
}

func TestLlmAgent_CallbackExecution(t *testing.T) {
	var beforeModelCalled, afterModelCalled bool
	var beforeToolCalled, afterToolCalled bool

	callbacks := &LlmAgentCallbacks{
		BeforeModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
			beforeModelCalled = true
			return nil
		},
		AfterModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
			afterModelCalled = true
			return nil
		},
		BeforeToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
			beforeToolCalled = true
			return nil
		},
		AfterToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
			afterToolCalled = true
			return nil
		},
	}

	// Create mock LLM connection with tool call
	mockLLM := NewMockLLMConnection(
		&core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_1",
							Name: "test_tool",
							Args: map[string]any{},
						},
					},
				},
			},
		},
		&core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr("Tool execution completed"),
					},
				},
			},
		},
	)

	agent := NewEnhancedLlmAgent("callback-agent", "Agent with callbacks", nil)
	agent.SetLLMConnection(mockLLM)
	agent.SetCallbacks(callbacks)
	agent.AddTool(NewMockTool("test_tool", "tool result"))

	// Create test context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("Use the tool"),
			},
		},
	}

	// Run the agent
	ctx := context.Background()
	_, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify callbacks were called
	if !beforeModelCalled {
		t.Error("Expected before-model callback to be called")
	}
	if !afterModelCalled {
		t.Error("Expected after-model callback to be called")
	}
	if !beforeToolCalled {
		t.Error("Expected before-tool callback to be called")
	}
	if !afterToolCalled {
		t.Error("Expected after-tool callback to be called")
	}
}
