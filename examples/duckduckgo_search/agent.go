// Package main provides the Google Search agent implementation for ADK-Golang.
// This agent demonstrates using Ollama with Google Search functionality.
package main

import (
	"log"
	"os"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

// RootAgent creates and configures the main agent with Google Search capability.
// This agent uses Ollama for LLM inference and includes a local search tool.
var RootAgent core.BaseAgent

func init() {
	log.Println("Initializing DuckDuckGo Search Agent...")

	// Get model name from environment or use default
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.2"
	}

	// Create Ollama configuration
	ollamaConfig := &ollama.OllamaConfig{
		BaseURL:     "http://localhost:11434",
		Model:       modelName,
		Temperature: ptr.Float32(0.7),
		Timeout:     30 * time.Second,
		Stream:      false,
	}

	// Allow environment variable overrides
	if baseURL := os.Getenv("OLLAMA_API_BASE"); baseURL != "" {
		ollamaConfig.BaseURL = baseURL
	}

	// Create Ollama connection
	ollamaConnection := ollama.NewOllamaConnection(ollamaConfig)

	// Create agent configuration for Ollama
	agentConfig := &agents.LlmAgentConfig{
		Model:            modelName,
		Temperature:      ptr.Float32(0.3), // Lower temperature for more consistent behavior
		MaxTokens:        ptr.Ptr(4096),
		MaxToolCalls:     1, // Only allow 1 tool call to prevent loops
		ToolCallTimeout:  30 * time.Second,
		RetryAttempts:    2,    // Reduce retries to prevent multiple calls
		StreamingEnabled: true, // Enable streaming for web UI
	}

	// Create the LLM agent
	agent := agents.NewEnhancedLlmAgent(
		"duckduckgo_search_agent",
		"Agent to answer questions using DuckDuckGo Search", // Description
		agentConfig,
	)

	// Set the LLM connection
	agent.SetLLMConnection(ollamaConnection)

	// Set instruction for better tool usage
	agent.SetInstruction(`You are a helpful search assistant. When users ask questions:

1. Use the duckduckgo_search tool ONCE to find relevant information
2. Present the search results in a clear, organized format with:
   - A brief summary of what you found
   - List the key results with titles and brief descriptions
   - Include relevant URLs so users can learn more
3. Do NOT call the search tool multiple times for the same query
4. Always provide a complete response based on the search results

Example response format:
"I found several great resources about [topic]:

1. **[Title 1]** - [Brief description]
   Link: [URL]

2. **[Title 2]** - [Brief description] 
   Link: [URL]

[Additional context or summary]"

Remember: Call each tool only ONCE per user question.`)

	// Add the local search tool (DuckDuckGo Search implementation)
	searchTool := tools.NewDuckDuckGoSearchTool()
	log.Println("Adding tools to the agent...")
	log.Println("Adding DuckDuckGo Search Tool...")
	agent.AddTool(searchTool)

	// Add some additional useful tools for demonstration

	// Time tool for current time queries
	timeTool, err := tools.NewFunctionTool(
		"get_current_time",
		"Gets the current time and date in a specific location",
		func(location string) map[string]interface{} {
			if location == "" {
				location = "UTC"
			}
			now := time.Now()
			return map[string]interface{}{
				"location": location,
				"time":     now.Format("3:04 PM"),
				"date":     now.Format("Monday, January 2, 2006"),
				"timezone": now.Format("MST"),
				"iso":      now.Format(time.RFC3339),
			}
		},
	)
	log.Println("Adding Time Tool...")
	if err == nil {
		agent.AddTool(timeTool)
	} else {
		log.Printf("Failed to add Time Tool: %v", err)
	}

	// Static Time tool for testing without parameters
	staticTimeTool, err := tools.NewFunctionTool(
		"get_static_time",
		"Gets the current server time without requiring any parameters",
		func() map[string]interface{} {
			now := time.Now()
			return map[string]interface{}{
				"time":     now.Format("3:04 PM"),
				"date":     now.Format("Monday, January 2, 2006"),
				"timezone": now.Format("MST"),
				"iso":      now.Format(time.RFC3339),
			}
		},
	)
	log.Println("Adding Static Time Tool...")
	if err == nil {
		agent.AddTool(staticTimeTool)
	} else {
		log.Printf("Failed to add Static Time Tool: %v", err)
	}

	log.Println("DuckDuckGo Search Agent initialization complete.")

	// Set the global RootAgent
	RootAgent = agent
}
