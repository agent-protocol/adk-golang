// Package main demonstrates how to use the Google Search tool and Enhanced Agent Tool.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

func main() {
	fmt.Println("=== Google Search Tool Example ===")
	demonstrateGoogleSearchTool()

	fmt.Println("\n=== Enhanced Agent Tool Example ===")
	demonstrateEnhancedAgentTool()

	fmt.Println("\n=== Multi-Agent Workflow Example ===")
	demonstrateMultiAgentWorkflow()
}

// demonstrateGoogleSearchTool shows how to use the Google Search tool
func demonstrateGoogleSearchTool() {
	// Create the Google Search tool
	googleSearch := tools.GlobalGoogleSearchTool

	fmt.Printf("Tool Name: %s\n", googleSearch.Name())
	fmt.Printf("Tool Description: %s\n", googleSearch.Description())

	// Create an LLM agent with Google Search capability
	config := &agents.LlmAgentConfig{
		Model: "gemini-2.0-flash",
	}
	searchAgent := agents.NewEnhancedLlmAgent(
		"search_agent",
		"An agent that can search the web using Google Search",
		config,
	)

	// Add the Google Search tool
	searchAgent.AddTool(googleSearch)

	fmt.Printf("Created agent: %s with Google Search capability\n", searchAgent.Name())

	// In a real implementation, you would run the agent with a session
	// This demonstrates the tool configuration
	fmt.Println("Google Search tool configured successfully!")
}

// demonstrateEnhancedAgentTool shows how to use the Enhanced Agent Tool
func demonstrateEnhancedAgentTool() {
	// Create a specialist agent
	instruction := "You are an expert mathematician. Solve mathematical problems step by step."
	config := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &instruction,
	}
	mathAgent := agents.NewEnhancedLlmAgent(
		"math_specialist",
		"A specialist agent for mathematical calculations and problem solving",
		config,
	)

	// Create an Enhanced Agent Tool that wraps the math agent
	mathTool := tools.NewEnhancedAgentTool(mathAgent)

	fmt.Printf("Created Enhanced Agent Tool: %s\n", mathTool.Name())
	fmt.Printf("Tool Description: %s\n", mathTool.Description())

	// Check the tool declaration
	declaration := mathTool.GetDeclaration()
	if declaration != nil {
		fmt.Printf("Function Declaration: %s\n", declaration.Name)
		fmt.Printf("Parameters: %v\n", declaration.Parameters)
	}

	// Create a coordinator agent that can use the math specialist
	coordinatorInstruction := "You coordinate tasks and delegate to appropriate specialist agents."
	coordinatorConfig := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &coordinatorInstruction,
	}
	coordinatorAgent := agents.NewEnhancedLlmAgent(
		"coordinator",
		"A coordinator agent that delegates tasks to specialist agents",
		coordinatorConfig,
	)
	coordinatorAgent.AddTool(mathTool)

	fmt.Printf("Created coordinator agent: %s with math specialist tool\n", coordinatorAgent.Name())
}

// demonstrateMultiAgentWorkflow shows a complex multi-agent setup
func demonstrateMultiAgentWorkflow() {
	// Create specialist agents
	researchInstruction := "You are a research specialist. Gather and analyze information thoroughly."
	researchConfig := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &researchInstruction,
	}
	researchAgent := agents.NewEnhancedLlmAgent(
		"researcher",
		"Research specialist for gathering information",
		researchConfig,
	)
	researchAgent.AddTool(tools.GlobalGoogleSearchTool)

	analysisInstruction := "You are a data analyst. Analyze information and provide insights."
	analysisConfig := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &analysisInstruction,
	}
	analysisAgent := agents.NewEnhancedLlmAgent(
		"analyst",
		"Data analysis specialist",
		analysisConfig,
	)

	writingInstruction := "You are a professional writer. Create clear, engaging content."
	writingConfig := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &writingInstruction,
	}
	writingAgent := agents.NewEnhancedLlmAgent(
		"writer",
		"Content writing specialist",
		writingConfig,
	)

	// Create Enhanced Agent Tools for each specialist
	researchTool := tools.NewEnhancedAgentToolWithConfig(researchAgent, &tools.AgentToolConfig{
		IsolateState:      false, // Allow sharing state
		ErrorStrategy:     tools.ErrorStrategyReturnError,
		CustomInstruction: "Focus on gathering comprehensive information",
	})

	analysisTool := tools.NewEnhancedAgentToolWithConfig(analysisAgent, &tools.AgentToolConfig{
		IsolateState:      false,
		ErrorStrategy:     tools.ErrorStrategyReturnError,
		CustomInstruction: "Provide detailed analysis and insights",
	})

	writingTool := tools.NewEnhancedAgentToolWithConfig(writingAgent, &tools.AgentToolConfig{
		IsolateState:      false,
		ErrorStrategy:     tools.ErrorStrategyReturnError,
		CustomInstruction: "Create professional, well-structured content",
	})

	// Create a master coordinator agent
	masterInstruction := `You are a master coordinator that manages complex workflows using specialist agents:
			1. Use the researcher for information gathering
			2. Use the analyst for data analysis  
			3. Use the writer for content creation
			Coordinate their work to produce comprehensive results.`
	masterConfig := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &masterInstruction,
	}
	masterAgent := agents.NewEnhancedLlmAgent(
		"master_coordinator",
		"Master coordinator for complex multi-agent workflows",
		masterConfig,
	)
	masterAgent.AddTool(researchTool)
	masterAgent.AddTool(analysisTool)
	masterAgent.AddTool(writingTool)

	fmt.Printf("Created multi-agent workflow with:\n")
	fmt.Printf("- Master Coordinator: %s\n", masterAgent.Name())
	fmt.Printf("- Research Tool: %s\n", researchTool.Name())
	fmt.Printf("- Analysis Tool: %s\n", analysisTool.Name())
	fmt.Printf("- Writing Tool: %s\n", writingTool.Name())

	// Demonstrate a complex workflow
	fmt.Println("\nWorkflow example:")
	fmt.Println("1. Master coordinator receives a complex research task")
	fmt.Println("2. Delegates research to research agent (with Google Search)")
	fmt.Println("3. Passes research results to analysis agent")
	fmt.Println("4. Sends analysis to writing agent for final report")
	fmt.Println("5. Coordinates the entire process and returns final result")
}

// Example of how to actually run an agent (this would be in a real application)
func exampleAgentExecution() {
	ctx := context.Background()

	// Create a simple agent with Google Search
	instruction := "Answer questions using web search when needed."
	config := &agents.LlmAgentConfig{
		Model:             "gemini-2.0-flash",
		SystemInstruction: &instruction,
	}
	agent := agents.NewEnhancedLlmAgent(
		"example_agent",
		"Example agent with search capability",
		config,
	)
	agent.AddTool(tools.GlobalGoogleSearchTool)

	// Create session (simplified)
	session := core.NewSession("session_1", "example_app", "user_1")

	// Create invocation context
	invocationCtx := core.NewInvocationContext("invocation_1", agent, session, nil)

	// Set user input
	invocationCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("What are the latest developments in AI?"),
			},
		},
	}

	// Run the agent (this would require proper LLM connection setup)
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		log.Printf("Error running agent: %v", err)
		return
	}

	// Process results
	for _, event := range events {
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != nil {
					fmt.Printf("Agent response: %s\n", *part.Text)
				}
			}
		}
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
