// Package main demonstrates how to ensure LLM responses follow proper structure and use tools effectively.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/llm"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

// Example tool for demonstration
func getCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func getWeatherInfo(location string) map[string]interface{} {
	// Simulated weather data
	return map[string]interface{}{
		"location":    location,
		"temperature": "22°C",
		"condition":   "Sunny",
		"humidity":    "65%",
	}
}

func main() {
	ctx := context.Background()

	// Example 1: Enhanced LLM Agent with Proper Validation
	fmt.Println("=== Example 1: Enhanced LLM Agent with Validation ===")
	if err := demonstrateEnhancedAgent(ctx); err != nil {
		log.Printf("Enhanced agent demo failed: %v", err)
	}

	// Example 2: Adaptive Agent for Different Model Capabilities
	fmt.Println("\n=== Example 2: Adaptive Agent for Model Capabilities ===")
	if err := demonstrateAdaptiveAgent(ctx); err != nil {
		log.Printf("Adaptive agent demo failed: %v", err)
	}

	// Example 3: System Instruction Builder
	fmt.Println("\n=== Example 3: Advanced System Instructions ===")
	if err := demonstrateSystemInstructionBuilder(ctx); err != nil {
		log.Printf("System instruction demo failed: %v", err)
	}
}

func demonstrateEnhancedAgent(ctx context.Context) error {
	// Create enhanced LLM configuration
	config := &agents.LlmAgentConfig{
		Model:            "llama3.2",
		Temperature:      ptr.Float32(0.7),
		MaxTokens:        ptr.Ptr(1000),
		MaxToolCalls:     3,
		ToolCallTimeout:  30 * time.Second,
		RetryAttempts:    2,
		StreamingEnabled: false,
	}

	// Create agent with enhanced capabilities
	agent := agents.NewEnhancedLlmAgent(
		"weather-assistant",
		"A helpful assistant that can provide time and weather information",
		config,
	)

	// Add tools with proper declarations
	timeTool, _ := tools.NewEnhancedFunctionTool("get_current_time", "Gets the current date and time", getCurrentTime)

	weatherTool, _ := tools.NewEnhancedFunctionTool("get_weather", "Gets weather information for a specified location", getWeatherInfo)

	agent.AddTool(timeTool)
	agent.AddTool(weatherTool)

	// Set up LLM connection with enhanced prompting
	ollamaConfig := &llm.OllamaConfig{
		BaseURL:     "http://localhost:11434",
		Model:       "llama3.2",
		Temperature: ptr.Float32(0.7),
		Timeout:     30 * time.Second,
	}

	llmConn := llm.NewOllamaConnection(ollamaConfig)
	agent.SetLLMConnection(llmConn)

	// Create session and context
	session := core.NewSession("demo-session", "demo-app", "demo-user")
	invocationCtx := core.NewInvocationContext("demo-invocation", agent, session, nil)

	// Test with user input that should trigger tool usage
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("What's the current time and weather in New York?"),
			},
		},
	}

	// Add response validator
	validator := core.NewResponseValidator(true) // strict mode

	// Run agent and validate responses
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	fmt.Printf("Agent produced %d events\n", len(events))

	// Validate each event's content
	for i, event := range events {
		if event.Content != nil {
			if err := validator.ValidateContent(event.Content); err != nil {
				log.Printf("Event %d validation failed: %v", i, err)

				// Try to sanitize the content
				cleaned, cleanErr := validator.SanitizeContent(event.Content)
				if cleanErr == nil {
					log.Printf("Content sanitized successfully")
					event.Content = cleaned
				}
			} else {
				log.Printf("Event %d content is valid", i)
			}
		}
	}

	return nil
}

func demonstrateAdaptiveAgent(ctx context.Context) error {
	// Create adaptive agent that can handle different model capabilities
	config := &agents.LlmAgentConfig{
		Model:            "llama3.2", // This will be assessed as having limited capabilities
		Temperature:      ptr.Float32(0.7),
		MaxTokens:        ptr.Ptr(1000),
		MaxToolCalls:     2, // Reduced for weaker models
		ToolCallTimeout:  30 * time.Second,
		RetryAttempts:    3,
		StreamingEnabled: false,
	}

	// Create adaptive agent
	adaptiveAgent := agents.NewAdaptiveLlmAgent(
		"adaptive-assistant",
		"An assistant that adapts to different model capabilities",
		config,
	)

	// Add simple tools
	timeTool, _ := tools.NewEnhancedFunctionTool("get_time", "Gets current time", getCurrentTime)
	adaptiveAgent.AddTool(timeTool)

	// Configure retry strategy
	retryStrategy := &agents.RetryStrategy{
		MaxRetries:        3,
		BackoffStrategy:   "exponential",
		FallbackToSimple:  true,
		SimplifyOnFailure: true,
	}
	adaptiveAgent.SetRetryStrategy(retryStrategy)

	// Set up LLM connection
	ollamaConfig := &llm.OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   "llama3.2",
		Timeout: 30 * time.Second,
	}

	llmConn := llm.NewOllamaConnection(ollamaConfig)
	adaptiveAgent.SetLLMConnection(llmConn)

	// Check model capabilities
	capability := adaptiveAgent.GetModelCapability()
	fmt.Printf("Model capabilities assessment:\n")
	fmt.Printf("- Supports tool calling: %v\n", capability.SupportsToolCalling)
	fmt.Printf("- Supports complex JSON: %v\n", capability.SupportsComplexJSON)
	fmt.Printf("- Requires simple prompts: %v\n", capability.RequiresSimplePrompts)
	fmt.Printf("- Max tool calls per turn: %d\n", capability.MaxToolCallsPerTurn)
	fmt.Printf("- Preferred prompt style: %s\n", capability.PreferredPromptStyle)

	// Test scenarios
	testCases := []string{
		"What time is it?",
		"Tell me about artificial intelligence",
		"Can you help me with my homework?",
	}

	for i, testCase := range testCases {
		fmt.Printf("\nTest case %d: %s\n", i+1, testCase)

		session := core.NewSession(fmt.Sprintf("adaptive-session-%d", i), "demo-app", "demo-user")
		invocationCtx := core.NewInvocationContext(fmt.Sprintf("adaptive-invocation-%d", i), adaptiveAgent, session, nil)

		invocationCtx.UserContent = &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr(testCase),
				},
			},
		}

		events, err := adaptiveAgent.Run(ctx, invocationCtx)
		if err != nil {
			log.Printf("Test case %d failed: %v", i+1, err)
			continue
		}

		fmt.Printf("Generated %d events\n", len(events))
		for j, event := range events {
			if event.Content != nil && len(event.Content.Parts) > 0 {
				for _, part := range event.Content.Parts {
					if part.Type == "text" && part.Text != nil {
						fmt.Printf("Event %d text: %s\n", j, *part.Text)
					} else if part.Type == "function_call" && part.FunctionCall != nil {
						fmt.Printf("Event %d function call: %s\n", j, part.FunctionCall.Name)
					}
				}
			}
		}
	}

	return nil
}

func demonstrateSystemInstructionBuilder(ctx context.Context) error {
	// Create tools for demonstration
	timeTool, _ := tools.NewEnhancedFunctionTool("get_current_time", "Gets the current date and time", getCurrentTime)

	weatherTool, _ := tools.NewEnhancedFunctionTool("get_weather", "Gets weather information for a specified location", getWeatherInfo)

	availableTools := []core.BaseTool{timeTool, weatherTool}

	// Build system instructions for different scenarios
	scenarios := []struct {
		name        string
		strictMode  bool
		preventLoop bool
	}{
		{"Normal Mode", false, true},
		{"Strict Mode", true, true},
		{"No Loop Prevention", false, false},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n--- %s ---\n", scenario.name)

		builder := agents.NewSystemInstructionBuilder(
			"You are a helpful assistant that can provide time and weather information.",
		)

		instruction := builder.
			WithTools(availableTools).
			WithStrictMode(scenario.strictMode).
			WithLoopPrevention(scenario.preventLoop).
			Build()

		fmt.Printf("Generated instruction:\n%s\n", instruction)
	}

	// Demonstrate intelligent tool selector
	fmt.Printf("\n--- Intelligent Tool Selection ---\n")

	selector := agents.NewIntelligentToolSelector(availableTools)

	testQueries := []string{
		"What time is it now?",
		"What's the weather like in London?",
		"Tell me about the history of computers",
		"Can you search the internet for recent news?",
	}

	for _, query := range testQueries {
		recommendation := selector.ShouldUseTools(query, []*core.Event{})

		fmt.Printf("\nQuery: %s\n", query)
		fmt.Printf("Should use tools: %v\n", recommendation.ShouldUse)
		fmt.Printf("Reason: %s\n", recommendation.Reason)
		fmt.Printf("Suggestion: %s\n", recommendation.Suggestion)
		if len(recommendation.RelevantTools) > 0 {
			fmt.Printf("Relevant tools: %v\n", recommendation.RelevantTools)
		}
	}

	return nil
}

// Example of creating a custom validator for specific use cases
func createCustomValidator() *core.ResponseValidator {
	validator := core.NewResponseValidator(false) // not strict mode
	return validator
}

// Example of handling different model types
func handleModelSpecificIssues(modelName string) *agents.ModelCapabilityAssessment {
	switch modelName {
	case "ollama/llama3.2":
		return &agents.ModelCapabilityAssessment{
			SupportsToolCalling:   false, // Most local models struggle
			SupportsComplexJSON:   false,
			SupportsInstructions:  true,
			RequiresSimplePrompts: true,
			MaxToolCallsPerTurn:   1,
			PreferredPromptStyle:  "simple",
		}
	case "gemini-pro":
		return &agents.ModelCapabilityAssessment{
			SupportsToolCalling:   true,
			SupportsComplexJSON:   true,
			SupportsInstructions:  true,
			RequiresSimplePrompts: false,
			MaxToolCallsPerTurn:   5,
			PreferredPromptStyle:  "detailed",
		}
	default:
		// Safe defaults for unknown models
		return &agents.ModelCapabilityAssessment{
			SupportsToolCalling:   false,
			SupportsComplexJSON:   false,
			SupportsInstructions:  true,
			RequiresSimplePrompts: true,
			MaxToolCallsPerTurn:   1,
			PreferredPromptStyle:  "simple",
		}
	}
}
