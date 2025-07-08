// Package agents provides strategies for handling less capable LLM models.
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

// ModelCapabilityAssessment represents the assessed capabilities of an LLM model.
type ModelCapabilityAssessment struct {
	SupportsToolCalling   bool
	SupportsComplexJSON   bool
	SupportsInstructions  bool
	RequiresSimplePrompts bool
	MaxToolCallsPerTurn   int
	PreferredPromptStyle  string
}

// AdaptiveLlmAgent is an enhanced LLM agent that adapts to model capabilities.
type AdaptiveLlmAgent struct {
	*EnhancedLlmAgent
	modelCapability *ModelCapabilityAssessment
	fallbackMode    bool
	retryStrategy   *RetryStrategy
}

// RetryStrategy defines how to handle failed tool calls.
type RetryStrategy struct {
	MaxRetries        int
	BackoffStrategy   string // "exponential", "linear", "fixed"
	FallbackToSimple  bool
	SimplifyOnFailure bool
}

// NewAdaptiveLlmAgent creates an agent that adapts to model capabilities.
func NewAdaptiveLlmAgent(name, description string, config *LlmAgentConfig) *AdaptiveLlmAgent {
	baseAgent := NewEnhancedLlmAgent(name, description, config)

	return &AdaptiveLlmAgent{
		EnhancedLlmAgent: baseAgent,
		modelCapability:  assessModelCapability(config.Model),
		fallbackMode:     false,
		retryStrategy: &RetryStrategy{
			MaxRetries:        3,
			BackoffStrategy:   "exponential",
			FallbackToSimple:  true,
			SimplifyOnFailure: true,
		},
	}
}

// assessModelCapability assesses model capabilities based on model name/type.
func assessModelCapability(modelName string) *ModelCapabilityAssessment {
	modelLower := strings.ToLower(modelName)

	// Define capability profiles for different model types
	switch {
	case strings.Contains(modelLower, "gemini"):
		return &ModelCapabilityAssessment{
			SupportsToolCalling:   true,
			SupportsComplexJSON:   true,
			SupportsInstructions:  true,
			RequiresSimplePrompts: false,
			MaxToolCallsPerTurn:   5,
			PreferredPromptStyle:  "detailed",
		}
	case strings.Contains(modelLower, "gpt-4"):
		return &ModelCapabilityAssessment{
			SupportsToolCalling:   true,
			SupportsComplexJSON:   true,
			SupportsInstructions:  true,
			RequiresSimplePrompts: false,
			MaxToolCallsPerTurn:   5,
			PreferredPromptStyle:  "detailed",
		}
	case strings.Contains(modelLower, "gpt-3.5"):
		return &ModelCapabilityAssessment{
			SupportsToolCalling:   true,
			SupportsComplexJSON:   false,
			SupportsInstructions:  true,
			RequiresSimplePrompts: true,
			MaxToolCallsPerTurn:   3,
			PreferredPromptStyle:  "simple",
		}
	case strings.Contains(modelLower, "llama"):
		return &ModelCapabilityAssessment{
			SupportsToolCalling:   false, // Most open models struggle with this
			SupportsComplexJSON:   false,
			SupportsInstructions:  true,
			RequiresSimplePrompts: true,
			MaxToolCallsPerTurn:   1,
			PreferredPromptStyle:  "very_simple",
		}
	default:
		// Conservative defaults for unknown models
		return &ModelCapabilityAssessment{
			SupportsToolCalling:   false,
			SupportsComplexJSON:   false,
			SupportsInstructions:  true,
			RequiresSimplePrompts: true,
			MaxToolCallsPerTurn:   1,
			PreferredPromptStyle:  "simple",
		}
	}
}

// buildLLMRequest overrides the base method to adapt to model capabilities.
func (a *AdaptiveLlmAgent) buildLLMRequest(invocationCtx *core.InvocationContext) (*core.LLMRequest, error) {
	// Start with the base request
	request, err := a.EnhancedLlmAgent.buildLLMRequest(invocationCtx)
	if err != nil {
		return nil, err
	}

	// Adapt based on model capabilities
	if !a.modelCapability.SupportsToolCalling || a.fallbackMode {
		// Remove tools for models that don't support them
		request.Tools = nil
		request.Config.Tools = nil

		// Enhance system instruction to compensate
		request = a.enhanceSystemInstructionForNonToolModels(request)
	} else {
		// Limit tool complexity for weaker models
		request = a.simplifyToolsForModel(request)
	}

	// Adjust prompt complexity
	request = a.adjustPromptComplexity(request)

	return request, nil
}

// enhanceSystemInstructionForNonToolModels creates detailed instructions when tools aren't available.
func (a *AdaptiveLlmAgent) enhanceSystemInstructionForNonToolModels(request *core.LLMRequest) *core.LLMRequest {
	// Build a comprehensive instruction that includes tool information
	var toolInfo strings.Builder

	toolInfo.WriteString("You don't have access to external tools, but you can help with these types of requests:\n\n")

	for _, tool := range a.tools {
		decl := tool.GetDeclaration()
		if decl != nil {
			toolInfo.WriteString(fmt.Sprintf("- %s: %s\n", decl.Name, decl.Description))
			toolInfo.WriteString("  (Provide the best answer you can based on your training data)\n\n")
		}
	}

	toolInfo.WriteString("\nIMPORTANT: When you can't access external data, clearly state this limitation and provide the best information you can from your training data.")

	// Find system instruction in contents and enhance it
	for i, content := range request.Contents {
		if content.Role == "system" {
			for j, part := range content.Parts {
				if part.Type == "text" && part.Text != nil {
					enhanced := *part.Text + "\n\n" + toolInfo.String()
					request.Contents[i].Parts[j].Text = &enhanced
				}
			}
		}
	}

	return request
}

// simplifyToolsForModel reduces tool complexity for weaker models.
func (a *AdaptiveLlmAgent) simplifyToolsForModel(request *core.LLMRequest) *core.LLMRequest {
	if len(request.Tools) <= a.modelCapability.MaxToolCallsPerTurn {
		return request
	}

	// Limit to most essential tools
	simplified := request.Tools[:a.modelCapability.MaxToolCallsPerTurn]
	request.Tools = simplified
	request.Config.Tools = simplified

	return request
}

// adjustPromptComplexity adjusts prompt complexity based on model capability.
func (a *AdaptiveLlmAgent) adjustPromptComplexity(request *core.LLMRequest) *core.LLMRequest {
	if !a.modelCapability.RequiresSimplePrompts {
		return request
	}

	// Simplify system instructions
	for i, content := range request.Contents {
		if content.Role == "system" {
			for j, part := range content.Parts {
				if part.Type == "text" && part.Text != nil {
					simplified := a.simplifyText(*part.Text)
					request.Contents[i].Parts[j].Text = &simplified
				}
			}
		}
	}

	return request
}

// simplifyText makes text instructions simpler and more direct.
func (a *AdaptiveLlmAgent) simplifyText(text string) string {
	// Remove complex formatting and make instructions more direct
	lines := strings.Split(text, "\n")
	var simplified []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove markdown formatting
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "#", "")

		// Keep only essential information
		if !strings.Contains(line, "##") && len(line) > 10 {
			simplified = append(simplified, line)
		}
	}

	return strings.Join(simplified, "\n")
}

// executeConversationFlow overrides to handle model-specific issues.
func (a *AdaptiveLlmAgent) executeConversationFlow(ctx context.Context, invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	// If model doesn't support tools, use simplified flow
	if !a.modelCapability.SupportsToolCalling || a.fallbackMode {
		return a.executeSimpleConversationFlow(ctx, invocationCtx, eventChan)
	}

	// Use enhanced flow with retry logic
	return a.executeConversationFlowWithRetry(ctx, invocationCtx, eventChan)
}

// executeSimpleConversationFlow handles models that don't support tool calling.
func (a *AdaptiveLlmAgent) executeSimpleConversationFlow(ctx context.Context, invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	log.Println("Using simple conversation flow for model with limited capabilities")

	// Build request without tools
	request, err := a.buildLLMRequest(invocationCtx)
	if err != nil {
		return fmt.Errorf("failed to build LLM request: %w", err)
	}

	// Make LLM call
	response, err := a.makeRetriableLLMCall(ctx, request)
	if err != nil {
		return fmt.Errorf("LLM request failed: %w", err)
	}

	// Create event from response
	event := core.NewEvent(invocationCtx.InvocationID, a.name)
	event.Content = response.Content
	event.TurnComplete = ptr.Ptr(true)

	// If the response seems to be asking for tool usage, provide helpful guidance
	if a.seemsToNeedTools(response.Content) {
		event = a.enhanceResponseWithToolGuidance(event)
	}

	select {
	case eventChan <- event:
	case <-ctx.Done():
		return ctx.Err()
	}

	invocationCtx.Session.AddEvent(event)
	return nil
}

// executeConversationFlowWithRetry adds retry logic for tool calls.
func (a *AdaptiveLlmAgent) executeConversationFlowWithRetry(ctx context.Context, invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	for attempt := 0; attempt < a.retryStrategy.MaxRetries; attempt++ {
		err := a.EnhancedLlmAgent.executeConversationFlow(ctx, invocationCtx, eventChan)

		if err == nil {
			return nil // Success
		}

		log.Printf("Conversation flow attempt %d failed: %v", attempt+1, err)

		// If this is the last attempt and we have fallback enabled
		if attempt == a.retryStrategy.MaxRetries-1 && a.retryStrategy.FallbackToSimple {
			log.Println("Falling back to simple conversation flow")
			a.fallbackMode = true
			return a.executeSimpleConversationFlow(ctx, invocationCtx, eventChan)
		}

		// Wait before retry
		if attempt < a.retryStrategy.MaxRetries-1 {
			waitTime := a.calculateBackoffTime(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}
	}

	return fmt.Errorf("conversation flow failed after %d attempts", a.retryStrategy.MaxRetries)
}

// calculateBackoffTime calculates wait time for retry.
func (a *AdaptiveLlmAgent) calculateBackoffTime(attempt int) time.Duration {
	base := time.Second

	switch a.retryStrategy.BackoffStrategy {
	case "exponential":
		return base * time.Duration(1<<attempt)
	case "linear":
		return base * time.Duration(attempt+1)
	default: // "fixed"
		return base
	}
}

// seemsToNeedTools checks if the response indicates the model needs tools.
func (a *AdaptiveLlmAgent) seemsToNeedTools(content *core.Content) bool {
	if content == nil {
		return false
	}

	for _, part := range content.Parts {
		if part.Type == "text" && part.Text != nil {
			text := strings.ToLower(*part.Text)

			// Look for indicators that the model wants to use tools
			indicators := []string{
				"i need to", "let me search", "i'll look up",
				"i should check", "i need access to",
				"i cannot access", "i don't have access to",
				"i need to call", "let me call",
			}

			for _, indicator := range indicators {
				if strings.Contains(text, indicator) {
					return true
				}
			}
		}
	}

	return false
}

// enhanceResponseWithToolGuidance adds helpful guidance when tools are needed but unavailable.
func (a *AdaptiveLlmAgent) enhanceResponseWithToolGuidance(event *core.Event) *core.Event {
	if event.Content == nil || len(event.Content.Parts) == 0 {
		return event
	}

	// Find the text part and enhance it
	for i, part := range event.Content.Parts {
		if part.Type == "text" && part.Text != nil {
			enhanced := *part.Text + "\n\n(Note: I don't have access to external tools in this mode, so I've provided the best answer I can based on my training data. For real-time or specific data, you may need to check external sources.)"
			event.Content.Parts[i].Text = &enhanced
			break
		}
	}

	return event
}

// SetModelCapability allows manual override of assessed model capabilities.
func (a *AdaptiveLlmAgent) SetModelCapability(capability *ModelCapabilityAssessment) {
	a.modelCapability = capability
}

// SetRetryStrategy configures the retry strategy for failed operations.
func (a *AdaptiveLlmAgent) SetRetryStrategy(strategy *RetryStrategy) {
	a.retryStrategy = strategy
}

// GetModelCapability returns the current model capability assessment.
func (a *AdaptiveLlmAgent) GetModelCapability() *ModelCapabilityAssessment {
	return a.modelCapability
}

// EnableFallbackMode forces the agent into simple mode.
func (a *AdaptiveLlmAgent) EnableFallbackMode() {
	a.fallbackMode = true
}

// DisableFallbackMode restores normal operation mode.
func (a *AdaptiveLlmAgent) DisableFallbackMode() {
	a.fallbackMode = false
}
