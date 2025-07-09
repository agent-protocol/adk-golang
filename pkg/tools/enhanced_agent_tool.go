// Package tools provides concrete implementations of tool types.
package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// EnhancedAgentTool wraps another agent as a tool with enhanced capabilities.
// This tool enables multi-agent workflows by allowing one agent to call another agent as a tool.
type EnhancedAgentTool struct {
	*BaseToolImpl
	agent         core.BaseAgent
	timeout       time.Duration
	isolateState  bool // Whether to isolate the agent's state changes
	errorStrategy ErrorStrategy
}

// ErrorStrategy defines how the tool handles errors from the wrapped agent.
type ErrorStrategy int

const (
	// ErrorStrategyPropagate propagates errors up to the calling agent
	ErrorStrategyPropagate ErrorStrategy = iota
	// ErrorStrategyReturnError returns the error as a string result
	ErrorStrategyReturnError
	// ErrorStrategyReturnEmpty returns an empty result on errors
	ErrorStrategyReturnEmpty
)

// AgentToolConfig configures the behavior of an EnhancedAgentTool.
type AgentToolConfig struct {
	// Timeout specifies the maximum duration for agent execution
	Timeout time.Duration
	// IsolateState determines if state changes should be isolated
	IsolateState bool
	// ErrorStrategy defines how errors are handled
	ErrorStrategy ErrorStrategy
	// CustomInstruction provides additional context for the agent
	CustomInstruction string
}

// DefaultAgentToolConfig returns a sensible default configuration.
func DefaultAgentToolConfig() *AgentToolConfig {
	return &AgentToolConfig{
		Timeout:       30 * time.Second,
		IsolateState:  false,
		ErrorStrategy: ErrorStrategyPropagate,
	}
}

// NewEnhancedAgentTool creates a new enhanced agent tool with default configuration.
func NewEnhancedAgentTool(agent core.BaseAgent) *EnhancedAgentTool {
	return NewEnhancedAgentToolWithConfig(agent, DefaultAgentToolConfig())
}

// NewEnhancedAgentToolWithConfig creates a new enhanced agent tool with custom configuration.
func NewEnhancedAgentToolWithConfig(agent core.BaseAgent, config *AgentToolConfig) *EnhancedAgentTool {
	description := agent.Description()
	if config.CustomInstruction != "" {
		description = fmt.Sprintf("%s. %s", description, config.CustomInstruction)
	}

	return &EnhancedAgentTool{
		BaseToolImpl: NewBaseTool(
			fmt.Sprintf("agent_%s", agent.Name()),
			description,
		),
		agent:         agent,
		timeout:       config.Timeout,
		isolateState:  config.IsolateState,
		errorStrategy: config.ErrorStrategy,
	}
}

// GetDeclaration returns the function declaration for the enhanced agent tool.
func (t *EnhancedAgentTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        t.name,
		Description: t.description,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"request": map[string]interface{}{
					"type":        "string",
					"description": "The request or question to send to the agent",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "Optional additional context for the agent",
				},
			},
			"required": []string{"request"},
		},
	}
}

// RunAsync executes the wrapped agent with the given request and enhanced error handling.
func (t *EnhancedAgentTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	// Extract arguments
	request, ok := args["request"].(string)
	if !ok {
		return nil, fmt.Errorf("request parameter must be a string")
	}

	additionalContext, _ := args["context"].(string)

	// Apply timeout if configured
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	// Build the full request with additional context if provided
	fullRequest := request
	if additionalContext != "" {
		fullRequest = fmt.Sprintf("%s\n\nAdditional context: %s", request, additionalContext)
	}

	// Create a new invocation context for the agent
	agentCtx := core.NewInvocationContext(
		ctx,
		fmt.Sprintf("%s_sub_%d", toolCtx.InvocationContext.InvocationID, time.Now().UnixNano()),
		t.agent,
		toolCtx.InvocationContext.Session,
		toolCtx.InvocationContext.SessionService,
	)

	// Copy services from parent context
	agentCtx.ArtifactService = toolCtx.InvocationContext.ArtifactService
	agentCtx.MemoryService = toolCtx.InvocationContext.MemoryService
	agentCtx.CredentialService = toolCtx.InvocationContext.CredentialService

	// Set the user content
	agentCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: &fullRequest,
			},
		},
	}

	// Run the agent with enhanced error handling
	result, err := t.executeAgent(ctx, agentCtx, toolCtx)
	if err != nil {
		return t.handleError(err)
	}

	return result, nil
}

// executeAgent runs the agent and collects the results.
func (t *EnhancedAgentTool) executeAgent(ctx context.Context, agentCtx *core.InvocationContext, toolCtx *core.ToolContext) (string, error) {
	// Run the agent
	eventStream, err := t.agent.RunAsync(agentCtx)
	if err != nil {
		return "", fmt.Errorf("failed to start agent %s: %w", t.agent.Name(), err)
	}

	// Collect all events and track state changes
	var events []*core.Event
	var stateChanges = make(map[string]any)
	var artifactChanges = make(map[string]int)

	for {
		select {
		case event, ok := <-eventStream:
			if !ok {
				// Channel closed, we're done
				goto ProcessResults
			}

			events = append(events, event)

			// Track state changes
			if len(event.Actions.StateDelta) > 0 {
				for k, v := range event.Actions.StateDelta {
					stateChanges[k] = v
				}
			}

			// Track artifact changes
			if len(event.Actions.ArtifactDelta) > 0 {
				for k, v := range event.Actions.ArtifactDelta {
					artifactChanges[k] = v
				}
			}

			// Check for errors in events
			if event.ErrorMessage != nil && *event.ErrorMessage != "" {
				return "", fmt.Errorf("agent %s returned error: %s", t.agent.Name(), *event.ErrorMessage)
			}

		case <-ctx.Done():
			// Context cancelled (timeout or user cancellation)
			return "", fmt.Errorf("agent %s execution cancelled: %w", t.agent.Name(), ctx.Err())
		}
	}

ProcessResults:
	// Apply state changes if not isolated
	if !t.isolateState && len(stateChanges) > 0 {
		toolCtx.State.Update(stateChanges)
	}

	// Apply artifact changes if not isolated
	if !t.isolateState && len(artifactChanges) > 0 {
		for k, v := range artifactChanges {
			toolCtx.Actions.ArtifactDelta[k] = v
		}
	}

	// Extract the final result from events
	return t.extractResult(events)
}

// extractResult extracts the final text result from the event sequence.
func (t *EnhancedAgentTool) extractResult(events []*core.Event) (string, error) {
	if len(events) == 0 {
		return "", nil
	}

	var resultParts []string

	// Look for the final response from the agent
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Content == nil || event.Author == "user" {
			continue
		}

		// Extract text from this event
		for _, part := range event.Content.Parts {
			if part.Text != nil && strings.TrimSpace(*part.Text) != "" {
				resultParts = append([]string{*part.Text}, resultParts...)
			}
		}

		// If this is marked as a final response, stop here
		if event.IsFinalResponse() {
			break
		}
	}

	if len(resultParts) == 0 {
		return "", nil
	}

	return strings.Join(resultParts, "\n"), nil
}

// handleError processes errors according to the configured error strategy.
func (t *EnhancedAgentTool) handleError(err error) (any, error) {
	switch t.errorStrategy {
	case ErrorStrategyPropagate:
		return nil, err
	case ErrorStrategyReturnError:
		return fmt.Sprintf("Error from agent %s: %s", t.agent.Name(), err.Error()), nil
	case ErrorStrategyReturnEmpty:
		return "", nil
	default:
		return nil, err
	}
}

// Agent returns the wrapped agent.
func (t *EnhancedAgentTool) Agent() core.BaseAgent {
	return t.agent
}

// SetTimeout updates the execution timeout.
func (t *EnhancedAgentTool) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// SetIsolateState configures whether state changes should be isolated.
func (t *EnhancedAgentTool) SetIsolateState(isolate bool) {
	t.isolateState = isolate
}

// SetErrorStrategy configures the error handling strategy.
func (t *EnhancedAgentTool) SetErrorStrategy(strategy ErrorStrategy) {
	t.errorStrategy = strategy
}
