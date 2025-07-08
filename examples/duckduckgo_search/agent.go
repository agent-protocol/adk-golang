// Package main provides the Google Search agent implementation for ADK-Golang.
// This agent demonstrates using Ollama with Google Search functionality.
package main

import (
	"os"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/llm"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

// RootAgent creates and configures the main agent with Google Search capability.
// This agent uses Ollama for LLM inference and includes a local search tool.
var RootAgent core.BaseAgent

func init() {
	// Get model name from environment or use default
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.2"
	}

	// Create Ollama configuration
	ollamaConfig := &llm.OllamaConfig{
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
	ollamaConnection := llm.NewOllamaConnection(ollamaConfig)

	// Create agent configuration for Ollama
	agentConfig := &agents.LlmAgentConfig{
		Model:            modelName,
		Temperature:      ptr.Float32(0.7),
		MaxTokens:        ptr.Ptr(4096),
		MaxToolCalls:     3, // Limit tool calls to prevent loops
		ToolCallTimeout:  30 * time.Second,
		RetryAttempts:    3,
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

	// Set instruction to match the Python tutorial
	agent.SetInstruction("You are an expert researcher. You always stick to the facts. When users ask questions, first use the google_search tool to find current and accurate information, then provide a comprehensive answer based on the search results. Always provide a final response to the user after using tools - do not keep searching repeatedly for the same information.")

	// Add the local search tool (equivalent to google_search in Python)
	searchTool := tools.NewDuckDuckGoSearchTool()
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
	if err == nil {
		agent.AddTool(timeTool)
	}

	// Weather helper tool (mock implementation for demo)
	weatherTool, err := tools.NewFunctionTool(
		"get_weather_info",
		"Gets weather information for a location (searches for current weather data)",
		func(location string) map[string]interface{} {
			return map[string]interface{}{
				"suggestion": "I'll search for current weather information for " + location,
				"note":       "Use google_search to find the most current weather data",
			}
		},
	)
	if err == nil {
		agent.AddTool(weatherTool)
	}

	// Set the global RootAgent
	RootAgent = agent
}
