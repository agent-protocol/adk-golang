package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// TestEnhancedLlmAgent_ConversationDuplication tests the specific issue where
// the same user query is repeated and causes the LLM to call the same tool repeatedly
func TestEnhancedLlmAgent_ConversationDuplication(t *testing.T) {

	config := &LlmAgentConfig{
		Model:         "test-model",
		MaxToolCalls:  5, // Higher limit to allow the pattern to emerge
		RetryAttempts: 1,
	}
	agent := NewLLMAgent("search-agent", "Search agent", config)

	// This test simulates the exact behavior seen in the logs:
	// The LLM keeps calling the same tool with the same arguments
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
							Name: "duckduckgo_search",
							Args: map[string]interface{}{"query": "weather today in Melbourne"},
						},
					},
				},
			},
		},
		// Second call - returns the SAME function call (this is the problem!)
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_2",
							Name: "duckduckgo_search",
							Args: map[string]interface{}{"query": "weather today in Melbourne"},
						},
					},
				},
			},
		},
		// Third call - returns the SAME function call again
		{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_3",
							Name: "duckduckgo_search",
							Args: map[string]interface{}{"query": "weather today in Melbourne"},
						},
					},
				},
			},
		},
	}

	mockLLM := NewMockLLMConnection(mockResponses...)
	agent.SetLLMConnection(mockLLM)

	// Setup mock tool
	mockTool := NewMockTool("duckduckgo_search", map[string]interface{}{
		"query": "weather today in Melbourne",
		"results": []map[string]interface{}{
			{
				"title":   "Melbourne Weather - Bureau of Meteorology",
				"url":     "http://www.bom.gov.au/places/vic/melbourne/",
				"snippet": "Information about Melbourne Weather - Bureau of Meteorology",
			},
		},
	})
	agent.AddTool(mockTool)

	// Setup session and invocation context
	session := core.NewSession("test-session", "test-app", "test-user")
	invocationCtx := core.NewInvocationContext(context.Background(), "test-invocation", agent, session, nil)
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{Type: "text", Text: ptr.Ptr("What is today weather in Melbourne")},
		},
	}

	// Execute the agent
	eventStream, err := agent.RunAsync(invocationCtx)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}
	if eventStream == nil {
		t.Fatal("EventStream should not be nil")
	}

	// Collect all events
	var events []*core.Event
	var functionCallEvents []*core.Event
	var functionResponseEvents []*core.Event

	for event := range eventStream {
		events = append(events, event)
		t.Logf("Event: ID=%s, Author=%s, TurnComplete=%v",
			event.ID, event.Author,
			event.TurnComplete != nil && *event.TurnComplete)

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
						functionCallEvents = append(functionCallEvents, event)
					}
				case "function_response":
					if part.FunctionResponse != nil {
						t.Logf("  Part[%d]: function_response=%s", i, part.FunctionResponse.Name)
						functionResponseEvents = append(functionResponseEvents, event)
					}
				}
			}
		}
	}

	// Verify that we have events
	if len(events) == 0 {
		t.Fatal("Should have received events")
	}

	// Log session state for analysis
	t.Logf("Session has %d events total", len(invocationCtx.Session.Events))
	t.Logf("Found %d function call events", len(functionCallEvents))
	t.Logf("Found %d function response events", len(functionResponseEvents))

	// Count how many times the same tool was called with same arguments
	sameToolCallCount := 0
	for _, event := range functionCallEvents {
		if event.Content != nil && len(event.Content.Parts) > 0 {
			if part := event.Content.Parts[0]; part.FunctionCall != nil {
				if part.FunctionCall.Name == "duckduckgo_search" {
					if query, ok := part.FunctionCall.Args["query"].(string); ok {
						if query == "weather today in Melbourne" {
							sameToolCallCount++
						}
					}
				}
			}
		}
	}

	t.Logf("Same tool called %d times with same arguments", sameToolCallCount)

	// The test should demonstrate that this is problematic behavior
	// In a well-behaved system, we would expect either:
	// 1. The tool to be called once and then the LLM to provide a text response
	// 2. Or the repeating pattern detection to kick in after 3 calls

	// For now, let's just document what we observe
	if sameToolCallCount > 1 {
		t.Logf("ISSUE DETECTED: Same tool called %d times with identical arguments", sameToolCallCount)
		t.Logf("This suggests the LLM is not properly using the tool responses from previous calls")
	}

	// Verify tool was called multiple times (demonstrating the issue)
	if mockTool.callCount < 2 {
		t.Errorf("Expected tool to be called at least 2 times to demonstrate the issue, was called %d times", mockTool.callCount)
	}

	// Check if we eventually got a final response
	var finalEvent *core.Event
	for _, event := range events {
		if event.TurnComplete != nil && *event.TurnComplete {
			finalEvent = event
			break
		}
	}

	if finalEvent != nil {
		t.Logf("Conversation completed with final event")
		if finalEvent.Content != nil && len(finalEvent.Content.Parts) > 0 {
			if finalEvent.Content.Parts[0].Text != nil {
				if strings.Contains(*finalEvent.Content.Parts[0].Text, "completed the tool execution") ||
					strings.Contains(*finalEvent.Content.Parts[0].Text, "maximum number of tool calls") {
					t.Logf("Final response was generated by loop detection: %s", *finalEvent.Content.Parts[0].Text)
				}
			}
		}
	} else {
		t.Log("No final event found - conversation may not have completed properly")
	}
}
