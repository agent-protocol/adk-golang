// Package agents provides enhanced LLM agent implementation with comprehensive tool execution.
//
// This package includes sophisticated loop detection mechanisms to prevent infinite loops
// during conversation flows. For detailed documentation on loop detection, see loop_detection.go.
//
// Key components:
//   - EnhancedLlmAgent: Main LLM agent with tool execution capabilities
//   - ConversationFlowManager: Orchestrates conversation flow and loop detection
//   - LoopDetector: Implements multiple loop detection strategies
//   - EventPublisher: Handles event creation and publishing
//
// The agents in this package follow SOLID principles and provide robust safeguards
// for production use, including configurable limits, graceful error handling,
// and comprehensive testing.
package agents

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// formatContent formats Content for logging, showing actual text instead of pointers
func formatContent(content *core.Content) string {
	if content == nil {
		return "<nil>"
	}

	parts := make([]string, 0, len(content.Parts))
	for i, part := range content.Parts {
		switch part.Type {
		case "text":
			if part.Text != nil {
				parts = append(parts, fmt.Sprintf("Part[%d]:text=%q", i, *part.Text))
			} else {
				parts = append(parts, fmt.Sprintf("Part[%d]:text=<nil>", i))
			}
		case "function_call":
			if part.FunctionCall != nil {
				parts = append(parts, fmt.Sprintf("Part[%d]:function_call={name=%s, args=%+v}", i, part.FunctionCall.Name, part.FunctionCall.Args))
			} else {
				parts = append(parts, fmt.Sprintf("Part[%d]:function_call=<nil>", i))
			}
		case "function_response":
			if part.FunctionResponse != nil {
				parts = append(parts, fmt.Sprintf("Part[%d]:function_response={name=%s, response=%+v}", i, part.FunctionResponse.Name, part.FunctionResponse.Response))
			} else {
				parts = append(parts, fmt.Sprintf("Part[%d]:function_response=<nil>", i))
			}
		default:
			parts = append(parts, fmt.Sprintf("Part[%d]:type=%s", i, part.Type))
		}
	}

	return fmt.Sprintf("Content{role=%s, parts=[%s]}", content.Role, strings.Join(parts, ", "))
}

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
		Temperature:      ptr.Float32(0.7),
		MaxTokens:        ptr.Ptr(4096),
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

// LLMAgent is an enhanced implementation of an LLM-based agent with comprehensive tool execution.
type LLMAgent struct {
	*CustomAgent
	config        *LlmAgentConfig
	tools         []core.BaseTool
	toolMap       map[string]core.BaseTool
	llmConnection core.LLMConnection
	callbacks     *LlmAgentCallbacks
}

// NewLLMAgent creates a new enhanced LLM agent with the specified configuration.
func NewLLMAgent(name, description string, config *LlmAgentConfig) *LLMAgent {
	if config == nil {
		config = DefaultLlmAgentConfig()
	}

	agent := &LLMAgent{
		CustomAgent: NewBaseAgent(name, description),
		config:      config,
		tools:       make([]core.BaseTool, 0),
		toolMap:     make(map[string]core.BaseTool),
		callbacks:   &LlmAgentCallbacks{},
	}

	// Set system instruction if provided in config
	if config.SystemInstruction != nil {
		agent.SetInstruction(*config.SystemInstruction)
	}

	return agent
}

// Config returns the agent's configuration.
func (a *LLMAgent) Config() *LlmAgentConfig {
	return a.config
}

// SetConfig updates the agent's configuration.
func (a *LLMAgent) SetConfig(config *LlmAgentConfig) {
	a.config = config
	if config.SystemInstruction != nil {
		a.SetInstruction(*config.SystemInstruction)
	}
}

// Model returns the LLM model name.
func (a *LLMAgent) Model() string {
	return a.config.Model
}

// SetModel sets the LLM model name.
func (a *LLMAgent) SetModel(model string) {
	a.config.Model = model
}

// Tools returns the available tools for this agent.
func (a *LLMAgent) Tools() []core.BaseTool {
	return a.tools
}

// AddTool adds a tool to this agent.
func (a *LLMAgent) AddTool(tool core.BaseTool) {
	a.tools = append(a.tools, tool)
	a.toolMap[tool.Name()] = tool
}

// RemoveTool removes a tool from this agent.
func (a *LLMAgent) RemoveTool(toolName string) bool {
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
func (a *LLMAgent) GetTool(name string) (core.BaseTool, bool) {
	tool, exists := a.toolMap[name]
	return tool, exists
}

// SetLLMConnection sets the LLM connection for this agent.
func (a *LLMAgent) SetLLMConnection(conn core.LLMConnection) {
	a.llmConnection = conn
}

// SetCallbacks sets the callback functions for this agent.
func (a *LLMAgent) SetCallbacks(callbacks *LlmAgentCallbacks) {
	a.callbacks = callbacks
}

// RunAsync executes the LLM agent with comprehensive tool execution pipeline.
func (a *LLMAgent) RunAsync(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	log.Printf("Starting RunAsync for agent: %s", a.name)
	if a.llmConnection == nil {
		log.Printf("LLM connection not configured for agent: %s", a.name)
		return nil, fmt.Errorf("LLM connection not configured for agent %s", a.name)
	}

	// Execute before-agent callback if present
	if a.beforeAgentCallback != nil {
		if err := a.beforeAgentCallback(invocationCtx); err != nil {
			return nil, fmt.Errorf("before-agent callback failed: %w", err)
		}
	}

	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		log.Println("Executing conversation flow...")
		if err := a.executeConversationFlow(invocationCtx, eventChan); err != nil {
			log.Printf("Conversation flow failed: %v", err)
			// Send error event
			errorEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
			errorEvent.ErrorMessage = ptr.Ptr(fmt.Sprintf("Conversation flow failed: %v", err))

			select {
			case eventChan <- errorEvent:
			case <-invocationCtx.Context.Done():
				return
			}
		}
	}()

	log.Println("RunAsync completed.")
	return eventChan, nil
}

// executeConversationFlow manages the complete conversation flow including tool execution.
func (a *LLMAgent) executeConversationFlow(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	log.Println("Starting conversation flow...")

	flowManager := NewConversationFlowManager(a, invocationCtx)

	for turn := 0; turn < flowManager.maxTurns; turn++ {
		// Check context cancellation
		select {
		case <-invocationCtx.Context.Done():
			return invocationCtx.Context.Err()
		default:
		}

		// Process LLM turn
		event, shouldContinue, err := a.processLLMTurn(invocationCtx, turn)
		if err != nil {
			return err
		}

		// Clear user content after first turn to prevent re-adding it to LLM requests
		if turn == 0 && invocationCtx.UserContent != nil {
			// Add user content to session as first event if not already present
			if len(invocationCtx.Session.Events) == 0 || invocationCtx.Session.Events[0].Content == nil || invocationCtx.Session.Events[0].Content.Role != "user" {
				userEvent := core.NewEvent(invocationCtx.InvocationID, "user")
				userEvent.Content = invocationCtx.UserContent
				invocationCtx.Session.AddEvent(userEvent)
			}
			// Clear it so it won't be added again in subsequent turns
			invocationCtx.UserContent = nil
		}

		if !shouldContinue {
			// Final response - publish and exit
			log.Printf("Publishing final response event: %s", formatContent(event.Content))
			if err := flowManager.eventPublisher.PublishEvent(invocationCtx, eventChan, event); err != nil {
				log.Printf("Failed to publish final event: %v", err)
				return err
			}
			invocationCtx.Session.AddEvent(event)
			log.Println("Final response published and added to session. Exiting conversation flow.")
			break
		}

		// Check for loop conditions
		functionCalls := event.GetFunctionCalls()
		if err := a.checkLoopConditions(invocationCtx, eventChan, flowManager, functionCalls, turn); err != nil {
			// Check if this is a graceful completion
			if _, isComplete := err.(ErrConversationComplete); isComplete {
				log.Printf("Conversation completed gracefully: %v", err)
				break // Exit gracefully without returning error
			}
			return err
		}

		// Process tool calls
		if err := a.processToolCalls(invocationCtx, eventChan, event, functionCalls); err != nil {
			return err
		}

		// Check for repeating patterns
		if flowManager.loopDetector.CheckRepeatingPattern(invocationCtx.Session.Events, turn) {
			log.Println("Detected repeating tool call pattern. Breaking out of loop.")

			finalEvent := flowManager.eventPublisher.CreateFinalResponse(
				invocationCtx.InvocationID,
				a.name,
				"I've completed the tool execution. Based on the results, I can provide you with the information you requested.",
			)

			if err := flowManager.eventPublisher.PublishEvent(invocationCtx, eventChan, finalEvent); err != nil {
				return err
			}
			invocationCtx.Session.AddEvent(finalEvent)
			break
		}
	}

	log.Println("Conversation flow completed.")
	return nil
}

// processLLMTurn processes a single LLM turn and returns the event and whether to continue
func (a *LLMAgent) processLLMTurn(invocationCtx *core.InvocationContext, turn int) (*core.Event, bool, error) {
	// Log user input if present
	if invocationCtx.UserContent != nil {
		log.Printf("User input: %s", formatContent(invocationCtx.UserContent))
	}

	// Build LLM request from conversation history
	log.Println("Building LLM request...")
	request, err := a.buildLLMRequest(invocationCtx)
	if err != nil {
		log.Printf("Failed to build LLM request: %v", err)
		return nil, false, fmt.Errorf("failed to build LLM request: %w", err)
	}

	// Execute before-model callback
	if a.callbacks.BeforeModelCallback != nil {
		if err := a.callbacks.BeforeModelCallback(invocationCtx); err != nil {
			return nil, false, fmt.Errorf("before-model callback failed: %w", err)
		}
	}

	// Make LLM call with retry logic
	log.Println("Making LLM call...")
	response, err := a.makeRetriableLLMCall(invocationCtx, request)
	if err != nil {
		log.Printf("LLM request failed: %v", err)
		return nil, false, fmt.Errorf("LLM request failed: %w", err)
	}

	log.Printf("LLM response content: %s", formatContent(response.Content))

	// Create event from LLM response
	event := core.NewEvent(invocationCtx.InvocationID, a.name)
	event.Content = response.Content

	// Execute after-model callback
	if a.callbacks.AfterModelCallback != nil {
		if err := a.callbacks.AfterModelCallback(invocationCtx, []*core.Event{event}); err != nil {
			return nil, false, fmt.Errorf("after-model callback failed: %w", err)
		}
	}

	// Check for function calls
	functionCalls := event.GetFunctionCalls()
	log.Printf("Found %d function calls in LLM response", len(functionCalls))
	if len(functionCalls) == 0 {
		// No tool calls - this is a final response
		log.Println("No function calls found - marking as final response")
		event.TurnComplete = ptr.Ptr(true)
		return event, false, nil
	}

	log.Println("Function calls found - continuing conversation flow")
	return event, true, nil
}

// ErrConversationComplete is a special error that indicates the conversation has completed gracefully
type ErrConversationComplete struct {
	Reason string
}

func (e ErrConversationComplete) Error() string {
	return e.Reason
}

// checkLoopConditions checks various loop conditions and handles them
func (a *LLMAgent) checkLoopConditions(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event, flowManager *ConversationFlowManager, functionCalls []*core.FunctionCall, turn int) error {
	// Check total tool calls limit to prevent infinite loops
	if flowManager.loopDetector.CheckToolCallLimit(functionCalls, flowManager.maxToolCalls) {
		log.Printf("Maximum total tool calls exceeded: %d (max: %d)", flowManager.loopDetector.totalToolCalls, flowManager.maxToolCalls)

		finalEvent := flowManager.eventPublisher.CreateFinalResponse(
			invocationCtx.InvocationID,
			a.name,
			"I've reached the maximum number of tool calls. Let me provide a direct response based on the information I have.",
		)

		if err := flowManager.eventPublisher.PublishEvent(invocationCtx, eventChan, finalEvent); err != nil {
			return err
		}
		invocationCtx.Session.AddEvent(finalEvent)
		// Return special error to indicate graceful completion
		return ErrConversationComplete{Reason: "conversation ended due to tool call limit"}
	}

	// Check per-turn tool calls limit
	if len(functionCalls) > a.config.MaxToolCalls {
		return fmt.Errorf("too many tool calls in single turn: %d (max: %d)", len(functionCalls), a.config.MaxToolCalls)
	}

	return nil
}

// processToolCalls processes tool calls and publishes events
func (a *LLMAgent) processToolCalls(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event, event *core.Event, functionCalls []*core.FunctionCall) error {
	// Validate function call arguments (allow empty args for no-parameter functions)
	for _, funcCall := range functionCalls {
		if funcCall.Args == nil {
			// Initialize empty args map for functions with no parameters
			funcCall.Args = make(map[string]interface{})
		}
		log.Printf("Function call: %s with args: %+v", funcCall.Name, funcCall.Args)
	}

	// Send the function call event first
	select {
	case eventChan <- event:
	case <-invocationCtx.Done():
		return invocationCtx.Err()
	}

	// Add event to session for next iteration
	invocationCtx.Session.AddEvent(event)

	// Execute tools and collect responses
	toolResponses, err := a.executeToolCalls(invocationCtx, functionCalls, eventChan)
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
	case <-invocationCtx.Done():
		return invocationCtx.Err()
	}

	// Add tool response event to session
	invocationCtx.Session.AddEvent(responseEvent)

	// Log session state for debugging
	log.Printf("Session now has %d events", len(invocationCtx.Session.Events))

	return nil
}

// executeToolCalls executes all function calls and returns their responses.
func (a *LLMAgent) executeToolCalls(invocationCtx *core.InvocationContext, functionCalls []*core.FunctionCall, eventChan chan<- *core.Event) ([]core.Part, error) {
	log.Println("Starting tool execution...")

	// Execute before-tool callback
	if a.callbacks.BeforeToolCallback != nil {
		if err := a.callbacks.BeforeToolCallback(invocationCtx); err != nil {
			return nil, fmt.Errorf("before-tool callback failed: %w", err)
		}
	}

	toolResponses := make([]core.Part, 0, len(functionCalls))

	for _, funcCall := range functionCalls {
		log.Printf("Processing function call: %s", funcCall.Name)
		tool, exists := a.toolMap[funcCall.Name]
		if !exists {
			log.Printf("Unknown tool: %s", funcCall.Name)
			// Return error response for unknown tool
			toolResponses = append(toolResponses, core.Part{
				Type: "function_response",
				FunctionResponse: &core.FunctionResponse{
					ID:   funcCall.ID,
					Name: funcCall.Name,
					Response: map[string]any{
						"error": fmt.Sprintf("Unknown tool: %s", funcCall.Name),
					},
				},
			})
			continue
		}

		log.Printf("Executing tool: %s", tool.Name())
		toolCtx := core.NewToolContext(invocationCtx)
		toolCtx.FunctionCallID = &funcCall.ID

		result, err := a.executeToolWithTimeout(toolCtx, tool, funcCall.Args)
		if err != nil {
			log.Printf("Tool execution failed for %s: %v", tool.Name(), err)
			toolResponses = append(toolResponses, core.Part{
				Type: "function_response",
				FunctionResponse: &core.FunctionResponse{
					ID:   funcCall.ID,
					Name: funcCall.Name,
					Response: map[string]any{
						"error": err.Error(),
					},
				},
			})
			continue
		}

		log.Printf("Tool execution succeeded for %s: %v", tool.Name(), result)

		// Format the response properly for the LLM
		var response map[string]any
		if resultMap, ok := result.(map[string]interface{}); ok {
			// If result is already a map, use it directly
			response = resultMap
		} else {
			// Otherwise wrap it in a result field
			response = map[string]any{
				"result": result,
			}
		}

		toolResponses = append(toolResponses, core.Part{
			Type: "function_response",
			FunctionResponse: &core.FunctionResponse{
				ID:       funcCall.ID,
				Name:     funcCall.Name,
				Response: response,
			},
		})

		// Apply any state changes from the tool
		if len(toolCtx.Actions.StateDelta) > 0 {
			log.Printf("Applying state delta from tool %s: %v", tool.Name(), toolCtx.Actions.StateDelta)
			invocationCtx.Session.UpdateState(toolCtx.Actions.StateDelta)
		}
	}

	log.Println("Tool execution completed.")

	// Execute after-tool callback
	if a.callbacks.AfterToolCallback != nil {
		// Create events for the tool responses to pass to the callback
		var toolEvents []*core.Event
		for _, part := range toolResponses {
			if part.Type == "function_response" && part.FunctionResponse != nil {
				event := core.NewEvent(invocationCtx.InvocationID, a.name)
				event.Content = &core.Content{
					Role:  "agent",
					Parts: []core.Part{part},
				}
				toolEvents = append(toolEvents, event)
			}
		}

		if err := a.callbacks.AfterToolCallback(invocationCtx, toolEvents); err != nil {
			return nil, fmt.Errorf("after-tool callback failed: %w", err)
		}
	}

	return toolResponses, nil
}

// executeToolWithTimeout executes a tool with the configured timeout.
func (a *LLMAgent) executeToolWithTimeout(toolCtx *core.ToolContext, tool core.BaseTool, args map[string]any) (any, error) {
	// Create context with timeout

	log.Printf("Tool arguments: %+v", args)
	if args == nil {
		return nil, fmt.Errorf("tool arguments are nil")
	}

	// Execute tool
	return tool.RunAsync(toolCtx, args)
}

// makeRetriableLLMCall makes an LLM call with retry logic.
func (a *LLMAgent) makeRetriableLLMCall(ctx context.Context, request *core.LLMRequest) (*core.LLMResponse, error) {
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
func (a *LLMAgent) isRetryableError(err error) bool {
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

// buildLLMRequest constructs an LLM request from the session context using functional programming style.
func (a *LLMAgent) buildLLMRequest(invocationCtx *core.InvocationContext) (*core.LLMRequest, error) {
	log.Println("Building LLM request...")

	// Step 1: Start with empty contents and build step by step
	contents := make([]core.Content, 0)

	// Step 2: Add system instruction (if present)
	contents = a.addSystemInstruction(contents)

	// Step 3: Add session history (excluding system messages)
	contents = a.addSessionHistory(contents, invocationCtx.Session.Events)

	// Step 4: Add current user content (with deduplication)
	contents = a.addUserContentIfNew(contents, invocationCtx.UserContent)

	// Step 5: Build tool declarations
	tools := a.buildToolDeclarations()

	// Step 6: Create LLM configuration
	llmConfig := a.createLLMConfig(tools)

	// Step 7: Log final contents for debugging
	a.logRequestContents(contents)

	return &core.LLMRequest{
		Contents: contents,
		Config:   llmConfig,
		Tools:    tools,
	}, nil
}

// addSystemInstruction adds system instruction to contents if present.
func (a *LLMAgent) addSystemInstruction(contents []core.Content) []core.Content {
	if a.instruction == "" {
		return contents
	}

	systemContent := core.Content{
		Role: "system",
		Parts: []core.Part{
			{
				Type: "text",
				Text: &a.instruction,
			},
		},
	}

	return append(contents, systemContent)
}

// addSessionHistory adds session events to contents, excluding system messages.
func (a *LLMAgent) addSessionHistory(contents []core.Content, events []*core.Event) []core.Content {
	for _, event := range events {
		if event.Content != nil && event.Content.Role != "system" {
			contents = append(contents, *event.Content)
		}
	}
	log.Printf("Added %d session events to contents", len(events))
	return contents
}

// addUserContentIfNew adds user content only if it's not already in the session.
func (a *LLMAgent) addUserContentIfNew(contents []core.Content, userContent *core.Content) []core.Content {
	if userContent == nil {
		return contents
	}

	// Check if user content already exists in contents
	if a.isUserContentDuplicate(contents, userContent) {
		log.Println("User content already exists in session - skipping duplicate")
		return contents
	}

	log.Printf("Adding new user content: %s", formatContent(userContent))
	return append(contents, *userContent)
}

// isUserContentDuplicate checks if the user content is already present in contents.
func (a *LLMAgent) isUserContentDuplicate(contents []core.Content, userContent *core.Content) bool {
	if len(contents) == 0 {
		return false
	}

	// Check the last content item to see if it matches the user content
	lastContent := contents[len(contents)-1]
	if lastContent.Role != "user" {
		return false
	}

	return a.contentsEqual(&lastContent, userContent)
}

// contentsEqual compares two Content objects for equality.
func (a *LLMAgent) contentsEqual(content1, content2 *core.Content) bool {
	if content1.Role != content2.Role {
		return false
	}

	if len(content1.Parts) != len(content2.Parts) {
		return false
	}

	// For simplicity, check only the first text part
	// This covers the most common case of simple text messages
	if len(content1.Parts) > 0 && len(content2.Parts) > 0 {
		part1 := content1.Parts[0]
		part2 := content2.Parts[0]

		if part1.Type == "text" && part2.Type == "text" &&
			part1.Text != nil && part2.Text != nil {
			return *part1.Text == *part2.Text
		}
	}

	return false
}

// buildToolDeclarations creates tool declarations from available tools.
func (a *LLMAgent) buildToolDeclarations() []*core.FunctionDeclaration {
	var tools []*core.FunctionDeclaration

	for _, tool := range a.tools {
		if decl := tool.GetDeclaration(); decl != nil {
			tools = append(tools, decl)
			log.Printf("Added tool declaration: %s", decl.Name)
		}
	}

	log.Printf("Built %d tool declarations", len(tools))
	return tools
}

// createLLMConfig creates the LLM configuration object.
func (a *LLMAgent) createLLMConfig(tools []*core.FunctionDeclaration) *core.LLMConfig {
	config := &core.LLMConfig{
		Model:             a.config.Model,
		Temperature:       a.config.Temperature,
		MaxTokens:         a.config.MaxTokens,
		TopP:              a.config.TopP,
		TopK:              a.config.TopK,
		Tools:             tools,
		SystemInstruction: &a.instruction,
	}

	log.Printf("Created LLM config: Model=%s, Tools=%d", config.Model, len(tools))
	return config
}

// logRequestContents logs the final conversation contents for debugging.
func (a *LLMAgent) logRequestContents(contents []core.Content) {
	log.Printf("LLM Request Contents (%d items):", len(contents))
	for i, content := range contents {
		log.Printf("  [%d] Role: %s, Parts: %d", i, content.Role, len(content.Parts))
		for j, part := range content.Parts {
			switch part.Type {
			case "text":
				if part.Text != nil {
					log.Printf("    Part[%d]: text='%s'", j, *part.Text)
				}
			case "function_call":
				if part.FunctionCall != nil {
					log.Printf("    Part[%d]: function_call=%s(%v)", j, part.FunctionCall.Name, part.FunctionCall.Args)
				}
			case "function_response":
				if part.FunctionResponse != nil {
					log.Printf("    Part[%d]: function_response=%s -> %+v", j, part.FunctionResponse.Name, part.FunctionResponse.Response)
				}
			}
		}
	}
}

// Run is a synchronous wrapper around RunAsync.
func (a *LLMAgent) Run(invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	stream, err := a.RunAsync(invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range stream {
		events = append(events, event)
	}

	// Execute after-agent callback if present
	if a.afterAgentCallback != nil {
		if err := a.afterAgentCallback(invocationCtx, events); err != nil {
			return events, fmt.Errorf("after-agent callback failed: %w", err)
		}
	}

	return events, nil
}
