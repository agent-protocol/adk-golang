package a2a

import (
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func TestConvertA2AMessageToContent(t *testing.T) {
	// Test with text message
	a2aMessage := &Message{
		MessageID: "test123",
		Role:      "user",
		Parts: []Part{
			{
				Type: "text",
				Text: ptr.Ptr("Hello, world!"),
			},
		},
	}

	content := ConvertA2AMessageToContent(a2aMessage)

	if content == nil {
		t.Fatal("Expected content, got nil")
	}

	if content.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", content.Role)
	}

	if len(content.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(content.Parts))
	}

	if content.Parts[0].Type != "text" {
		t.Errorf("Expected part type 'text', got '%s'", content.Parts[0].Type)
	}

	if content.Parts[0].Text == nil || *content.Parts[0].Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got %v", content.Parts[0].Text)
	}
}

func TestConvertCoreContentToA2AMessage(t *testing.T) {
	// Test with text content
	content := &core.Content{
		Role: "agent",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("How can I help you?"),
			},
		},
	}

	message := ConvertCoreContentToA2AMessage(content, "msg456")

	if message == nil {
		t.Fatal("Expected message, got nil")
	}

	if message.MessageID != "msg456" {
		t.Errorf("Expected messageID 'msg456', got '%s'", message.MessageID)
	}

	if message.Role != "agent" {
		t.Errorf("Expected role 'agent', got '%s'", message.Role)
	}

	if len(message.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(message.Parts))
	}

	if message.Parts[0].Type != "text" {
		t.Errorf("Expected part type 'text', got '%s'", message.Parts[0].Type)
	}

	if message.Parts[0].Text == nil || *message.Parts[0].Text != "How can I help you?" {
		t.Errorf("Expected text 'How can I help you?', got %v", message.Parts[0].Text)
	}
}

func TestConvertA2ATaskStatusToContent(t *testing.T) {
	// Test with task status containing a message
	status := &TaskStatus{
		State: TaskStateCompleted,
		Message: &Message{
			MessageID: "status123",
			Role:      "agent",
			Parts: []Part{
				{
					Type: "text",
					Text: ptr.Ptr("Task completed successfully"),
				},
			},
		},
	}

	content := ConvertA2ATaskStatusToContent(status)

	if content == nil {
		t.Fatal("Expected content, got nil")
	}

	if content.Role != "agent" {
		t.Errorf("Expected role 'agent', got '%s'", content.Role)
	}

	if len(content.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(content.Parts))
	}

	if content.Parts[0].Text == nil || *content.Parts[0].Text != "Task completed successfully" {
		t.Errorf("Expected text 'Task completed successfully', got %v", content.Parts[0].Text)
	}
}

func TestConvertA2AArtifactToContent(t *testing.T) {
	// Test with artifact containing parts
	artifact := &Artifact{
		Name:        ptr.Ptr("test-artifact"),
		Description: ptr.Ptr("A test artifact"),
		Parts: []Part{
			{
				Type: "text",
				Text: ptr.Ptr("Artifact content here"),
			},
		},
	}

	content := ConvertA2AArtifactToContent(artifact)

	if content == nil {
		t.Fatal("Expected content, got nil")
	}

	if content.Role != "agent" {
		t.Errorf("Expected role 'agent', got '%s'", content.Role)
	}

	if len(content.Parts) != 1 {
		t.Errorf("Expected 1 part, got %d", len(content.Parts))
	}

	if content.Parts[0].Text == nil || *content.Parts[0].Text != "Artifact content here" {
		t.Errorf("Expected text 'Artifact content here', got %v", content.Parts[0].Text)
	}
}

func TestConvertWithFileContent(t *testing.T) {
	// Test file part conversion
	a2aPart := Part{
		Type: "file",
		File: &FileContent{
			Name:     ptr.Ptr("test.txt"),
			MimeType: ptr.Ptr("text/plain"),
			URI:      ptr.Ptr("https://example.com/test.txt"),
		},
	}

	corePart := ConvertA2APartToCorePart(a2aPart)

	if corePart == nil {
		t.Fatal("Expected core part, got nil")
	}

	if corePart.Type != "file" {
		t.Errorf("Expected type 'file', got '%s'", corePart.Type)
	}

	if corePart.Metadata == nil {
		t.Fatal("Expected metadata, got nil")
	}

	fileName, ok := corePart.Metadata["file_name"].(*string)
	if !ok || fileName == nil || *fileName != "test.txt" {
		t.Errorf("Expected file_name 'test.txt', got %v", fileName)
	}
}

func TestConvertWithFunctionCall(t *testing.T) {
	// Test function call conversion
	corePart := core.Part{
		Type: "function_call",
		FunctionCall: &core.FunctionCall{
			Name: "test_function",
			Args: map[string]any{"param": "value"},
		},
	}

	a2aPart := ConvertCorePartToA2APart(corePart)

	if a2aPart == nil {
		t.Fatal("Expected A2A part, got nil")
	}

	if a2aPart.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", a2aPart.Type)
	}

	if a2aPart.Text == nil {
		t.Fatal("Expected text, got nil")
	}

	// Should contain function call information in text representation
	if *a2aPart.Text == "" {
		t.Error("Expected non-empty text representation of function call")
	}

	if a2aPart.Metadata == nil {
		t.Fatal("Expected metadata, got nil")
	}

	if originalType, ok := a2aPart.Metadata["original_type"].(string); !ok || originalType != "function_call" {
		t.Errorf("Expected original_type 'function_call', got %v", originalType)
	}
}

func TestExtractTextFromParts(t *testing.T) {
	// Test A2A parts text extraction
	a2aParts := []Part{
		{Type: "data", Data: map[string]any{"test": "value"}},
		{Type: "text", Text: ptr.Ptr("Hello from A2A")},
	}

	text := ExtractTextFromA2AParts(a2aParts)
	if text != "Hello from A2A" {
		t.Errorf("Expected 'Hello from A2A', got '%s'", text)
	}

	// Test core parts text extraction
	coreParts := []core.Part{
		{Type: "function_call", FunctionCall: &core.FunctionCall{Name: "test"}},
		{Type: "text", Text: ptr.Ptr("Hello from Core")},
	}

	text = ExtractTextFromCoreParts(coreParts)
	if text != "Hello from Core" {
		t.Errorf("Expected 'Hello from Core', got '%s'", text)
	}
}

func TestCreateSimpleFunctions(t *testing.T) {
	// Test creating simple A2A message
	message := CreateSimpleTextA2AMessage("test123", "user", "Test message")

	if message.MessageID != "test123" {
		t.Errorf("Expected messageID 'test123', got '%s'", message.MessageID)
	}

	if message.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", message.Role)
	}

	if len(message.Parts) != 1 || message.Parts[0].Text == nil || *message.Parts[0].Text != "Test message" {
		t.Error("Expected single text part with 'Test message'")
	}

	// Test creating simple core content
	content := CreateSimpleTextCoreContent("agent", "Test response")

	if content.Role != "agent" {
		t.Errorf("Expected role 'agent', got '%s'", content.Role)
	}

	if len(content.Parts) != 1 || content.Parts[0].Text == nil || *content.Parts[0].Text != "Test response" {
		t.Error("Expected single text part with 'Test response'")
	}
}
