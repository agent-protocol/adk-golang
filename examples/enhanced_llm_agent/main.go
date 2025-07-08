// Example: Enhanced LLM Agent with Tool Execution Pipeline
//
// This example demonstrates how to use the EnhancedLlmAgent with a comprehensive
// tool execution pipeline, including function calling, conversation flow management,
// and error handling.

package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

// ExampleLLMConnection is a simple mock LLM connection for demonstration.
type ExampleLLMConnection struct {
}

func NewExampleLLMConnection() *ExampleLLMConnection {
	return &ExampleLLMConnection{}
}

func (e *ExampleLLMConnection) GenerateContent(ctx context.Context, request *core.LLMRequest) (*core.LLMResponse, error) {
	// Simulate processing the request and generating appropriate responses
	lastContent := e.getLastUserMessage(request.Contents)

	// Check if we need to call tools based on the user's request
	if strings.Contains(strings.ToLower(lastContent), "calculate") {
		return e.generateCalculatorToolCall(lastContent)
	}

	if strings.Contains(strings.ToLower(lastContent), "weather") {
		return e.generateWeatherToolCall(lastContent)
	}

	if strings.Contains(strings.ToLower(lastContent), "search") {
		return e.generateSearchToolCall(lastContent)
	}

	// Check if we have function responses to process
	if e.hasFunctionResponses(request.Contents) {
		return e.generateFinalResponse(request.Contents)
	}

	// Default conversational response
	return &core.LLMResponse{
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Hello! I can help you with calculations, weather information, and web searches. What would you like to do?"),
				},
			},
		},
	}, nil
}

func (e *ExampleLLMConnection) GenerateContentStream(ctx context.Context, request *core.LLMRequest) (<-chan *core.LLMResponse, error) {
	// For simplicity, just return a single response as a stream
	stream := make(chan *core.LLMResponse, 1)
	go func() {
		defer close(stream)
		response, err := e.GenerateContent(ctx, request)
		if err == nil {
			stream <- response
		}
	}()
	return stream, nil
}

func (e *ExampleLLMConnection) Close(ctx context.Context) error {
	return nil
}

func (e *ExampleLLMConnection) getLastUserMessage(contents []core.Content) string {
	for i := len(contents) - 1; i >= 0; i-- {
		if contents[i].Role == "user" {
			for _, part := range contents[i].Parts {
				if part.Text != nil {
					return *part.Text
				}
			}
		}
	}
	return ""
}

func (e *ExampleLLMConnection) hasFunctionResponses(contents []core.Content) bool {
	for _, content := range contents {
		for _, part := range content.Parts {
			if part.FunctionResponse != nil {
				return true
			}
		}
	}
	return false
}

func (e *ExampleLLMConnection) generateCalculatorToolCall(userMessage string) (*core.LLMResponse, error) {
	return &core.LLMResponse{
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "function_call",
					FunctionCall: &core.FunctionCall{
						ID:   "calc_1",
						Name: "calculator",
						Args: map[string]any{
							"expression": e.extractCalculationExpression(userMessage),
						},
					},
				},
			},
		},
	}, nil
}

func (e *ExampleLLMConnection) generateWeatherToolCall(userMessage string) (*core.LLMResponse, error) {
	return &core.LLMResponse{
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "function_call",
					FunctionCall: &core.FunctionCall{
						ID:   "weather_1",
						Name: "weather",
						Args: map[string]any{
							"location": e.extractLocation(userMessage),
						},
					},
				},
			},
		},
	}, nil
}

func (e *ExampleLLMConnection) generateSearchToolCall(userMessage string) (*core.LLMResponse, error) {
	return &core.LLMResponse{
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "function_call",
					FunctionCall: &core.FunctionCall{
						ID:   "search_1",
						Name: "web_search",
						Args: map[string]any{
							"query": e.extractSearchQuery(userMessage),
						},
					},
				},
			},
		},
	}, nil
}

func (e *ExampleLLMConnection) generateFinalResponse(contents []core.Content) (*core.LLMResponse, error) {
	// Find the last function response and generate a response based on it
	var lastFunctionResponse *core.FunctionResponse
	for i := len(contents) - 1; i >= 0; i-- {
		for _, part := range contents[i].Parts {
			if part.FunctionResponse != nil {
				lastFunctionResponse = part.FunctionResponse
				break
			}
		}
		if lastFunctionResponse != nil {
			break
		}
	}

	if lastFunctionResponse == nil {
		return &core.LLMResponse{
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{
						Type: "text",
						Text: stringPtr("I apologize, but I couldn't process the tool response properly."),
					},
				},
			},
		}, nil
	}

	// Generate response based on the tool that was called
	var responseText string
	switch lastFunctionResponse.Name {
	case "calculator":
		if result, ok := lastFunctionResponse.Response["result"]; ok {
			responseText = fmt.Sprintf("The calculation result is: %v", result)
		} else {
			responseText = "I was unable to perform the calculation."
		}
	case "weather":
		if result, ok := lastFunctionResponse.Response["result"]; ok {
			responseText = fmt.Sprintf("Here's the weather information: %v", result)
		} else {
			responseText = "I was unable to get weather information."
		}
	case "web_search":
		if result, ok := lastFunctionResponse.Response["result"]; ok {
			responseText = fmt.Sprintf("Here's what I found: %v", result)
		} else {
			responseText = "I was unable to perform the search."
		}
	default:
		responseText = "I've completed the requested task."
	}

	return &core.LLMResponse{
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "text",
					Text: &responseText,
				},
			},
		},
	}, nil
}

func (e *ExampleLLMConnection) extractCalculationExpression(message string) string {
	// Simple extraction - in practice, you'd use more sophisticated parsing
	message = strings.ToLower(message)
	if strings.Contains(message, "2+2") {
		return "2+2"
	}
	if strings.Contains(message, "10*5") {
		return "10*5"
	}
	if strings.Contains(message, "sqrt(16)") {
		return "sqrt(16)"
	}
	return "2+2" // Default calculation
}

func (e *ExampleLLMConnection) extractLocation(message string) string {
	message = strings.ToLower(message)
	if strings.Contains(message, "new york") {
		return "New York"
	}
	if strings.Contains(message, "london") {
		return "London"
	}
	if strings.Contains(message, "tokyo") {
		return "Tokyo"
	}
	return "San Francisco" // Default location
}

func (e *ExampleLLMConnection) extractSearchQuery(message string) string {
	// Extract search query - simplified implementation
	words := strings.Fields(strings.ToLower(message))
	for i, word := range words {
		if word == "search" && i+1 < len(words) {
			return strings.Join(words[i+1:], " ")
		}
	}
	return "artificial intelligence"
}

// CalculatorTool performs mathematical calculations.
type CalculatorTool struct {
	*tools.BaseToolImpl
}

func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{
		BaseToolImpl: tools.NewBaseTool("calculator", "Performs mathematical calculations"),
	}
}

func (t *CalculatorTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "calculator",
		Description: "Performs mathematical calculations",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "Mathematical expression to calculate (e.g., '2+2', '10*5', 'sqrt(16)')",
				},
			},
			"required": []string{"expression"},
		},
	}
}

func (t *CalculatorTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	expression, ok := args["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("expression must be a string")
	}

	// Simple calculator implementation
	switch expression {
	case "2+2":
		return 4, nil
	case "10*5":
		return 50, nil
	case "sqrt(16)":
		return math.Sqrt(16), nil
	default:
		return nil, fmt.Errorf("unsupported expression: %s", expression)
	}
}

// WeatherTool provides weather information.
type WeatherTool struct {
	*tools.BaseToolImpl
}

func NewWeatherTool() *WeatherTool {
	return &WeatherTool{
		BaseToolImpl: tools.NewBaseTool("weather", "Provides weather information for a location"),
	}
}

func (t *WeatherTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "weather",
		Description: "Get weather information for a specific location",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The location to get weather for",
				},
			},
			"required": []string{"location"},
		},
	}
}

func (t *WeatherTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	location, ok := args["location"].(string)
	if !ok {
		return nil, fmt.Errorf("location must be a string")
	}

	// Simulate weather data
	weatherData := map[string]interface{}{
		"location":    location,
		"temperature": "22°C",
		"condition":   "Sunny",
		"humidity":    "65%",
		"wind_speed":  "10 km/h",
	}

	return weatherData, nil
}

// WebSearchTool performs web searches.
type WebSearchTool struct {
	*tools.BaseToolImpl
}

func NewWebSearchTool() *WebSearchTool {
	tool := &WebSearchTool{
		BaseToolImpl: tools.NewBaseTool("web_search", "Searches the web for information"),
	}
	tool.SetLongRunning(true) // Simulate that web searches take time
	return tool
}

func (t *WebSearchTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "web_search",
		Description: "Search the web for information",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (t *WebSearchTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	// Simulate search delay
	time.Sleep(1 * time.Second)

	// Simulate search results
	searchResults := []map[string]interface{}{
		{
			"title":   "Search Result 1 for: " + query,
			"url":     "https://example.com/result1",
			"snippet": "This is a sample search result snippet for " + query,
		},
		{
			"title":   "Search Result 2 for: " + query,
			"url":     "https://example.com/result2",
			"snippet": "Another search result about " + query,
		},
	}

	return searchResults, nil
}

func main() {
	fmt.Println("🤖 Enhanced LLM Agent with Tool Execution Pipeline Demo")
	fmt.Println("======================================================")

	// Create enhanced LLM agent with custom configuration
	config := &agents.LlmAgentConfig{
		Model:            "gpt-4",
		Temperature:      floatPtr(0.7),
		MaxTokens:        intPtr(2048),
		MaxToolCalls:     5,
		ToolCallTimeout:  30 * time.Second,
		RetryAttempts:    3,
		StreamingEnabled: false,
	}

	agent := agents.NewEnhancedLlmAgent(
		"enhanced-assistant",
		"An AI assistant with calculation, weather, and search capabilities",
		config,
	)

	// Set system instruction
	agent.SetInstruction("You are a helpful AI assistant with access to tools for calculations, weather information, and web searches. Always use the appropriate tool when the user requests these capabilities.")

	// Set up LLM connection
	llmConnection := NewExampleLLMConnection()
	agent.SetLLMConnection(llmConnection)

	// Add tools to the agent
	agent.AddTool(NewCalculatorTool())
	agent.AddTool(NewWeatherTool())
	agent.AddTool(NewWebSearchTool())

	// Set up callbacks for monitoring
	callbacks := &agents.LlmAgentCallbacks{
		BeforeModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
			fmt.Println("🧠 About to call LLM...")
			return nil
		},
		AfterModelCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
			fmt.Println("✅ LLM call completed")
			return nil
		},
		BeforeToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext) error {
			fmt.Println("🔧 About to execute tool...")
			return nil
		},
		AfterToolCallback: func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
			fmt.Println("✅ Tool execution completed")
			return nil
		},
	}
	agent.SetCallbacks(callbacks)

	// Create session and context
	session := core.NewSession("demo-session", "demo-app", "demo-user")
	invocationID := "demo-invocation-" + time.Now().Format("20060102150405")

	// Demo conversations
	conversations := []string{
		"Hello! What can you help me with?",
		"Can you calculate 2+2 for me?",
		"What's the weather like in New York?",
		"Search for information about artificial intelligence",
		"Thanks for all your help!",
	}

	ctx := context.Background()

	for i, userMessage := range conversations {
		fmt.Printf("\n--- Conversation Turn %d ---\n", i+1)
		fmt.Printf("👤 User: %s\n", userMessage)

		// Create invocation context
		invocationCtx := core.NewInvocationContext(invocationID, agent, session, nil)
		invocationCtx.UserContent = &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: &userMessage,
				},
			},
		}

		// Run the agent
		events, err := agent.Run(ctx, invocationCtx)
		if err != nil {
			log.Printf("❌ Error running agent: %v", err)
			continue
		}

		// Process and display events
		fmt.Printf("🤖 Assistant: ")
		for _, event := range events {
			if event.Content != nil {
				for _, part := range event.Content.Parts {
					if part.Type == "text" && part.Text != nil {
						fmt.Printf("%s", *part.Text)
					} else if part.Type == "function_call" && part.FunctionCall != nil {
						fmt.Printf("[Calling %s tool...]", part.FunctionCall.Name)
					} else if part.Type == "function_response" && part.FunctionResponse != nil {
						fmt.Printf("[Tool response received]")
					}
				}
			}
		}
		fmt.Println()

		// Add some delay for better UX
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n🎉 Demo completed!")
	fmt.Printf("📊 Agent Info:\n")
	fmt.Printf("   - Model: %s\n", agent.Model())
	fmt.Printf("   - Tools: %d\n", len(agent.Tools()))
	fmt.Printf("   - Session Events: %d\n", len(session.Events))

	// Display session summary
	fmt.Printf("\n📝 Session Summary:\n")
	for i, event := range session.Events {
		if event.Content != nil {
			role := event.Author
			if role == agent.Name() {
				role = "assistant"
			}
			fmt.Printf("   %d. %s: ", i+1, role)

			for _, part := range event.Content.Parts {
				if part.Text != nil {
					text := *part.Text
					if len(text) > 50 {
						text = text[:50] + "..."
					}
					fmt.Printf("%s", text)
				} else if part.FunctionCall != nil {
					fmt.Printf("[Called %s]", part.FunctionCall.Name)
				}
			}
			fmt.Println()
		}
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float32) *float32 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
