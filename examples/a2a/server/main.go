// Package main demonstrates how to create and run an A2A server that exposes local agents as remote services.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/a2a/server"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

func main() {
	fmt.Println("🚀 Starting A2A Server Demo")
	fmt.Println("============================")

	// Create some example tools for our agents
	greetingTool := createGreetingTool()
	calculatorTool := createCalculatorTool()
	weatherTool := createWeatherTool()

	// Create example agents
	agents := createExampleAgents(greetingTool, calculatorTool, weatherTool)

	// Create agent cards (metadata descriptions)
	agentCards := createAgentCards()

	// Create and configure the A2A server
	a2aServer := server.NewA2AServer(server.A2AServerConfig{
		Agents:     agents,
		AgentCards: agentCards,
	})

	// Set up HTTP server
	mux := http.NewServeMux()

	// A2A endpoint - handles JSON-RPC requests
	mux.Handle("/a2a", a2aServer)

	// Well-known agent discovery endpoint
	mux.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return the primary agent card
		if card, exists := agentCards["assistant"]; exists {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
  "name": "%s",
  "description": "%s",
  "url": "%s",
  "version": "%s",
  "capabilities": {
    "streaming": %t,
    "pushNotifications": %t
  },
  "skills": [
    {
      "id": "greeting",
      "name": "Greeting",
      "description": "Generate friendly greetings",
      "examples": ["Hello", "Hi there", "Good morning"]
    },
    {
      "id": "calculation",
      "name": "Calculator",
      "description": "Perform mathematical calculations",
      "examples": ["What is 2+2?", "Calculate 15*7", "Divide 100 by 5"]
    },
    {
      "id": "weather",
      "name": "Weather Info",
      "description": "Get weather information",
      "examples": ["What's the weather in Tokyo?", "Temperature in New York"]
    }
  ]
}`,
				card.Name,
				getStringValue(card.Description),
				card.URL,
				card.Version,
				card.Capabilities.Streaming,
				card.Capabilities.PushNotifications)
		} else {
			http.Error(w, "Agent not found", http.StatusNotFound)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})

	// Agent list endpoint
	mux.HandleFunc("/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		agentList := a2aServer.ListAgents()
		fmt.Fprintf(w, `{"agents": [`)
		for i, name := range agentList {
			if i > 0 {
				fmt.Fprintf(w, `,`)
			}
			fmt.Fprintf(w, `"%s"`, name)
		}
		fmt.Fprintf(w, `]}`)
	})

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Graceful shutdown handling
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\n🛑 Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Print server information
	fmt.Printf("📡 A2A Server running on http://localhost:8080\n")
	fmt.Printf("🔍 Agent discovery: http://localhost:8080/.well-known/agent.json\n")
	fmt.Printf("📋 Available agents: http://localhost:8080/agents\n")
	fmt.Printf("❤️  Health check: http://localhost:8080/health\n")
	fmt.Printf("🎯 A2A endpoint: http://localhost:8080/a2a\n")
	fmt.Println("\nAvailable agents:")
	for name, card := range agentCards {
		fmt.Printf("  - %s: %s\n", name, getStringValue(card.Description))
	}
	fmt.Println("\nPress Ctrl+C to stop the server")

	// Start server
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}

	fmt.Println("✅ Server stopped gracefully")
}

// createGreetingTool creates a tool for generating greetings
func createGreetingTool() core.BaseTool {
	tool, err := tools.NewFunctionTool(
		"greeting",
		"Generates a personalized greeting message",
		func(name string, timeOfDay string) string {
			greetings := map[string]string{
				"morning":   "Good morning",
				"afternoon": "Good afternoon",
				"evening":   "Good evening",
				"night":     "Good night",
			}

			greeting, exists := greetings[timeOfDay]
			if !exists {
				greeting = "Hello"
			}

			if name == "" {
				return fmt.Sprintf("%s! How can I help you today?", greeting)
			}
			return fmt.Sprintf("%s, %s! How can I help you today?", greeting, name)
		},
	)
	if err != nil {
		log.Fatalf("Failed to create greeting tool: %v", err)
	}
	return tool
}

// createCalculatorTool creates a tool for mathematical calculations
func createCalculatorTool() core.BaseTool {
	tool, err := tools.NewFunctionTool(
		"calculator",
		"Performs basic mathematical operations",
		func(operation string, a float64, b float64) (float64, error) {
			switch operation {
			case "add", "+":
				return a + b, nil
			case "subtract", "-":
				return a - b, nil
			case "multiply", "*":
				return a * b, nil
			case "divide", "/":
				if b == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return a / b, nil
			case "power", "^":
				result := 1.0
				for i := 0; i < int(b); i++ {
					result *= a
				}
				return result, nil
			default:
				return 0, fmt.Errorf("unsupported operation: %s", operation)
			}
		},
	)
	if err != nil {
		log.Fatalf("Failed to create calculator tool: %v", err)
	}
	return tool
}

// createWeatherTool creates a mock weather information tool
func createWeatherTool() core.BaseTool {
	tool, err := tools.NewFunctionTool(
		"weather",
		"Gets weather information for a specified location",
		func(location string) map[string]interface{} {
			// Mock weather data - in a real implementation, this would call a weather API
			weatherData := map[string]map[string]interface{}{
				"tokyo": {
					"temperature": 22,
					"condition":   "sunny",
					"humidity":    65,
					"windSpeed":   12,
				},
				"new york": {
					"temperature": 18,
					"condition":   "cloudy",
					"humidity":    72,
					"windSpeed":   8,
				},
				"london": {
					"temperature": 15,
					"condition":   "rainy",
					"humidity":    85,
					"windSpeed":   15,
				},
			}

			if data, exists := weatherData[location]; exists {
				return data
			}

			// Default weather for unknown locations
			return map[string]interface{}{
				"temperature": 20,
				"condition":   "partly cloudy",
				"humidity":    70,
				"windSpeed":   10,
				"note":        fmt.Sprintf("Simulated weather data for %s", location),
			}
		},
	)
	if err != nil {
		log.Fatalf("Failed to create weather tool: %v", err)
	}
	return tool
}

// createExampleAgents creates sample agents with different capabilities
func createExampleAgents(greetingTool, calculatorTool, weatherTool core.BaseTool) map[string]core.BaseAgent {
	agentMap := make(map[string]core.BaseAgent)

	// Assistant Agent - general purpose with all tools
	assistant := agents.NewLLMAgent(
		"assistant",
		"A helpful general-purpose assistant",
		"gemini-2.0-flash",
	)
	assistant.SetInstruction("You are a helpful assistant that can greet users, perform calculations, and provide weather information. Always be polite and helpful.")
	assistant.AddTool(greetingTool)
	assistant.AddTool(calculatorTool)
	assistant.AddTool(weatherTool)
	agentMap["assistant"] = assistant

	// Math Agent - specialized for calculations
	mathAgent := agents.NewLLMAgent(
		"math_specialist",
		"A specialized agent for mathematical calculations",
		"gemini-2.0-flash",
	)
	mathAgent.SetInstruction("You are a mathematics specialist. Focus on providing accurate calculations and mathematical insights.")
	mathAgent.AddTool(calculatorTool)
	agentMap["math_specialist"] = mathAgent

	// Weather Agent - specialized for weather information
	weatherAgent := agents.NewLLMAgent(
		"weather_specialist",
		"A specialized agent for weather information",
		"gemini-2.0-flash",
	)
	weatherAgent.SetInstruction("You are a weather specialist. Provide detailed weather information and forecasts.")
	weatherAgent.AddTool(weatherTool)
	agentMap["weather_specialist"] = weatherAgent

	// Greeter Agent - simple greeting functionality
	greeterAgent := agents.NewBaseAgent(
		"greeter",
		"A friendly greeting agent",
	)
	greeterAgent.SetInstruction("You are a friendly greeter. Always welcome users warmly.")
	// Note: BaseAgent would need tool support, this is for demonstration
	agentMap["greeter"] = greeterAgent

	return agentMap
}

// createAgentCards creates metadata cards for the agents
func createAgentCards() map[string]*a2a.AgentCard {
	cards := make(map[string]*a2a.AgentCard)

	// Assistant card
	cards["assistant"] = &a2a.AgentCard{
		Name:        "assistant",
		Description: stringPtr("A helpful general-purpose assistant that can handle greetings, calculations, and weather queries"),
		URL:         "http://localhost:8080/a2a",
		Version:     "1.0.0",
		Provider: &a2a.AgentProvider{
			Organization: "ADK Demo",
			URL:          stringPtr("https://github.com/agent-protocol/adk-golang"),
		},
		Capabilities: a2a.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{
			{
				ID:          "greeting",
				Name:        "Greeting",
				Description: stringPtr("Generate personalized greeting messages"),
				Tags:        []string{"social", "conversation"},
				Examples:    []string{"Hello there!", "Good morning, John", "Greet me"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "calculation",
				Name:        "Calculator",
				Description: stringPtr("Perform mathematical calculations"),
				Tags:        []string{"math", "calculation"},
				Examples:    []string{"Calculate 2+2", "What is 15 * 7?", "Divide 100 by 5"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "weather",
				Name:        "Weather Information",
				Description: stringPtr("Get weather information for any location"),
				Tags:        []string{"weather", "forecast"},
				Examples:    []string{"Weather in Tokyo", "What's the temperature in New York?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}

	// Math specialist card
	cards["math_specialist"] = &a2a.AgentCard{
		Name:        "math_specialist",
		Description: stringPtr("A specialized agent for mathematical calculations and problem solving"),
		URL:         "http://localhost:8080/a2a",
		Version:     "1.0.0",
		Provider: &a2a.AgentProvider{
			Organization: "ADK Demo",
			URL:          stringPtr("https://github.com/agent-protocol/adk-golang"),
		},
		Capabilities: a2a.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{
			{
				ID:          "calculation",
				Name:        "Advanced Calculator",
				Description: stringPtr("Perform complex mathematical calculations"),
				Tags:        []string{"math", "calculation", "advanced"},
				Examples:    []string{"Calculate compound interest", "Solve quadratic equations", "Statistical analysis"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}

	// Weather specialist card
	cards["weather_specialist"] = &a2a.AgentCard{
		Name:        "weather_specialist",
		Description: stringPtr("A specialized agent for weather information and forecasts"),
		URL:         "http://localhost:8080/a2a",
		Version:     "1.0.0",
		Provider: &a2a.AgentProvider{
			Organization: "ADK Demo",
			URL:          stringPtr("https://github.com/agent-protocol/adk-golang"),
		},
		Capabilities: a2a.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      false,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{
			{
				ID:          "weather",
				Name:        "Weather Forecast",
				Description: stringPtr("Comprehensive weather information and forecasting"),
				Tags:        []string{"weather", "forecast", "meteorology"},
				Examples:    []string{"7-day forecast for Paris", "Current conditions in Sydney", "Weather alerts"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}

	// Greeter card
	cards["greeter"] = &a2a.AgentCard{
		Name:        "greeter",
		Description: stringPtr("A friendly agent specialized in greetings and welcomes"),
		URL:         "http://localhost:8080/a2a",
		Version:     "1.0.0",
		Provider: &a2a.AgentProvider{
			Organization: "ADK Demo",
			URL:          stringPtr("https://github.com/agent-protocol/adk-golang"),
		},
		Capabilities: a2a.AgentCapabilities{
			Streaming:              false,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{
			{
				ID:          "greeting",
				Name:        "Personal Greeting",
				Description: stringPtr("Warm and personalized greeting messages"),
				Tags:        []string{"greeting", "welcome", "social"},
				Examples:    []string{"Welcome new users", "Say hello", "Introduce yourself"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}

	return cards
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Helper function to safely get string value from pointer
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
