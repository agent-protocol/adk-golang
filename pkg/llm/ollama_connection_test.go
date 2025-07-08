package llm

import (
	"context"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func TestOllamaConnection_Creation(t *testing.T) {
	// Test default config
	conn := NewOllamaConnectionFromEnv()
	if conn == nil {
		t.Fatal("Expected connection to be created")
	}

	if conn.baseURL == "" {
		t.Error("Expected baseURL to be set")
	}

	if conn.model == "" {
		t.Error("Expected model to be set")
	}
}

func TestOllamaConnection_CustomConfig(t *testing.T) {
	config := &OllamaConfig{
		BaseURL:     "http://custom:11434",
		Model:       "custom-model",
		Temperature: ptr.Float32(0.5),
		Timeout:     60 * time.Second,
	}

	conn := NewOllamaConnection(config)
	if conn.baseURL != "http://custom:11434" {
		t.Errorf("Expected baseURL to be 'http://custom:11434', got '%s'", conn.baseURL)
	}

	if conn.model != "custom-model" {
		t.Errorf("Expected model to be 'custom-model', got '%s'", conn.model)
	}
}

func TestOllamaConnection_RequestConversion(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Create test request
	request := &core.LLMRequest{
		Contents: []core.Content{
			{
				Role: "user",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("Hello, world!"),
					},
				},
			},
		},
		Config: &core.LLMConfig{
			Model:       "test-model",
			Temperature: ptr.Float32(0.8),
		},
	}

	// Convert to Ollama format
	ollamaReq, err := conn.convertToOllamaRequest(request, false)
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	if ollamaReq.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", ollamaReq.Model)
	}

	if len(ollamaReq.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(ollamaReq.Messages))
	}

	if ollamaReq.Messages[0].Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", ollamaReq.Messages[0].Content)
	}

	temp, ok := ollamaReq.Options["temperature"].(float32)
	if !ok {
		t.Error("Expected temperature to be set as float32")
	} else if temp != 0.8 {
		t.Errorf("Expected temperature 0.8, got %f", temp)
	}
}

func TestOllamaConnection_ResponseConversion(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Create test Ollama response
	ollamaResp := &OllamaChatResponse{
		Model:     "test-model",
		CreatedAt: "2024-01-01T00:00:00Z",
		Message: OllamaMessage{
			Role:    "assistant",
			Content: "Hello back!",
		},
		Done: true,
	}

	// Convert to ADK format
	adkResp := conn.convertFromOllamaResponse(ollamaResp)

	if adkResp.Content == nil {
		t.Fatal("Expected content to be set")
	}

	if adkResp.Content.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", adkResp.Content.Role)
	}

	if len(adkResp.Content.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(adkResp.Content.Parts))
	}

	part := adkResp.Content.Parts[0]
	if part.Type != "text" {
		t.Errorf("Expected part type 'text', got '%s'", part.Type)
	}

	if part.Text == nil || *part.Text != "Hello back!" {
		t.Errorf("Expected text 'Hello back!', got '%v'", part.Text)
	}

	if adkResp.Partial == nil || *adkResp.Partial != false {
		t.Error("Expected partial to be false for done response")
	}
}

func TestOllamaConnection_RoleMapping(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	tests := []struct {
		input    string
		expected string
	}{
		{"user", "user"},
		{"agent", "assistant"},
		{"model", "assistant"},
		{"assistant", "assistant"},
		{"system", "system"},
		{"unknown", "user"},
	}

	for _, test := range tests {
		result := conn.mapRole(test.input)
		if result != test.expected {
			t.Errorf("mapRole(%s): expected %s, got %s", test.input, test.expected, result)
		}
	}
}

func TestOllamaConnection_ToolConversion(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Create request with tools
	request := &core.LLMRequest{
		Contents: []core.Content{
			{
				Role: "user",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("Calculate 2 + 2"),
					},
				},
			},
		},
		Tools: []*core.FunctionDeclaration{
			{
				Name:        "calculator",
				Description: "Performs basic math",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{"type": "string"},
						"a":         map[string]interface{}{"type": "number"},
						"b":         map[string]interface{}{"type": "number"},
					},
				},
			},
		},
	}

	ollamaReq, err := conn.convertToOllamaRequest(request, false)
	if err != nil {
		t.Fatalf("Failed to convert request: %v", err)
	}

	if len(ollamaReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(ollamaReq.Tools))
	}

	tool := ollamaReq.Tools[0]
	if tool.Type != "function" {
		t.Errorf("Expected tool type 'function', got '%s'", tool.Type)
	}

	if tool.Function.Name != "calculator" {
		t.Errorf("Expected function name 'calculator', got '%s'", tool.Function.Name)
	}

	if tool.Function.Description != "Performs basic math" {
		t.Errorf("Expected description 'Performs basic math', got '%s'", tool.Function.Description)
	}
}

func TestOllamaConnection_Close(t *testing.T) {
	conn := NewOllamaConnection(DefaultOllamaConfig())

	// Close should not return an error
	err := conn.Close(context.Background())
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// Note: The following test would require an actual Ollama server running
// func TestOllamaConnection_GenerateContent_Integration(t *testing.T) {
//     // This test would require Ollama to be running
//     // and would make actual HTTP requests
// }
