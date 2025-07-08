package tools

import (
	"context"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// Mock agent for testing EnhancedAgentTool
type mockAgent struct {
	name            string
	description     string
	instruction     string
	simulateError   bool
	simulateTimeout bool
	responseText    string
	responseDelay   time.Duration
}

func (m *mockAgent) Name() string                                             { return m.name }
func (m *mockAgent) Description() string                                      { return m.description }
func (m *mockAgent) Instruction() string                                      { return m.instruction }
func (m *mockAgent) SubAgents() []core.BaseAgent                              { return nil }
func (m *mockAgent) ParentAgent() core.BaseAgent                              { return nil }
func (m *mockAgent) SetParentAgent(parent core.BaseAgent)                     {}
func (m *mockAgent) FindAgent(name string) core.BaseAgent                     { return nil }
func (m *mockAgent) FindSubAgent(name string) core.BaseAgent                  { return nil }
func (m *mockAgent) GetBeforeAgentCallback() core.BeforeAgentCallback         { return nil }
func (m *mockAgent) SetBeforeAgentCallback(callback core.BeforeAgentCallback) {}
func (m *mockAgent) GetAfterAgentCallback() core.AfterAgentCallback           { return nil }
func (m *mockAgent) SetAfterAgentCallback(callback core.AfterAgentCallback)   {}
func (m *mockAgent) Cleanup(ctx context.Context) error                        { return nil }

func (m *mockAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 2)

	go func() {
		defer close(eventChan)

		// Check for immediate cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Simulate delay if configured
		if m.responseDelay > 0 {
			select {
			case <-time.After(m.responseDelay):
			case <-ctx.Done():
				return // Respect context cancellation
			}
		}

		// Check for timeout simulation
		if m.simulateTimeout {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
		}

		// Check context again before creating response
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Create response event
		event := core.NewEvent(invocationCtx.InvocationID, m.name)

		if m.simulateError {
			errorMsg := "simulated agent error"
			event.ErrorMessage = &errorMsg
		} else {
			responseText := m.responseText
			if responseText == "" {
				responseText = "Mock agent response"
			}
			event.Content = &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{
						Type: "text",
						Text: &responseText,
					},
				},
			}
		}

		// Simulate some state changes
		event.Actions.StateDelta = map[string]any{
			"last_agent_call": m.name,
			"call_count":      1,
		}

		select {
		case eventChan <- event:
		case <-ctx.Done():
			return
		}
	}()

	return eventChan, nil
}

func (m *mockAgent) Run(ctx context.Context, invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	stream, err := m.RunAsync(ctx, invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range stream {
		events = append(events, event)
	}

	return events, nil
}

// TestGoogleSearchTool tests the Google Search tool
func TestGoogleSearchTool(t *testing.T) {
	tool := NewGoogleSearchTool()

	// Test basic properties
	if tool.Name() != "google_search" {
		t.Errorf("Expected name 'google_search', got %s", tool.Name())
	}

	if tool.GetDeclaration() != nil {
		t.Error("Google Search tool should return nil declaration")
	}

	// Test RunAsync should return error (not meant to be executed locally)
	ctx := context.Background()
	toolCtx := &core.ToolContext{}
	_, err := tool.RunAsync(ctx, map[string]any{}, toolCtx)
	if err == nil {
		t.Error("Expected error when calling RunAsync on Google Search tool")
	}
}

func TestGoogleSearchTool_ProcessLLMRequest(t *testing.T) {
	tool := NewGoogleSearchTool()
	ctx := context.Background()
	toolCtx := &core.ToolContext{}

	tests := []struct {
		name          string
		model         string
		existingTools []*core.FunctionDeclaration
		expectError   bool
		errorContains string
	}{
		{
			name:        "Valid Gemini 2.x model",
			model:       "gemini-2.0-flash",
			expectError: false,
		},
		{
			name:        "Valid Gemini 1.x model without tools",
			model:       "gemini-1.5-pro",
			expectError: false,
		},
		{
			name:  "Gemini 1.x with existing tools should fail",
			model: "gemini-1.5-pro",
			existingTools: []*core.FunctionDeclaration{
				{Name: "other_tool", Description: "test"},
			},
			expectError:   true,
			errorContains: "cannot be used with other tools",
		},
		{
			name:          "Non-Gemini model should fail",
			model:         "gpt-4",
			expectError:   true,
			errorContains: "only supported for Gemini models",
		},
		{
			name:          "Empty model should fail",
			model:         "",
			expectError:   true,
			errorContains: "model name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &core.LLMRequest{
				Config: &core.LLMConfig{
					Model: tt.model,
					Tools: tt.existingTools,
				},
			}

			err := tool.ProcessLLMRequest(ctx, toolCtx, request)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Check that a tool was added
				if len(request.Config.Tools) == 0 {
					t.Error("Expected tools to be added to request")
				}
			}
		})
	}
}

// TestEnhancedAgentTool tests the enhanced agent tool
func TestEnhancedAgentTool(t *testing.T) {
	mockAgent := &mockAgent{
		name:         "test_agent",
		description:  "Test agent for testing",
		responseText: "Hello from test agent",
	}

	tool := NewEnhancedAgentTool(mockAgent)

	// Test basic properties
	expectedName := "agent_test_agent"
	if tool.Name() != expectedName {
		t.Errorf("Expected name '%s', got %s", expectedName, tool.Name())
	}

	// Test declaration
	decl := tool.GetDeclaration()
	if decl == nil {
		t.Fatal("Expected non-nil declaration")
	}

	if decl.Name != expectedName {
		t.Errorf("Expected declaration name '%s', got %s", expectedName, decl.Name)
	}

	// Check required parameters
	params, ok := decl.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected properties in parameters")
	}

	if _, hasRequest := params["request"]; !hasRequest {
		t.Error("Expected 'request' parameter in declaration")
	}
}

func TestEnhancedAgentTool_RunAsync(t *testing.T) {
	mockAgent := &mockAgent{
		name:         "test_agent",
		description:  "Test agent",
		responseText: "Agent response text",
	}

	tool := NewEnhancedAgentTool(mockAgent)
	ctx := context.Background()

	// Create test context
	session := &core.Session{
		ID:      "test_session",
		AppName: "test_app",
		UserID:  "test_user",
		State:   make(map[string]any),
	}

	invocationCtx := core.NewInvocationContext(
		"test_invocation",
		mockAgent,
		session,
		nil, // session service
	)

	state := core.NewState()
	toolCtx := &core.ToolContext{
		InvocationContext: invocationCtx,
		State:             state,
		Actions:           &core.EventActions{},
	}

	// Test successful execution
	t.Run("Successful execution", func(t *testing.T) {
		args := map[string]any{
			"request": "Test request",
		}

		result, err := tool.RunAsync(ctx, args, toolCtx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Errorf("Expected string result, got %T", result)
		}

		if resultStr != "Agent response text" {
			t.Errorf("Expected 'Agent response text', got '%s'", resultStr)
		}

		// Check that state was updated in the tool context state
		if val, exists := state.Get("last_agent_call"); !exists || val != "test_agent" {
			t.Error("Expected state to be updated with agent call")
		}
	})

	// Test with additional context
	t.Run("With additional context", func(t *testing.T) {
		args := map[string]any{
			"request": "Test request",
			"context": "Additional context info",
		}

		_, err := tool.RunAsync(ctx, args, toolCtx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	// Test missing request parameter
	t.Run("Missing request parameter", func(t *testing.T) {
		args := map[string]any{}

		_, err := tool.RunAsync(ctx, args, toolCtx)
		if err == nil {
			t.Error("Expected error for missing request parameter")
		}
	})
}

func TestEnhancedAgentTool_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		errorStrategy ErrorStrategy
		simulateError bool
		expectError   bool
		expectEmpty   bool
	}{
		{
			name:          "Propagate error strategy",
			errorStrategy: ErrorStrategyPropagate,
			simulateError: true,
			expectError:   true,
		},
		{
			name:          "Return error strategy",
			errorStrategy: ErrorStrategyReturnError,
			simulateError: true,
			expectError:   false,
		},
		{
			name:          "Return empty strategy",
			errorStrategy: ErrorStrategyReturnEmpty,
			simulateError: true,
			expectError:   false,
			expectEmpty:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgent := &mockAgent{
				name:          "error_agent",
				description:   "Error testing agent",
				simulateError: tt.simulateError,
			}

			config := DefaultAgentToolConfig()
			config.ErrorStrategy = tt.errorStrategy
			tool := NewEnhancedAgentToolWithConfig(mockAgent, config)

			ctx := context.Background()
			session := &core.Session{
				ID:      "test_session",
				AppName: "test_app",
				UserID:  "test_user",
				State:   make(map[string]any),
			}

			invocationCtx := core.NewInvocationContext(
				"test_invocation",
				mockAgent,
				session,
				nil,
			)

			state := core.NewState()
			toolCtx := &core.ToolContext{
				InvocationContext: invocationCtx,
				State:             state,
				Actions:           &core.EventActions{},
			}

			args := map[string]any{
				"request": "Test request",
			}

			result, err := tool.RunAsync(ctx, args, toolCtx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("Expected empty result, got: %v", result)
				}
			}
		})
	}
}

func TestEnhancedAgentTool_Timeout(t *testing.T) {
	mockAgent := &mockAgent{
		name:          "timeout_agent",
		description:   "Timeout testing agent",
		responseDelay: 200 * time.Millisecond, // Use response delay instead of simulate timeout
	}

	config := DefaultAgentToolConfig()
	config.Timeout = 50 * time.Millisecond // Very short timeout, less than response delay
	tool := NewEnhancedAgentToolWithConfig(mockAgent, config)

	ctx := context.Background()
	session := &core.Session{
		ID:      "test_session",
		AppName: "test_app",
		UserID:  "test_user",
		State:   make(map[string]any),
	}

	invocationCtx := core.NewInvocationContext(
		"test_invocation",
		mockAgent,
		session,
		nil,
	)

	state := core.NewState()
	toolCtx := &core.ToolContext{
		InvocationContext: invocationCtx,
		State:             state,
		Actions:           &core.EventActions{},
	}

	args := map[string]any{
		"request": "Test request",
	}

	start := time.Now()
	_, err := tool.RunAsync(ctx, args, toolCtx)
	duration := time.Since(start)

	// Should timeout quickly (within ~100ms including some overhead)
	if duration > 150*time.Millisecond {
		t.Errorf("Expected quick timeout, took %v", duration)
	}

	// The timeout should result in a context cancellation
	// With default error strategy (propagate), we expect an error
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
