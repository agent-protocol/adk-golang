// Package agents provides enhanced LLM agent implementation with comprehensive tool execution.
package agents

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// LlmAgentConfig contains configuration options for LLM agents.
type LlmAgentConfig struct {
	Model             string        `json:"model"`
	Temperature       *float32      `json:"temperature,omitempty"`
	MaxTokens         *int          `json:"max_tokens,omitempty"`
	TopP              *float32      `json:"top_p,omitempty"`
	TopK              *int          `json:"top_k,omitempty"`
	SystemInstruction *string       `json:"system_instruction,omitempty"`
	MaxToolCalls      int           `json:"max_tool_calls,omitempty"`
	ToolCallTimeout   time.Duration `json:"tool_call_timeout,omitempty"`
	RetryAttempts     int           `json:"retry_attempts,omitempty"`
	StreamingEnabled  bool          `json:"streaming_enabled,omitempty"`
}

// DefaultLlmAgentConfig returns a default configuration for LLM agents.
func DefaultLlmAgentConfig() *LlmAgentConfig {
	return &LlmAgentConfig{
		Model:            "gemini-1.5-pro",
		Temperature:      llmFloatPtr(0.7),
		MaxTokens:        llmIntPtr(4096),
		MaxToolCalls:     10,
		ToolCallTimeout:  30 * time.Second,
		RetryAttempts:    3,
		StreamingEnabled: false,
	}
}

// LlmAgentCallbacks contains callback functions for LLM agent lifecycle events.
type LlmAgentCallbacks struct {
	BeforeModelCallback core.BeforeAgentCallback
	AfterModelCallback  core.AfterAgentCallback
	BeforeToolCallback  core.BeforeAgentCallback
	AfterToolCallback   core.AfterAgentCallback
}

// EnhancedLlmAgent is an enhanced implementation of an LLM-based agent with comprehensive tool execution.
type EnhancedLlmAgent struct {
	*BaseAgentImpl
	config        *LlmAgentConfig
	tools         []core.BaseTool
	toolMap       map[string]core.BaseTool
	inputSchema   interface{}
	outputSchema  interface{}
	llmConnection core.LLMConnection
	callbacks     *LlmAgentCallbacks
}

// NewEnhancedLlmAgent creates a new enhanced LLM agent with the specified configuration.
func NewEnhancedLlmAgent(name, description string, config *LlmAgentConfig) *EnhancedLlmAgent {
	if config == nil {
		config = DefaultLlmAgentConfig()
	}

	agent := &EnhancedLlmAgent{
		BaseAgentImpl: NewBaseAgent(name, description),
		config:        config,
		tools:         make([]core.BaseTool, 0),
		toolMap:       make(map[string]core.BaseTool),
		callbacks:     &LlmAgentCallbacks{},
	}

	// Set system instruction if provided in config
	if config.SystemInstruction != nil {
		agent.SetInstruction(*config.SystemInstruction)
	}

	return agent
}

// Config returns the agent's configuration.
func (a *EnhancedLlmAgent) Config() *LlmAgentConfig {
	return a.config
}

// SetConfig updates the agent's configuration.
func (a *EnhancedLlmAgent) SetConfig(config *LlmAgentConfig) {
	a.config = config
	if config.SystemInstruction != nil {
		a.SetInstruction(*config.SystemInstruction)
	}
}

// Model returns the LLM model name.
func (a *EnhancedLlmAgent) Model() string {
	return a.config.Model
}

// SetModel sets the LLM model name.
func (a *EnhancedLlmAgent) SetModel(model string) {
	a.config.Model = model
}

// Tools returns the available tools for this agent.
func (a *EnhancedLlmAgent) Tools() []core.BaseTool {
	return a.tools
}

// AddTool adds a tool to this agent.
func (a *EnhancedLlmAgent) AddTool(tool core.BaseTool) {
	a.tools = append(a.tools, tool)
	a.toolMap[tool.Name()] = tool
}

// RemoveTool removes a tool from this agent.
func (a *EnhancedLlmAgent) RemoveTool(toolName string) bool {
	if _, exists := a.toolMap[toolName]; !exists {
		return false
	}

	delete(a.toolMap, toolName)

	// Remove from slice
	for i, tool := range a.tools {
		if tool.Name() == toolName {
			a.tools = append(a.tools[:i], a.tools[i+1:]...)
			break
		}
	}

	return true
}

// GetTool retrieves a tool by name.
func (a *EnhancedLlmAgent) GetTool(name string) (core.BaseTool, bool) {
	tool, exists := a.toolMap[name]
	return tool, exists
}

// SetLLMConnection sets the LLM connection for this agent.
func (a *EnhancedLlmAgent) SetLLMConnection(conn core.LLMConnection) {
	a.llmConnection = conn
}

// SetCallbacks sets the callback functions for this agent.
func (a *EnhancedLlmAgent) SetCallbacks(callbacks *LlmAgentCallbacks) {
	a.callbacks = callbacks
}

// SetInputSchema sets the input validation schema.
func (a *EnhancedLlmAgent) SetInputSchema(schema interface{}) {
	a.inputSchema = schema
}

// SetOutputSchema sets the output validation schema.
func (a *EnhancedLlmAgent) SetOutputSchema(schema interface{}) {
	a.outputSchema = schema
}

// RunAsync executes the LLM agent with comprehensive tool execution pipeline.
func (a *EnhancedLlmAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	if a.llmConnection == nil {
		return nil, fmt.Errorf("LLM connection not configured for agent %s", a.name)
	}

	// Execute before-agent callback if present
	if a.beforeAgentCallback != nil {
		if err := a.beforeAgentCallback(ctx, invocationCtx); err != nil {
			return nil, fmt.Errorf("before-agent callback failed: %w", err)
		}
	}

	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		if err := a.executeConversationFlow(ctx, invocationCtx, eventChan); err != nil {
			// Send error event
			errorEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
			errorEvent.ErrorMessage = stringPtr(fmt.Sprintf("Conversation flow failed: %v", err))

			select {
			case eventChan <- errorEvent:
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

// executeConversationFlow manages the complete conversation flow including tool execution.
func (a *EnhancedLlmAgent) executeConversationFlow(ctx context.Context, invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	maxTurns := 10 // Default max turns to prevent infinite loops
	if invocationCtx.RunConfig != nil && invocationCtx.RunConfig.MaxTurns != nil {
		maxTurns = *invocationCtx.RunConfig.MaxTurns
	}

	for turn := 0; turn < maxTurns; turn++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Build LLM request from conversation history
		request, err := a.buildLLMRequest(invocationCtx)
		if err != nil {
			return fmt.Errorf("failed to build LLM request: %w", err)
		}

		// Execute before-model callback
		if a.callbacks.BeforeModelCallback != nil {
			if err := a.callbacks.BeforeModelCallback(ctx, invocationCtx); err != nil {
				return fmt.Errorf("before-model callback failed: %w", err)
			}
		}

		// Make LLM call with retry logic
		response, err := a.makeRetriableLLMCall(ctx, request)
		if err != nil {
			return fmt.Errorf("LLM request failed: %w", err)
		}

		// Create event from LLM response
		event := core.NewEvent(invocationCtx.InvocationID, a.name)
		event.Content = response.Content

		// Execute after-model callback
		if a.callbacks.AfterModelCallback != nil {
			if err := a.callbacks.AfterModelCallback(ctx, invocationCtx, []*core.Event{event}); err != nil {
				return fmt.Errorf("after-model callback failed: %w", err)
			}
		}

		// Check for function calls
		functionCalls := event.GetFunctionCalls()
		if len(functionCalls) == 0 {
			// No tool calls - this is a final response
			event.TurnComplete = llmBoolPtr(true)

			select {
			case eventChan <- event:
			case <-ctx.Done():
				return ctx.Err()
			}

			// Add event to session for next iteration
			invocationCtx.Session.AddEvent(event)
			break
		}

		// Handle function calls
		if len(functionCalls) > a.config.MaxToolCalls {
			return fmt.Errorf("too many tool calls: %d (max: %d)", len(functionCalls), a.config.MaxToolCalls)
		}

		// Send the function call event first
		select {
		case eventChan <- event:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Add event to session for next iteration
		invocationCtx.Session.AddEvent(event)

		// Execute tools and collect responses
		toolResponses, err := a.executeToolCalls(ctx, invocationCtx, functionCalls, eventChan)
		if err != nil {
			return fmt.Errorf("tool execution failed: %w", err)
		}

		// Create tool response event
		responseEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
		responseEvent.Content = &core.Content{
			Role:  "agent",
			Parts: toolResponses,
		}

		select {
		case eventChan <- responseEvent:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Add tool response event to session
		invocationCtx.Session.AddEvent(responseEvent)

		// Continue the conversation loop to get the LLM's final response
	}

	return nil
}

// executeToolCalls executes all function calls and returns their responses.
func (a *EnhancedLlmAgent) executeToolCalls(ctx context.Context, invocationCtx *core.InvocationContext, functionCalls []*core.FunctionCall, eventChan chan<- *core.Event) ([]core.Part, error) {
	toolResponses := make([]core.Part, 0, len(functionCalls))

	for _, funcCall := range functionCalls {
		// Find the tool
		tool, exists := a.toolMap[funcCall.Name]
		if !exists {
			// Return error response for unknown tool
			errorResponse := &core.FunctionResponse{
				ID:   funcCall.ID,
				Name: funcCall.Name,
				Response: map[string]any{
					"error": fmt.Sprintf("Unknown tool: %s", funcCall.Name),
				},
			}

			toolResponses = append(toolResponses, core.Part{
				Type:             "function_response",
				FunctionResponse: errorResponse,
			})
			continue
		}

		// Execute before-tool callback
		if a.callbacks.BeforeToolCallback != nil {
			if err := a.callbacks.BeforeToolCallback(ctx, invocationCtx); err != nil {
				log.Printf("Before-tool callback failed: %v", err)
				// Continue execution but log the error
			}
		}

		// Create tool context
		toolCtx := core.NewToolContext(invocationCtx)
		toolCtx.FunctionCallID = &funcCall.ID

		// Execute tool with timeout
		toolCtx.InvocationContext = invocationCtx
		result, err := a.executeToolWithTimeout(ctx, tool, funcCall.Args, toolCtx)

		// Execute after-tool callback
		if a.callbacks.AfterToolCallback != nil {
			if err := a.callbacks.AfterToolCallback(ctx, invocationCtx, []*core.Event{}); err != nil {
				log.Printf("After-tool callback failed: %v", err)
				// Continue execution but log the error
			}
		}

		// Build tool response
		var response *core.FunctionResponse
		if err != nil {
			response = &core.FunctionResponse{
				ID:   funcCall.ID,
				Name: funcCall.Name,
				Response: map[string]any{
					"error": err.Error(),
				},
			}
		} else {
			response = &core.FunctionResponse{
				ID:   funcCall.ID,
				Name: funcCall.Name,
				Response: map[string]any{
					"result": result,
				},
			}
		}

		toolResponses = append(toolResponses, core.Part{
			Type:             "function_response",
			FunctionResponse: response,
		})

		// Apply any state changes from the tool
		if len(toolCtx.Actions.StateDelta) > 0 {
			invocationCtx.Session.UpdateState(toolCtx.Actions.StateDelta)
		}

		// Handle long-running tools
		if tool.IsLongRunning() {
			// Create intermediate event for long-running tool
			longRunningEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
			longRunningEvent.LongRunningToolIDs = []string{funcCall.ID}
			longRunningEvent.Partial = llmBoolPtr(true)

			select {
			case eventChan <- longRunningEvent:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return toolResponses, nil
}

// executeToolWithTimeout executes a tool with the configured timeout.
func (a *EnhancedLlmAgent) executeToolWithTimeout(ctx context.Context, tool core.BaseTool, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.ToolCallTimeout)
	defer cancel()

	// Execute tool
	return tool.RunAsync(timeoutCtx, args, toolCtx)
}

// makeRetriableLLMCall makes an LLM call with retry logic.
func (a *EnhancedLlmAgent) makeRetriableLLMCall(ctx context.Context, request *core.LLMRequest) (*core.LLMResponse, error) {
	var lastErr error

	for attempt := 0; attempt < a.config.RetryAttempts; attempt++ {
		response, err := a.llmConnection.GenerateContent(ctx, request)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if this is a retryable error
		if !a.isRetryableError(err) {
			break
		}

		// Wait before retry (exponential backoff)
		if attempt < a.config.RetryAttempts-1 {
			waitTime := time.Duration(attempt+1) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
			}
		}
	}

	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", a.config.RetryAttempts, lastErr)
}

// isRetryableError determines if an error is retryable.
func (a *EnhancedLlmAgent) isRetryableError(err error) bool {
	// Simple implementation - can be enhanced based on specific error types
	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"temporary",
		"rate limit",
		"503",
		"502",
		"500",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// buildLLMRequest constructs an LLM request from the session context.
func (a *EnhancedLlmAgent) buildLLMRequest(invocationCtx *core.InvocationContext) (*core.LLMRequest, error) {
	contents := make([]core.Content, 0)

	// Add system instruction if present
	if a.instruction != "" {
		contents = append(contents, core.Content{
			Role: "system",
			Parts: []core.Part{
				{
					Type: "text",
					Text: &a.instruction,
				},
			},
		})
	}

	// Add session history (excluding system messages from history)
	for _, event := range invocationCtx.Session.Events {
		if event.Content != nil && event.Content.Role != "system" {
			contents = append(contents, *event.Content)
		}
	}

	// Add current user message if present
	if invocationCtx.UserContent != nil {
		contents = append(contents, *invocationCtx.UserContent)
	}

	// Build tool declarations
	var tools []*core.FunctionDeclaration
	for _, tool := range a.tools {
		if decl := tool.GetDeclaration(); decl != nil {
			tools = append(tools, decl)
		}
	}

	// Build LLM config
	llmConfig := &core.LLMConfig{
		Model:             a.config.Model,
		Temperature:       a.config.Temperature,
		MaxTokens:         a.config.MaxTokens,
		TopP:              a.config.TopP,
		TopK:              a.config.TopK,
		Tools:             tools,
		SystemInstruction: &a.instruction,
	}

	return &core.LLMRequest{
		Contents: contents,
		Config:   llmConfig,
		Tools:    tools,
	}, nil
}

// Run is a synchronous wrapper around RunAsync.
func (a *EnhancedLlmAgent) Run(ctx context.Context, invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	stream, err := a.RunAsync(ctx, invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range stream {
		events = append(events, event)
	}

	// Execute after-agent callback if present
	if a.afterAgentCallback != nil {
		if err := a.afterAgentCallback(ctx, invocationCtx, events); err != nil {
			return events, fmt.Errorf("after-agent callback failed: %w", err)
		}
	}

	return events, nil
}

// StreamingLlmAgent extends EnhancedLlmAgent with streaming capabilities.
type StreamingLlmAgent struct {
	*EnhancedLlmAgent
}

// NewStreamingLlmAgent creates a new streaming LLM agent.
func NewStreamingLlmAgent(name, description string, config *LlmAgentConfig) *StreamingLlmAgent {
	if config == nil {
		config = DefaultLlmAgentConfig()
	}
	config.StreamingEnabled = true

	return &StreamingLlmAgent{
		EnhancedLlmAgent: NewEnhancedLlmAgent(name, description, config),
	}
}

// RunAsync executes the streaming LLM agent with real-time response streaming.
func (a *StreamingLlmAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	if a.llmConnection == nil {
		return nil, fmt.Errorf("LLM connection not configured for agent %s", a.name)
	}

	// Execute before-agent callback if present
	if a.beforeAgentCallback != nil {
		if err := a.beforeAgentCallback(ctx, invocationCtx); err != nil {
			return nil, fmt.Errorf("before-agent callback failed: %w", err)
		}
	}

	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		if err := a.executeStreamingConversationFlow(ctx, invocationCtx, eventChan); err != nil {
			// Send error event
			errorEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
			errorEvent.ErrorMessage = stringPtr(fmt.Sprintf("Streaming conversation flow failed: %v", err))

			select {
			case eventChan <- errorEvent:
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

// executeStreamingConversationFlow manages streaming conversation flow.
func (a *StreamingLlmAgent) executeStreamingConversationFlow(ctx context.Context, invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	// Build LLM request
	request, err := a.buildLLMRequest(invocationCtx)
	if err != nil {
		return fmt.Errorf("failed to build LLM request: %w", err)
	}

	// Execute before-model callback
	if a.callbacks.BeforeModelCallback != nil {
		if err := a.callbacks.BeforeModelCallback(ctx, invocationCtx); err != nil {
			return fmt.Errorf("before-model callback failed: %w", err)
		}
	}

	// Make streaming LLM call
	responseStream, err := a.llmConnection.GenerateContentStream(ctx, request)
	if err != nil {
		return fmt.Errorf("streaming LLM request failed: %w", err)
	}

	var accumulatedContent *core.Content
	var finalEvent *core.Event

	// Process streaming responses
	for response := range responseStream {
		// Create event from streaming response
		event := core.NewEvent(invocationCtx.InvocationID, a.name)
		event.Content = response.Content
		event.Partial = response.Partial

		// Accumulate content for final processing
		if accumulatedContent == nil {
			accumulatedContent = response.Content
		} else {
			// Merge content parts
			if response.Content != nil {
				accumulatedContent.Parts = append(accumulatedContent.Parts, response.Content.Parts...)
			}
		}

		// Send streaming event
		select {
		case eventChan <- event:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Check if this is the final response
		if response.Partial == nil || !*response.Partial {
			finalEvent = event
			break
		}
	}

	// Execute after-model callback
	if a.callbacks.AfterModelCallback != nil {
		if err := a.callbacks.AfterModelCallback(ctx, invocationCtx, []*core.Event{finalEvent}); err != nil {
			return fmt.Errorf("after-model callback failed: %w", err)
		}
	}

	// Process any function calls in the final response
	if finalEvent != nil {
		functionCalls := finalEvent.GetFunctionCalls()
		if len(functionCalls) > 0 {
			// Execute tools
			toolResponses, err := a.executeToolCalls(ctx, invocationCtx, functionCalls, eventChan)
			if err != nil {
				return fmt.Errorf("tool execution failed: %w", err)
			}

			// Create tool response event
			responseEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
			responseEvent.Content = &core.Content{
				Role:  "agent",
				Parts: toolResponses,
			}

			select {
			case eventChan <- responseEvent:
			case <-ctx.Done():
				return ctx.Err()
			}

			// Add events to session
			invocationCtx.Session.AddEvent(finalEvent)
			invocationCtx.Session.AddEvent(responseEvent)

			// Continue conversation to get final response after tool execution
			return a.executeStreamingConversationFlow(ctx, invocationCtx, eventChan)
		}

		// Add final event to session
		invocationCtx.Session.AddEvent(finalEvent)
	}

	return nil
}

// Helper functions for LlmAgent

func llmIntPtr(i int) *int {
	return &i
}

func llmBoolPtr(b bool) *bool {
	return &b
}

func llmFloatPtr(f float32) *float32 {
	return &f
}
