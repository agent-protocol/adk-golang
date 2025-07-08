package ollama

import (
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func TestConvertToOllamaRequest(t *testing.T) {
	// Create OllamaConnection
	config := &OllamaConfig{
		BaseURL:     "http://localhost:11434",
		Model:       "llama3.2",
		Temperature: ptr.Float32(0.7),
		MaxTokens:   ptr.Ptr(1000),
		TopP:        ptr.Float32(0.9),
		TopK:        ptr.Ptr(40),
	}
	conn := NewOllamaConnection(config)

	// Create test LLMRequest
	request := &core.LLMRequest{
		Contents: []core.Content{
			{
				Role: "user",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("Hello, how are you?"),
					},
				},
			},
			{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("I'm doing well, thank you!"),
					},
				},
			},
		},
		Config: &core.LLMConfig{
			Model:       "test-model",
			Temperature: ptr.Float32(0.5),
			MaxTokens:   ptr.Ptr(500),
		},
		Tools: []*core.FunctionDeclaration{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The location to get weather for",
						},
					},
					"required": []interface{}{"location"},
				},
			},
		},
	}

	// Test conversion
	chatReq, err := conn.convertToOllamaRequest(request, false)
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	// Validate result
	if chatReq == nil {
		t.Fatal("Converted request is nil")
	}

	if chatReq.Model != "llama3.2" {
		t.Errorf("Expected model 'llama3.2', got '%s'", chatReq.Model)
	}

	if chatReq.Stream == nil || *chatReq.Stream != false {
		t.Errorf("Expected stream to be false, got %v", chatReq.Stream)
	}

	if len(chatReq.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(chatReq.Messages))
	}

	// Check first message
	firstMsg := chatReq.Messages[0]
	if firstMsg.Role != "user" {
		t.Errorf("Expected first message role 'user', got '%s'", firstMsg.Role)
	}
	if firstMsg.Content != "Hello, how are you?" {
		t.Errorf("Expected first message content 'Hello, how are you?', got '%s'", firstMsg.Content)
	}

	// Check tools conversion
	if len(chatReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(chatReq.Tools))
	}

	tool := chatReq.Tools[0]
	if tool.Function.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", tool.Function.Name)
	}

	// Check options
	if temp, ok := chatReq.Options["temperature"].(float32); !ok || temp != 0.5 {
		t.Errorf("Expected temperature 0.5 from config, got %v", chatReq.Options["temperature"])
	}
}

func TestConvertFromOllamaResponse(t *testing.T) {
	// Create OllamaConnection
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Create test ChatResponse
	chatResp := &ChatResponse{
		Model:     "llama3.2",
		CreatedAt: time.Now(),
		Message: Message{
			Role:    "assistant",
			Content: "Hello! How can I help you today?",
		},
		Done:       true,
		DoneReason: "stop",
		Metrics: Metrics{
			TotalDuration:      time.Second,
			LoadDuration:       100 * time.Millisecond,
			PromptEvalCount:    10,
			PromptEvalDuration: 200 * time.Millisecond,
			EvalCount:          15,
			EvalDuration:       800 * time.Millisecond,
		},
	}

	// Test conversion
	llmResp := conn.convertFromOllamaResponse(chatResp)

	// Validate result
	if llmResp == nil {
		t.Fatal("Converted response is nil")
	}

	if llmResp.Content == nil {
		t.Fatal("Response content is nil")
	}

	if llmResp.Content.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", llmResp.Content.Role)
	}

	if len(llmResp.Content.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(llmResp.Content.Parts))
	}

	part := llmResp.Content.Parts[0]
	if part.Type != "text" {
		t.Errorf("Expected part type 'text', got '%s'", part.Type)
	}

	if part.Text == nil || *part.Text != "Hello! How can I help you today?" {
		t.Errorf("Expected text 'Hello! How can I help you today?', got %v", part.Text)
	}

	// Check partial flag
	if llmResp.Partial == nil || *llmResp.Partial != false {
		t.Errorf("Expected partial to be false (done=true), got %v", llmResp.Partial)
	}

	// Check metadata
	if model, ok := llmResp.Metadata["model"].(string); !ok || model != "llama3.2" {
		t.Errorf("Expected model in metadata to be 'llama3.2', got %v", llmResp.Metadata["model"])
	}

	if doneReason, ok := llmResp.Metadata["done_reason"].(string); !ok || doneReason != "stop" {
		t.Errorf("Expected done_reason in metadata to be 'stop', got %v", llmResp.Metadata["done_reason"])
	}

	// Check metrics in metadata
	if totalDuration, ok := llmResp.Metadata["total_duration"].(time.Duration); !ok || totalDuration != time.Second {
		t.Errorf("Expected total_duration in metadata to be 1s, got %v", llmResp.Metadata["total_duration"])
	}
}

func TestConvertToOllamaRequestWithFunctionCall(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Create LLMRequest with function call
	request := &core.LLMRequest{
		Contents: []core.Content{
			{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "function_call",
						FunctionCall: &core.FunctionCall{
							ID:   "call_123",
							Name: "get_weather",
							Args: map[string]any{
								"location": "New York",
							},
						},
					},
				},
			},
		},
	}

	// Test conversion
	chatReq, err := conn.convertToOllamaRequest(request, false)
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	// Validate tool calls
	if len(chatReq.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(chatReq.Messages))
	}

	msg := chatReq.Messages[0]
	if len(msg.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}

	toolCall := msg.ToolCalls[0]
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Expected tool call name 'get_weather', got '%s'", toolCall.Function.Name)
	}

	if location, ok := toolCall.Function.Arguments["location"].(string); !ok || location != "New York" {
		t.Errorf("Expected location 'New York', got %v", toolCall.Function.Arguments["location"])
	}
}

func TestConvertFromOllamaResponseWithToolCalls(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Create ChatResponse with tool calls
	chatResp := &ChatResponse{
		Model: "llama3.2",
		Message: Message{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					Function: ToolCallFunction{
						Index: 0,
						Name:  "get_weather",
						Arguments: ToolCallFunctionArguments{
							"location": "Boston",
						},
					},
				},
			},
		},
		Done: false, // Not done yet, waiting for tool response
	}

	// Test conversion
	llmResp := conn.convertFromOllamaResponse(chatResp)

	// Validate result
	if llmResp.Content == nil {
		t.Fatal("Response content is nil")
	}

	if len(llmResp.Content.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(llmResp.Content.Parts))
	}

	part := llmResp.Content.Parts[0]
	if part.Type != "function_call" {
		t.Errorf("Expected part type 'function_call', got '%s'", part.Type)
	}

	if part.FunctionCall == nil {
		t.Fatal("Function call is nil")
	}

	if part.FunctionCall.Name != "get_weather" {
		t.Errorf("Expected function call name 'get_weather', got '%s'", part.FunctionCall.Name)
	}

	if location, ok := part.FunctionCall.Args["location"].(string); !ok || location != "Boston" {
		t.Errorf("Expected location 'Boston', got %v", part.FunctionCall.Args["location"])
	}

	// Check partial flag (should be true since done=false)
	if llmResp.Partial == nil || *llmResp.Partial != true {
		t.Errorf("Expected partial to be true (done=false), got %v", llmResp.Partial)
	}
}

func TestConvertNilRequest(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	_, err := conn.convertToOllamaRequest(nil, false)
	if err == nil {
		t.Error("Expected error for nil request, got nil")
	}
}

func TestConvertNilResponse(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	result := conn.convertFromOllamaResponse(nil)
	if result == nil {
		t.Error("Expected empty response for nil input, got nil")
	}
}

func TestStreamingFlag(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	request := &core.LLMRequest{
		Contents: []core.Content{
			{
				Role: "user",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("Hello"),
					},
				},
			},
		},
	}

	// Test streaming=true
	chatReq, err := conn.convertToOllamaRequest(request, true)
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	if chatReq.Stream == nil || *chatReq.Stream != true {
		t.Errorf("Expected stream to be true, got %v", chatReq.Stream)
	}

	// Test streaming=false
	chatReq, err = conn.convertToOllamaRequest(request, false)
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	if chatReq.Stream == nil || *chatReq.Stream != false {
		t.Errorf("Expected stream to be false, got %v", chatReq.Stream)
	}
}
