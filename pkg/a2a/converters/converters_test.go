package converters

import (
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

func TestConvertA2ARequestToADKRunArgs(t *testing.T) {
	// Create test A2A request context
	requestCtx := &RequestContext{
		TaskID:    "task-123",
		ContextID: "ctx-456",
		Message: &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr("Hello, agent!"),
				},
				{
					Type: "data",
					Data: map[string]any{
						"name": "test_function",
						"args": map[string]any{
							"param1": "value1",
						},
					},
					Metadata: map[string]any{
						"adk:type": "function_call",
					},
				},
			},
		},
		UserID: "user-123",
	}

	// Convert to ADK run args
	runArgs, err := ConvertA2ARequestToADKRunArgs(requestCtx)
	if err != nil {
		t.Fatalf("ConvertA2ARequestToADKRunArgs failed: %v", err)
	}

	// Verify the conversion
	if runArgs.UserID != "user-123" {
		t.Errorf("Expected UserID 'user-123', got '%s'", runArgs.UserID)
	}

	if runArgs.SessionID != "ctx-456" {
		t.Errorf("Expected SessionID 'ctx-456', got '%s'", runArgs.SessionID)
	}

	if runArgs.NewMessage == nil {
		t.Fatal("NewMessage should not be nil")
	}

	if runArgs.NewMessage.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", runArgs.NewMessage.Role)
	}

	if len(runArgs.NewMessage.Parts) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(runArgs.NewMessage.Parts))
	}

	// Check text part
	textPart := runArgs.NewMessage.Parts[0]
	if textPart.Type != "text" {
		t.Errorf("Expected first part type 'text', got '%s'", textPart.Type)
	}
	if textPart.Text == nil || *textPart.Text != "Hello, agent!" {
		t.Errorf("Expected text 'Hello, agent!', got %v", textPart.Text)
	}

	// Check function call part
	funcPart := runArgs.NewMessage.Parts[1]
	if funcPart.Type != "function_call" {
		t.Errorf("Expected second part type 'function_call', got '%s'", funcPart.Type)
	}
	if funcPart.FunctionCall == nil {
		t.Fatal("FunctionCall should not be nil")
	}
	if funcPart.FunctionCall.Name != "test_function" {
		t.Errorf("Expected function name 'test_function', got '%s'", funcPart.FunctionCall.Name)
	}
}

func TestConvertEventToA2AEvents(t *testing.T) {
	// Create test ADK event
	adkEvent := &core.Event{
		ID:           "event-123",
		InvocationID: "inv-456",
		Author:       "test-agent",
		Content: &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Hello from agent"),
				},
				{
					Type: "function_call",
					FunctionCall: &core.FunctionCall{
						ID:   "call-123",
						Name: "test_tool",
						Args: map[string]any{
							"input": "test",
						},
					},
				},
			},
		},
		Timestamp:          time.Now(),
		LongRunningToolIDs: []string{"call-123"},
		Actions:            core.EventActions{},
	}

	// Create test session
	session := &core.Session{
		ID:      "session-123",
		UserID:  "user-456",
		AppName: "test-app",
	}

	// Convert to A2A events
	a2aEvents, err := ConvertEventToA2AEvents(adkEvent, session, "task-789", "ctx-012")
	if err != nil {
		t.Fatalf("ConvertEventToA2AEvents failed: %v", err)
	}

	if len(a2aEvents) == 0 {
		t.Fatal("Expected at least one A2A event")
	}

	// Check that we got a status update event
	statusEvent, ok := a2aEvents[0].(*a2a.TaskStatusUpdateEvent)
	if !ok {
		t.Fatalf("Expected TaskStatusUpdateEvent, got %T", a2aEvents[0])
	}

	if statusEvent.ID != "task-789" {
		t.Errorf("Expected task ID 'task-789', got '%s'", statusEvent.ID)
	}

	if statusEvent.Status.State != a2a.TaskStateInputRequired {
		t.Errorf("Expected state %s (due to long-running tool), got %s", a2a.TaskStateInputRequired, statusEvent.Status.State)
	}

	if statusEvent.Status.Message == nil {
		t.Fatal("Status message should not be nil")
	}

	if len(statusEvent.Status.Message.Parts) != 2 {
		t.Fatalf("Expected 2 message parts, got %d", len(statusEvent.Status.Message.Parts))
	}
}

func TestConvertEventToA2AMessage(t *testing.T) {
	// Create test ADK event
	adkEvent := &core.Event{
		ID:           "event-123",
		InvocationID: "inv-456",
		Author:       "test-agent",
		Content: &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Hello from agent"),
				},
				{
					Type: "function_response",
					FunctionResponse: &core.FunctionResponse{
						ID:   "call-123",
						Name: "test_tool",
						Response: map[string]any{
							"result": "success",
						},
					},
				},
			},
		},
		Timestamp: time.Now(),
		Actions:   core.EventActions{},
	}

	// Convert to A2A message
	message, err := ConvertEventToA2AMessage(adkEvent)
	if err != nil {
		t.Fatalf("ConvertEventToA2AMessage failed: %v", err)
	}

	if message == nil {
		t.Fatal("Message should not be nil")
	}

	if message.Role != "agent" {
		t.Errorf("Expected role 'agent', got '%s'", message.Role)
	}

	if len(message.Parts) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(message.Parts))
	}

	// Check text part
	textPart := message.Parts[0]
	if textPart.Type != "text" {
		t.Errorf("Expected first part type 'text', got '%s'", textPart.Type)
	}
	if textPart.Text == nil || *textPart.Text != "Hello from agent" {
		t.Errorf("Expected text 'Hello from agent', got %v", textPart.Text)
	}

	// Check function response part
	responsePart := message.Parts[1]
	if responsePart.Type != "data" {
		t.Errorf("Expected second part type 'data', got '%s'", responsePart.Type)
	}
	if responsePart.Data == nil {
		t.Fatal("Data should not be nil for function response")
	}
	if responsePart.Data["name"] != "test_tool" {
		t.Errorf("Expected function name 'test_tool', got %v", responsePart.Data["name"])
	}

	// Check metadata indicates function response
	if responsePart.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if responsePart.Metadata["adk:type"] != "function_response" {
		t.Errorf("Expected metadata type 'function_response', got %v", responsePart.Metadata["adk:type"])
	}
}

func TestConvertA2APartToADKPart_TextPart(t *testing.T) {
	a2aPart := a2a.Part{
		Type: "text",
		Text: stringPtr("Hello world"),
		Metadata: map[string]any{
			"test": "value",
		},
	}

	adkPart, err := convertA2APartToADKPart(a2aPart)
	if err != nil {
		t.Fatalf("convertA2APartToADKPart failed: %v", err)
	}

	if adkPart.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", adkPart.Type)
	}
	if adkPart.Text == nil || *adkPart.Text != "Hello world" {
		t.Errorf("Expected text 'Hello world', got %v", adkPart.Text)
	}
	if adkPart.Metadata["test"] != "value" {
		t.Errorf("Expected metadata test='value', got %v", adkPart.Metadata["test"])
	}
}

func TestConvertA2APartToADKPart_FunctionCall(t *testing.T) {
	a2aPart := a2a.Part{
		Type: "data",
		Data: map[string]any{
			"name": "test_function",
			"args": map[string]any{
				"param1": "value1",
				"param2": 42,
			},
			"id": "call-123",
		},
		Metadata: map[string]any{
			"adk:type": "function_call",
		},
	}

	adkPart, err := convertA2APartToADKPart(a2aPart)
	if err != nil {
		t.Fatalf("convertA2APartToADKPart failed: %v", err)
	}

	if adkPart.Type != "function_call" {
		t.Errorf("Expected type 'function_call', got '%s'", adkPart.Type)
	}
	if adkPart.FunctionCall == nil {
		t.Fatal("FunctionCall should not be nil")
	}
	if adkPart.FunctionCall.Name != "test_function" {
		t.Errorf("Expected name 'test_function', got '%s'", adkPart.FunctionCall.Name)
	}
	if adkPart.FunctionCall.ID != "call-123" {
		t.Errorf("Expected ID 'call-123', got '%s'", adkPart.FunctionCall.ID)
	}
	if adkPart.FunctionCall.Args["param1"] != "value1" {
		t.Errorf("Expected param1='value1', got %v", adkPart.FunctionCall.Args["param1"])
	}
	if adkPart.FunctionCall.Args["param2"] != 42 {
		t.Errorf("Expected param2=42, got %v", adkPart.FunctionCall.Args["param2"])
	}
}

func TestIsA2AFunctionCall(t *testing.T) {
	// Test with metadata type indicator
	part1 := a2a.Part{
		Type: "data",
		Data: map[string]any{
			"name": "test",
			"args": map[string]any{},
		},
		Metadata: map[string]any{
			"adk:type": "function_call",
		},
	}
	if !isA2AFunctionCall(part1) {
		t.Error("Expected part1 to be identified as function call")
	}

	// Test with data structure
	part2 := a2a.Part{
		Type: "data",
		Data: map[string]any{
			"name": "test",
			"args": map[string]any{},
		},
	}
	if !isA2AFunctionCall(part2) {
		t.Error("Expected part2 to be identified as function call")
	}

	// Test negative case
	part3 := a2a.Part{
		Type: "data",
		Data: map[string]any{
			"content": "not a function call",
		},
	}
	if isA2AFunctionCall(part3) {
		t.Error("Expected part3 to NOT be identified as function call")
	}
}
