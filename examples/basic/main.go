// Package main demonstrates the Go ADK interfaces and implementations.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/runners"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

func main() {
	ctx := context.Background()

	// Create a simple greeting tool
	greetingTool, err := tools.NewFunctionTool(
		"greeting",
		"Generates a greeting message",
		func(name string) string {
			return fmt.Sprintf("Hello, %s! Welcome to ADK for Go!", name)
		},
	)
	if err != nil {
		log.Fatalf("Failed to create greeting tool: %v", err)
	}

	// Create a simple math tool
	mathTool, err := tools.NewFunctionTool(
		"add_numbers",
		"Adds two numbers together",
		func(a, b float64) float64 {
			return a + b
		},
	)
	if err != nil {
		log.Fatalf("Failed to create math tool: %v", err)
	}

	// Create a base agent (we'll use this as a simple echo agent)
	baseAgent := agents.NewBaseAgent(
		"echo_agent",
		"A simple agent that echoes messages with available tools",
	)
	baseAgent.SetInstruction("You are a helpful assistant that can greet users and perform simple math.")

	// Create an LLM agent (without actual LLM connection for this demo)
	llmAgent := agents.NewLLMAgent(
		"llm_agent",
		"An agent powered by a language model",
		"gemini-2.0-flash",
	)
	llmAgent.AddTool(greetingTool)
	llmAgent.AddTool(mathTool)

	// Create a sequential agent that coordinates multiple agents
	coordinator := agents.NewSequentialAgent(
		"coordinator",
		"Coordinates multiple agents in sequence",
	)
	coordinator.AddSubAgent(baseAgent)
	coordinator.AddSubAgent(llmAgent)

	// Create services
	sessionService := sessions.NewInMemorySessionService()

	// Create runner
	runner := runners.NewRunner("demo_app", coordinator, sessionService)

	// Run a simple interaction
	fmt.Println("🤖 ADK Go Demo Starting...")
	fmt.Println("=========================")

	// Test 1: Simple message
	fmt.Println("\n📝 Test 1: Simple Message")
	runRequest := &core.RunRequest{
		UserID:    "demo_user",
		SessionID: "demo_session",
		NewMessage: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Hello! Can you greet me?"),
				},
			},
		},
	}

	events, err := runner.Run(ctx, runRequest)
	if err != nil {
		log.Fatalf("Failed to run agent: %v", err)
	}

	printEvents("Simple Message", events)

	// Test 2: Tool usage demonstration
	fmt.Println("\n🔧 Test 2: Tool Declaration Demo")

	// Show tool declarations
	fmt.Println("Available tools:")
	for _, tool := range []core.BaseTool{greetingTool, mathTool} {
		if decl := tool.GetDeclaration(); decl != nil {
			fmt.Printf("  - %s: %s\n", decl.Name, decl.Description)
		}
	}

	// Test 3: Agent hierarchy
	fmt.Println("\n🌳 Test 3: Agent Hierarchy")
	fmt.Printf("Root agent: %s\n", coordinator.Name())
	fmt.Printf("Sub-agents:\n")
	for _, subAgent := range coordinator.SubAgents() {
		fmt.Printf("  - %s: %s\n", subAgent.Name(), subAgent.Description())
	}

	// Test 4: Session management
	fmt.Println("\n💾 Test 4: Session Management")

	// List sessions
	listReq := &core.ListSessionsRequest{
		AppName: "demo_app",
		UserID:  "demo_user",
	}

	sessionsResp, err := sessionService.ListSessions(ctx, listReq)
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}

	fmt.Printf("Found %d sessions for user demo_user\n", len(sessionsResp.Sessions))
	for _, session := range sessionsResp.Sessions {
		fmt.Printf("  - Session ID: %s, Last Update: %v\n",
			session.ID, session.LastUpdateTime.Format("15:04:05"))
	}

	// Test 5: Event streaming simulation
	fmt.Println("\n🌊 Test 5: Event Streaming")

	streamReq := &core.RunRequest{
		UserID:    "demo_user",
		SessionID: "stream_session",
		NewMessage: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Please process this request step by step"),
				},
			},
		},
	}

	eventStream, err := runner.RunAsync(ctx, streamReq)
	if err != nil {
		log.Fatalf("Failed to start event stream: %v", err)
	}

	fmt.Println("Processing events from stream:")
	eventCount := 0
	for event := range eventStream {
		eventCount++
		fmt.Printf("  Event %d: Author=%s, Type=%s\n",
			eventCount, event.Author, getEventType(event))

		if event.ErrorMessage != nil {
			fmt.Printf("    Error: %s\n", *event.ErrorMessage)
		}
	}

	fmt.Printf("Received %d events total\n", eventCount)

	// Cleanup
	fmt.Println("\n🧹 Cleanup")
	if err := runner.Close(ctx); err != nil {
		log.Printf("Warning: Failed to close runner: %v", err)
	} else {
		fmt.Println("Runner closed successfully")
	}

	fmt.Println("\n✅ ADK Go Demo Complete!")
}

// printEvents prints a summary of events.
func printEvents(testName string, events []*core.Event) {
	fmt.Printf("Results for %s:\n", testName)
	for i, event := range events {
		fmt.Printf("  Event %d: Author=%s", i+1, event.Author)

		if event.Content != nil && len(event.Content.Parts) > 0 {
			for _, part := range event.Content.Parts {
				if part.Text != nil {
					fmt.Printf(", Text=%q", truncateString(*part.Text, 50))
				}
				if part.FunctionCall != nil {
					fmt.Printf(", FunctionCall=%s", part.FunctionCall.Name)
				}
				if part.FunctionResponse != nil {
					fmt.Printf(", FunctionResponse=%s", part.FunctionResponse.Name)
				}
			}
		}

		if event.ErrorMessage != nil {
			fmt.Printf(", Error=%s", *event.ErrorMessage)
		}

		fmt.Println()
	}
}

// getEventType determines the type of an event.
func getEventType(event *core.Event) string {
	if event.ErrorMessage != nil {
		return "error"
	}
	if event.Content == nil {
		return "empty"
	}
	if len(event.Content.Parts) == 0 {
		return "no_parts"
	}

	for _, part := range event.Content.Parts {
		if part.Text != nil {
			return "text"
		}
		if part.FunctionCall != nil {
			return "function_call"
		}
		if part.FunctionResponse != nil {
			return "function_response"
		}
	}

	return "unknown"
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stringPtr returns a pointer to a string literal.
func stringPtr(s string) *string {
	return &s
}
