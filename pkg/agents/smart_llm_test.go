package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// SmartMockLLMConnection simulates a real LLM that changes its response based on conversation history
type SmartMockLLMConnection struct {
	callCount int
}

func NewSmartMockLLMConnection() *SmartMockLLMConnection {
	return &SmartMockLLMConnection{}
}

func (m *SmartMockLLMConnection) GenerateContent(ctx context.Context, request *core.LLMRequest) (*core.LLMResponse, error) {
	m.callCount++

	// Check if we have function responses in the conversation history
	hasFunctionResponse := false
	for _, content := range request.Contents {
		if content.Role == "agent" {
			for _, part := range content.Parts {
				if part.Type == "function_response" {
					hasFunctionResponse = true
					break
				}
			}
		}
	}

	// If we have function responses, provide a final answer instead of calling tools again
	if hasFunctionResponse {
		return &core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("Based on the search results, I can see information about Melbourne weather from the Bureau of Meteorology. The current weather conditions show typical Melbourne weather patterns."),
					},
				},
			},
		}, nil
	}

	// First call - make function call
	return &core.LLMResponse{
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "function_call",
					FunctionCall: &core.FunctionCall{
						ID:   fmt.Sprintf("call_%d", m.callCount),
						Name: "duckduckgo_search",
						Args: map[string]interface{}{"query": "weather today in Melbourne"},
					},
				},
			},
		},
	}, nil
}

func (m *SmartMockLLMConnection) GenerateContentStream(ctx context.Context, request *core.LLMRequest) (<-chan *core.LLMResponse, error) {
	// For simplicity, just return a channel with a single response
	ch := make(chan *core.LLMResponse, 1)
	go func() {
		defer close(ch)
		response, _ := m.GenerateContent(ctx, request)
		if response != nil {
			ch <- response
		}
	}()
	return ch, nil
}

func (m *SmartMockLLMConnection) Close(ctx context.Context) error {
	return nil
}

// TestEnhancedLlmAgent_SmartLLMBehavior tests with a more realistic LLM that uses function responses
func TestEnhancedLlmAgent_SmartLLMBehavior(t *testing.T) {
	// Setup

	config := &LlmAgentConfig{
		Model:         "test-model",
		MaxToolCalls:  5,
		RetryAttempts: 1,
	}
	agent := NewLLMAgent("search-agent", "Search agent", config)

	// Use the smart mock that behaves like a real LLM
	smartMockLLM := NewSmartMockLLMConnection()
	agent.SetLLMConnection(smartMockLLM)

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
	var textResponseEvents []*core.Event

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
						textResponseEvents = append(textResponseEvents, event)
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

	t.Logf("Session has %d events total", len(invocationCtx.Session.Events))
	t.Logf("Found %d function call events", len(functionCallEvents))
	t.Logf("Found %d function response events", len(functionResponseEvents))
	t.Logf("Found %d text response events", len(textResponseEvents))

	// Verify expected behavior: tool called once, then final text response
	if len(functionCallEvents) != 1 {
		t.Errorf("Expected exactly 1 function call, got %d", len(functionCallEvents))
	}

	if len(functionResponseEvents) != 1 {
		t.Errorf("Expected exactly 1 function response, got %d", len(functionResponseEvents))
	}

	if len(textResponseEvents) != 1 {
		t.Errorf("Expected exactly 1 text response, got %d", len(textResponseEvents))
	}

	// Verify tool was called only once
	if mockTool.callCount != 1 {
		t.Errorf("Expected tool to be called exactly once, was called %d times", mockTool.callCount)
	}

	// Verify conversation completed properly
	var finalEvent *core.Event
	for _, event := range events {
		if event.TurnComplete != nil && *event.TurnComplete {
			finalEvent = event
			break
		}
	}

	if finalEvent == nil {
		t.Fatal("Expected a final event with TurnComplete=true")
	}

	// Verify final event contains text response
	if finalEvent.Content == nil || len(finalEvent.Content.Parts) == 0 {
		t.Fatal("Final event should have content")
	}

	finalPart := finalEvent.Content.Parts[0]
	if finalPart.Type != "text" || finalPart.Text == nil {
		t.Fatal("Final event should contain text response")
	}

	if !strings.Contains(*finalPart.Text, "Melbourne weather") {
		t.Errorf("Final response should mention Melbourne weather, got: %s", *finalPart.Text)
	}

	t.Logf("SUCCESS: Smart LLM behaved correctly - called tool once, then provided final answer")
}
