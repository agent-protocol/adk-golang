package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
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
						Text: ptr.Ptr("Default response"),
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
		Temperature:     ptr.Float32(0.7),
		MaxTokens:       ptr.Ptr(int(1000)),
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
						Text: ptr.Ptr("Hello! How can I help you today?"),
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
				Text: ptr.Ptr("Hello"),
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
						Text: ptr.Ptr("I used the test tool and got: test result"),
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
				Text: ptr.Ptr("Use the test tool"),
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
						Text: ptr.Ptr("Hello"),
					},
				},
			},
			Partial: ptr.Ptr(true),
		},
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr(" there!"),
					},
				},
			},
			Partial: ptr.Ptr(false),
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
				Text: ptr.Ptr("Hello"),
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
		Temperature:      ptr.Float32(0.5),
		MaxTokens:        ptr.Ptr(2000),
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
						Text: ptr.Ptr("Tool execution completed"),
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
				Text: ptr.Ptr("Use the tool"),
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

// Test loop detection functionality
func TestLoopDetector_CheckToolCallLimit(t *testing.T) {
	detector := NewLoopDetector()

	// Test normal case - under limit
	functionCalls := []*core.FunctionCall{
		{ID: "1", Name: "test_tool", Args: map[string]any{}},
		{ID: "2", Name: "test_tool", Args: map[string]any{}},
	}

	exceeded := detector.CheckToolCallLimit(functionCalls, 10)
	if exceeded {
		t.Error("Expected tool call limit not to be exceeded")
	}

	if detector.totalToolCalls != 2 {
		t.Errorf("Expected total tool calls to be 2, got %d", detector.totalToolCalls)
	}

	// Test exceeding limit
	moreCalls := []*core.FunctionCall{
		{ID: "3", Name: "test_tool", Args: map[string]any{}},
		{ID: "4", Name: "test_tool", Args: map[string]any{}},
		{ID: "5", Name: "test_tool", Args: map[string]any{}},
		{ID: "6", Name: "test_tool", Args: map[string]any{}},
		{ID: "7", Name: "test_tool", Args: map[string]any{}},
		{ID: "8", Name: "test_tool", Args: map[string]any{}},
		{ID: "9", Name: "test_tool", Args: map[string]any{}},
		{ID: "10", Name: "test_tool", Args: map[string]any{}},
		{ID: "11", Name: "test_tool", Args: map[string]any{}},
	}

	exceeded = detector.CheckToolCallLimit(moreCalls, 10)
	if !exceeded {
		t.Error("Expected tool call limit to be exceeded")
	}

	if detector.totalToolCalls != 11 {
		t.Errorf("Expected total tool calls to be 11, got %d", detector.totalToolCalls)
	}
}

func TestLoopDetector_CheckRepeatingPattern(t *testing.T) {
	detector := NewLoopDetector()

	// Test case 1: Not enough events
	events := []*core.Event{
		{Content: &core.Content{Role: "user", Parts: []core.Part{{Type: "text", Text: ptr.Ptr("Hello")}}}},
		{Content: &core.Content{Role: "assistant", Parts: []core.Part{{Type: "text", Text: ptr.Ptr("Hi")}}}},
	}

	isRepeating := detector.CheckRepeatingPattern(events, 1)
	if isRepeating {
		t.Error("Expected no repeating pattern with insufficient events")
	}

	// Test case 2: Different functions - no pattern
	events = []*core.Event{
		{Content: &core.Content{Role: "user", Parts: []core.Part{{Type: "text", Text: ptr.Ptr("Hello")}}}},
		{Content: &core.Content{Role: "assistant", Parts: []core.Part{{Type: "function_call", FunctionCall: &core.FunctionCall{ID: "1", Name: "tool1", Args: map[string]any{}}}}}},
		{Content: &core.Content{Role: "agent", Parts: []core.Part{{Type: "function_response", FunctionResponse: &core.FunctionResponse{ID: "1", Name: "tool1", Response: map[string]any{"result": "response1"}}}}}},
		{Content: &core.Content{Role: "assistant", Parts: []core.Part{{Type: "function_call", FunctionCall: &core.FunctionCall{ID: "2", Name: "tool2", Args: map[string]any{}}}}}},
		{Content: &core.Content{Role: "agent", Parts: []core.Part{{Type: "function_response", FunctionResponse: &core.FunctionResponse{ID: "2", Name: "tool2", Response: map[string]any{"result": "response2"}}}}}},
	}

	isRepeating = detector.CheckRepeatingPattern(events, 3)
	if isRepeating {
		t.Error("Expected no repeating pattern with different functions")
	}

	// Test case 3: Same function called 3 times consecutively - should detect pattern
	events = []*core.Event{
		{Content: &core.Content{Role: "user", Parts: []core.Part{{Type: "text", Text: ptr.Ptr("Hello")}}}},
		{Content: &core.Content{Role: "assistant", Parts: []core.Part{{Type: "function_call", FunctionCall: &core.FunctionCall{ID: "1", Name: "same_tool", Args: map[string]any{}}}}}},
		{Content: &core.Content{Role: "agent", Parts: []core.Part{{Type: "function_response", FunctionResponse: &core.FunctionResponse{ID: "1", Name: "same_tool", Response: map[string]any{"result": "response1"}}}}}},
		{Content: &core.Content{Role: "assistant", Parts: []core.Part{{Type: "function_call", FunctionCall: &core.FunctionCall{ID: "2", Name: "same_tool", Args: map[string]any{}}}}}},
		{Content: &core.Content{Role: "agent", Parts: []core.Part{{Type: "function_response", FunctionResponse: &core.FunctionResponse{ID: "2", Name: "same_tool", Response: map[string]any{"result": "response2"}}}}}},
		{Content: &core.Content{Role: "assistant", Parts: []core.Part{{Type: "function_call", FunctionCall: &core.FunctionCall{ID: "3", Name: "same_tool", Args: map[string]any{}}}}}},
		{Content: &core.Content{Role: "agent", Parts: []core.Part{{Type: "function_response", FunctionResponse: &core.FunctionResponse{ID: "3", Name: "same_tool", Response: map[string]any{"result": "response3"}}}}}},
	}

	isRepeating = detector.CheckRepeatingPattern(events, 5)
	if !isRepeating {
		t.Error("Expected repeating pattern to be detected with same function called 3 times")
	}
}

func TestEnhancedLlmAgent_LoopDetection_ToolCallLimit(t *testing.T) {
	// Create mock LLM that always returns function calls
	var responses []*core.LLMResponse

	// Create 15 responses with function calls to exceed the limit
	for i := 0; i < 15; i++ {
		responses = append(responses, &core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   fmt.Sprintf("call_%d", i),
							Name: "test_tool",
							Args: map[string]any{"input": fmt.Sprintf("test%d", i)},
						},
					},
				},
			},
		})
	}

	mockLLM := NewMockLLMConnection(responses...)

	// Create agent with low tool call limit
	config := &LlmAgentConfig{
		Model:           "test-model",
		MaxToolCalls:    10, // This will be multiplied by 2 for total limit
		ToolCallTimeout: 10 * time.Second,
		RetryAttempts:   1,
	}

	agent := NewEnhancedLlmAgent("loop-test-agent", "Agent for loop testing", config)
	agent.SetLLMConnection(mockLLM)

	// Add mock tool
	mockTool := NewMockTool("test_tool", "tool result")
	agent.AddTool(mockTool)

	// Create test session and context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("Keep using the tool"),
			},
		},
	}

	// Run the agent
	ctx := context.Background()
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Should have terminated due to tool call limit OR repeating pattern (both are valid loop detection)
	foundToolLimit := false
	foundRepeatingPattern := false
	for _, event := range events {
		if event.Content != nil && len(event.Content.Parts) > 0 {
			for _, part := range event.Content.Parts {
				if part.Type == "text" && part.Text != nil {
					text := *part.Text
					if strings.Contains(text, "maximum number of tool calls") {
						foundToolLimit = true
					}
					if strings.Contains(text, "completed the tool execution") {
						foundRepeatingPattern = true
					}
				}
			}
		}
	}

	if !foundToolLimit && !foundRepeatingPattern {
		t.Error("Expected conversation to end with either tool call limit message or repeating pattern detection")
	}

	// Verify tool was called multiple times but not excessively
	if mockTool.callCount == 0 {
		t.Error("Expected tool to be called at least once")
	}

	if mockTool.callCount > 25 { // Should be limited by our logic
		t.Errorf("Tool was called too many times: %d", mockTool.callCount)
	}
}

func TestEnhancedLlmAgent_LoopDetection_RepeatingPattern(t *testing.T) {
	// Create mock LLM that returns the same function call multiple times
	var responses []*core.LLMResponse

	// Create responses that will trigger repeating pattern detection
	for i := 0; i < 10; i++ {
		responses = append(responses,
			// Function call response
			&core.LLMResponse{
				Content: &core.Content{
					Role: "assistant",
					Parts: []core.Part{
						{
							Type: "function_call",
							FunctionCall: &core.FunctionCall{
								ID:   fmt.Sprintf("call_%d", i),
								Name: "repeating_tool", // Same tool name
								Args: map[string]any{"input": "same input"},
							},
						},
					},
				},
			},
			// Follow-up response after tool execution
			&core.LLMResponse{
				Content: &core.Content{
					Role: "assistant",
					Parts: []core.Part{
						{
							Type: "function_call",
							FunctionCall: &core.FunctionCall{
								ID:   fmt.Sprintf("call_%d_followup", i),
								Name: "repeating_tool", // Same tool again
								Args: map[string]any{"input": "same input again"},
							},
						},
					},
				},
			},
		)
	}

	mockLLM := NewMockLLMConnection(responses...)

	// Create agent
	agent := NewEnhancedLlmAgent("pattern-test-agent", "Agent for pattern testing", nil)
	agent.SetLLMConnection(mockLLM)

	// Add mock tool
	mockTool := NewMockTool("repeating_tool", "same result")
	agent.AddTool(mockTool)

	// Create test session and context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("Use the repeating tool"),
			},
		},
	}

	// Run the agent
	ctx := context.Background()
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Should have terminated due to repeating pattern
	found := false
	for _, event := range events {
		if event.Content != nil && len(event.Content.Parts) > 0 {
			for _, part := range event.Content.Parts {
				if part.Type == "text" && part.Text != nil {
					text := *part.Text
					if strings.Contains(text, "completed the tool execution") {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Error("Expected conversation to end with repeating pattern detection message")
	}

	// Verify tool was called but conversation was terminated
	if mockTool.callCount == 0 {
		t.Error("Expected tool to be called at least once")
	}

	// Should not have been called too many times due to pattern detection
	if mockTool.callCount > 10 {
		t.Errorf("Tool was called too many times despite pattern detection: %d", mockTool.callCount)
	}
}

func TestEnhancedLlmAgent_LoopDetection_MaxTurns(t *testing.T) {
	// Create mock LLM that always returns function calls
	var responses []*core.LLMResponse

	// Create many responses to exceed max turns
	for i := 0; i < 20; i++ {
		responses = append(responses, &core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   fmt.Sprintf("call_%d", i),
							Name: "slow_tool",
							Args: map[string]any{"input": fmt.Sprintf("test%d", i)},
						},
					},
				},
			},
		})
	}

	mockLLM := NewMockLLMConnection(responses...)

	// Create agent
	agent := NewEnhancedLlmAgent("max-turns-test-agent", "Agent for max turns testing", nil)
	agent.SetLLMConnection(mockLLM)

	// Add mock tool
	mockTool := NewMockTool("slow_tool", "slow result")
	agent.AddTool(mockTool)

	// Create test session and context with limited max turns
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.RunConfig = &core.RunConfig{
		MaxTurns: ptr.Ptr(5), // Low limit to test termination
	}
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("Keep using the tool"),
			},
		},
	}

	// Run the agent
	ctx := context.Background()
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Should have terminated due to max turns
	// The conversation should have ended within the turn limit
	if len(events) == 0 {
		t.Error("Expected at least some events")
	}

	// Verify tool was called but not excessively
	if mockTool.callCount == 0 {
		t.Error("Expected tool to be called at least once")
	}

	// Should be limited by max turns
	if mockTool.callCount > 10 { // Should be much less due to turn limit
		t.Errorf("Tool was called too many times: %d", mockTool.callCount)
	}
}

func TestConversationFlowManager_Creation(t *testing.T) {
	agent := NewEnhancedLlmAgent("test-agent", "Test agent", nil)
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)

	// Test default max turns
	flowManager := NewConversationFlowManager(agent, invocationCtx)
	if flowManager.maxTurns != 10 {
		t.Errorf("Expected default max turns to be 10, got %d", flowManager.maxTurns)
	}

	// Test custom max turns
	invocationCtx.RunConfig = &core.RunConfig{
		MaxTurns: ptr.Ptr(15),
	}
	flowManager = NewConversationFlowManager(agent, invocationCtx)
	if flowManager.maxTurns != 15 {
		t.Errorf("Expected custom max turns to be 15, got %d", flowManager.maxTurns)
	}

	// Test max tool calls calculation
	expectedMaxToolCalls := agent.config.MaxToolCalls * 2
	if flowManager.maxToolCalls != expectedMaxToolCalls {
		t.Errorf("Expected max tool calls to be %d, got %d", expectedMaxToolCalls, flowManager.maxToolCalls)
	}
}

func TestEventPublisher_PublishEvent(t *testing.T) {
	publisher := NewEventPublisher()
	eventChan := make(chan *core.Event, 1)

	event := &core.Event{
		ID:     "test-event",
		Author: "test-agent",
	}

	ctx := context.Background()
	err := publisher.PublishEvent(ctx, eventChan, event)
	if err != nil {
		t.Fatalf("Expected no error publishing event, got: %v", err)
	}

	// Verify event was published
	select {
	case receivedEvent := <-eventChan:
		if receivedEvent.ID != "test-event" {
			t.Errorf("Expected event ID 'test-event', got '%s'", receivedEvent.ID)
		}
	default:
		t.Error("Expected event to be published to channel")
	}
}

func TestEventPublisher_CreateFinalResponse(t *testing.T) {
	publisher := NewEventPublisher()

	event := publisher.CreateFinalResponse("test-invocation", "test-agent", "Final message")

	if event.InvocationID != "test-invocation" {
		t.Errorf("Expected invocation ID 'test-invocation', got '%s'", event.InvocationID)
	}

	if event.Author != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", event.Author)
	}

	if event.Content == nil {
		t.Error("Expected event to have content")
	}

	if event.Content.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", event.Content.Role)
	}

	if len(event.Content.Parts) != 1 {
		t.Errorf("Expected 1 content part, got %d", len(event.Content.Parts))
	}

	if event.Content.Parts[0].Type != "text" {
		t.Errorf("Expected text part, got '%s'", event.Content.Parts[0].Type)
	}

	if event.Content.Parts[0].Text == nil || *event.Content.Parts[0].Text != "Final message" {
		t.Errorf("Expected text 'Final message', got %v", event.Content.Parts[0].Text)
	}

	if event.TurnComplete == nil || !*event.TurnComplete {
		t.Error("Expected turn to be complete")
	}
}
