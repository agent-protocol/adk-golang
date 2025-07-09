package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// TestEnhancedLlmAgent_ToolCallLimitExceeded tests the scenario where tool call limit is exceeded
func TestEnhancedLlmAgent_ToolCallLimitExceeded(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Create agent with low tool call limit for testing
	config := &LlmAgentConfig{
		Model:         "test-model",
		MaxToolCalls:  1, // Very low limit to trigger quickly
		RetryAttempts: 1, // At least 1 retry attempt
	}
	agent := NewLLMAgent("test-agent", "Test agent", config)

	// Setup mock LLM connection with multiple responses to trigger the limit
	// With MaxToolCalls=1, total limit = 1*2 = 2, so we need 3+ tool calls to exceed it
	mockResponses := []*core.LLMResponse{
		// First LLM call returns function call
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_1",
							Name: "test_tool",
							Args: map[string]interface{}{"input": "test"},
						},
					},
				},
			},
		},
		// Second LLM call returns another function call
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_2",
							Name: "test_tool",
							Args: map[string]interface{}{"input": "test2"},
						},
					},
				},
			},
		},
		// Third LLM call returns another function call (this should trigger the limit)
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_3",
							Name: "test_tool",
							Args: map[string]interface{}{"input": "test3"},
						},
					},
				},
			},
		},
	}

	mockLLM := NewMockLLMConnection(mockResponses...)
	agent.SetLLMConnection(mockLLM)

	// Setup mock tool
	mockTool := NewMockTool("test_tool", map[string]interface{}{"result": "tool response"})
	agent.AddTool(mockTool)

	// Setup session and invocation context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{Type: "text", Text: ptr.Ptr("Please search for something")},
		},
	}

	// Configure run config with low max turns and tool calls
	invocationCtx.RunConfig = &core.RunConfig{
		MaxTurns: ptr.Ptr(5),
	}

	// Execute the agent
	eventStream, err := agent.RunAsync(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	if eventStream == nil {
		t.Fatal("EventStream should not be nil")
	}

	// Collect all events
	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
		t.Logf("Event: ID=%s, Author=%s, TurnComplete=%v, Error=%v",
			event.ID, event.Author,
			event.TurnComplete != nil && *event.TurnComplete,
			event.ErrorMessage != nil)

		if event.Content != nil {
			for i, part := range event.Content.Parts {
				switch part.Type {
				case "text":
					if part.Text != nil {
						t.Logf("  Part[%d]: text=%s", i, *part.Text)
					}
				case "function_call":
					if part.FunctionCall != nil {
						t.Logf("  Part[%d]: function_call=%s(%v)", i, part.FunctionCall.Name, part.FunctionCall.Args)
					}
				case "function_response":
					if part.FunctionResponse != nil {
						t.Logf("  Part[%d]: function_response=%s -> %v", i, part.FunctionResponse.Name, part.FunctionResponse.Response)
					}
				}
			}
		}
	}

	// Verify that we have events
	if len(events) == 0 {
		t.Fatal("Should have received events")
	}

	// Find the final response event and error event
	var finalEvent *core.Event
	var errorEvent *core.Event

	for _, event := range events {
		if event.TurnComplete != nil && *event.TurnComplete {
			finalEvent = event
		}
		if event.ErrorMessage != nil {
			errorEvent = event
		}
	}

	// The issue: Currently we get an error event instead of clean completion
	if errorEvent != nil {
		t.Logf("ERROR EVENT: %s", *errorEvent.ErrorMessage)
		// This demonstrates the current problematic behavior
		if !strings.Contains(*errorEvent.ErrorMessage, "conversation ended due to tool call limit") {
			t.Errorf("Expected error message to contain 'conversation ended due to tool call limit', got: %s", *errorEvent.ErrorMessage)
		}
	}

	if finalEvent != nil {
		t.Logf("FINAL EVENT found with TurnComplete=true")
		if finalEvent.TurnComplete == nil || !*finalEvent.TurnComplete {
			t.Error("Final event should have TurnComplete=true")
		}
		// Verify the final response message
		if finalEvent.Content != nil && len(finalEvent.Content.Parts) > 0 {
			if finalEvent.Content.Parts[0].Text != nil {
				if !strings.Contains(*finalEvent.Content.Parts[0].Text, "maximum number of tool calls") {
					t.Errorf("Expected final message to contain 'maximum number of tool calls', got: %s", *finalEvent.Content.Parts[0].Text)
				}
			}
		}
	} else {
		t.Log("No final event with TurnComplete=true found")
	}

	// Verify tool was called at least once
	if mockTool.callCount < 1 {
		t.Errorf("Expected tool to be called at least once, was called %d times", mockTool.callCount)
	}
}

// TestEnhancedLlmAgent_RepeatingPatternDetection tests the repeating pattern detection
func TestEnhancedLlmAgent_RepeatingPatternDetection(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Create agent
	config := &LlmAgentConfig{
		Model:         "test-model",
		MaxToolCalls:  5, // Higher limit to test pattern detection
		RetryAttempts: 1,
	}
	agent := NewLLMAgent("test-agent", "Test agent", config)

	// Setup mock LLM connection - simulate the same tool being called repeatedly
	// This should trigger pattern detection after 3 consecutive calls
	mockResponses := []*core.LLMResponse{}
	for i := 0; i < 4; i++ {
		mockResponses = append(mockResponses, &core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_" + string(rune('1'+i)),
							Name: "search_tool",
							Args: map[string]interface{}{"query": "cars"},
						},
					},
				},
			},
		})
	}

	mockLLM := NewMockLLMConnection(mockResponses...)
	agent.SetLLMConnection(mockLLM)

	// Setup mock tool
	mockTool := NewMockTool("search_tool", map[string]interface{}{"result": "search results"})
	agent.AddTool(mockTool)

	// Setup session and invocation context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{Type: "text", Text: ptr.Ptr("Please search for cars")},
		},
	}

	// Configure run config
	invocationCtx.RunConfig = &core.RunConfig{
		MaxTurns: ptr.Ptr(10),
	}

	// Execute the agent
	eventStream, err := agent.RunAsync(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	if eventStream == nil {
		t.Fatal("EventStream should not be nil")
	}

	// Collect all events
	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
		t.Logf("Event: ID=%s, Author=%s, TurnComplete=%v, Error=%v",
			event.ID, event.Author,
			event.TurnComplete != nil && *event.TurnComplete,
			event.ErrorMessage != nil)
	}

	// Verify that pattern detection worked
	if len(events) == 0 {
		t.Fatal("Should have received events")
	}

	// Find the final response event (should be created by pattern detection)
	var finalEvent *core.Event
	for _, event := range events {
		if event.TurnComplete != nil && *event.TurnComplete {
			finalEvent = event
			break
		}
	}

	if finalEvent == nil {
		t.Fatal("Should have a final event due to pattern detection")
	}

	if finalEvent.Content != nil && len(finalEvent.Content.Parts) > 0 {
		if finalEvent.Content.Parts[0].Text != nil {
			if !strings.Contains(*finalEvent.Content.Parts[0].Text, "completed the tool execution") {
				t.Errorf("Expected final message to contain 'completed the tool execution', got: %s", *finalEvent.Content.Parts[0].Text)
			}
		}
	}

	// Verify tool was called multiple times
	if mockTool.callCount < 3 {
		t.Errorf("Expected tool to be called at least 3 times for pattern detection, was called %d times", mockTool.callCount)
	}
}

// TestEnhancedLlmAgent_NormalCompletion tests normal conversation completion
func TestEnhancedLlmAgent_NormalCompletion(t *testing.T) {
	// Setup
	ctx := context.Background()

	config := &LlmAgentConfig{
		Model:         "test-model",
		MaxToolCalls:  10,
		RetryAttempts: 1,
	}
	agent := NewLLMAgent("test-agent", "Test agent", config)

	// Setup mock LLM connection
	mockResponses := []*core.LLMResponse{
		// First call - returns function call
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_1",
							Name: "search_tool",
							Args: map[string]interface{}{"input": "test"},
						},
					},
				},
			},
		},
		// Second call - returns final text response (no function calls)
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("Based on the search results, here's what I found..."),
					},
				},
			},
		},
	}

	mockLLM := NewMockLLMConnection(mockResponses...)
	agent.SetLLMConnection(mockLLM)

	// Setup mock tool
	mockTool := NewMockTool("search_tool", map[string]interface{}{"result": "search completed"})
	agent.AddTool(mockTool)

	// Setup session and invocation context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext("test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{Type: "text", Text: ptr.Ptr("Search for something")},
		},
	}

	// Execute the agent
	eventStream, err := agent.RunAsync(ctx, invocationCtx)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	if eventStream == nil {
		t.Fatal("EventStream should not be nil")
	}

	// Collect all events
	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
	}

	// Verify normal completion
	if len(events) == 0 {
		t.Fatal("Should have received events")
	}

	// The last event should be the final response with TurnComplete=true
	lastEvent := events[len(events)-1]
	if lastEvent.TurnComplete == nil || !*lastEvent.TurnComplete {
		t.Error("Last event should have TurnComplete=true")
	}
	if lastEvent.ErrorMessage != nil {
		t.Errorf("Should not have error message in normal completion, got: %s", *lastEvent.ErrorMessage)
	}

	// Verify the final response content
	if lastEvent.Content == nil {
		t.Fatal("Last event should have content")
	}
	if lastEvent.Content.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got: %s", lastEvent.Content.Role)
	}
	if len(lastEvent.Content.Parts) == 0 {
		t.Fatal("Last event should have content parts")
	}
	if lastEvent.Content.Parts[0].Type != "text" {
		t.Errorf("Expected text part, got: %s", lastEvent.Content.Parts[0].Type)
	}
	if lastEvent.Content.Parts[0].Text == nil {
		t.Fatal("Text part should have text content")
	}
	if !strings.Contains(*lastEvent.Content.Parts[0].Text, "Based on the search results") {
		t.Errorf("Expected final response to contain 'Based on the search results', got: %s", *lastEvent.Content.Parts[0].Text)
	}

	// Verify tool was called exactly once
	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called exactly once, was called %d times", mockTool.callCount)
	}
}
