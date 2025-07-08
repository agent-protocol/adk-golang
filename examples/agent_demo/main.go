package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

func main() {
	fmt.Println("=== ADK Go Agent Demo ===")

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a simple in-memory session service
	sessionService := sessions.NewInMemorySessionService()

	// Create a session
	session, err := sessionService.CreateSession(ctx, &core.CreateSessionRequest{
		AppName: "agent_demo",
		UserID:  "user123",
		State:   map[string]any{"initialized": true},
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	fmt.Printf("Created session: %s\n", session.ID)

	// Demo 1: Basic Agent
	fmt.Println("\n--- Demo 1: Basic Agent ---")
	basicAgent := agents.NewBaseAgent("basic", "A simple base agent")
	basicAgent.SetInstruction("You are a helpful assistant")

	// Set up callbacks
	basicAgent.SetBeforeAgentCallback(func(ctx context.Context, invocationCtx *core.InvocationContext) error {
		fmt.Printf("Before agent callback: Starting execution for agent %s\n", invocationCtx.Agent.Name())
		return nil
	})

	basicAgent.SetAfterAgentCallback(func(ctx context.Context, invocationCtx *core.InvocationContext, events []*core.Event) error {
		fmt.Printf("After agent callback: Completed with %d events\n", len(events))
		return nil
	})

	runBasicAgent(ctx, basicAgent, session, sessionService)

	// Demo 2: Sequential Agent with Sub-agents
	fmt.Println("\n--- Demo 2: Sequential Agent with Sub-agents ---")
	sequentialAgent := agents.NewSequentialAgent("sequential", "Executes sub-agents in order")

	// Create sub-agents
	subAgent1 := agents.NewBaseAgent("sub1", "First sub-agent")
	subAgent2 := agents.NewBaseAgent("sub2", "Second sub-agent")

	sequentialAgent.AddSubAgent(subAgent1)
	sequentialAgent.AddSubAgent(subAgent2)

	runSequentialAgent(ctx, sequentialAgent, session, sessionService)

	// Demo 3: Agent Hierarchy and FindAgent
	fmt.Println("\n--- Demo 3: Agent Hierarchy ---")
	demonstrateAgentHierarchy(ctx, sequentialAgent)
}

func runBasicAgent(ctx context.Context, agent core.BaseAgent, session *core.Session, sessionService core.SessionService) {
	// Create user content
	userContent := &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("Hello, how can you help me?"),
			},
		},
	}

	// Create invocation context
	invocationCtx := core.NewInvocationContext(
		generateInvocationID(),
		agent,
		session,
		sessionService,
	).WithUserContent(userContent)

	// Run the agent synchronously
	events, err := agent.Run(ctx, invocationCtx)
	if err != nil {
		log.Printf("Error running agent: %v", err)
		return
	}

	fmt.Printf("Agent %s completed with %d events:\n", agent.Name(), len(events))
	for i, event := range events {
		fmt.Printf("  Event %d: Author=%s, Content=%v\n", i+1, event.Author, getEventText(event))
	}
}

func runSequentialAgent(ctx context.Context, agent core.BaseAgent, session *core.Session, sessionService core.SessionService) {
	// Create user content
	userContent := &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: stringPtr("Execute all sub-agents"),
			},
		},
	}

	// Create invocation context with branch
	invocationCtx := core.NewInvocationContext(
		generateInvocationID(),
		agent,
		session,
		sessionService,
	).WithUserContent(userContent).WithBranch("main")

	// Run the agent asynchronously to demonstrate streaming
	stream, err := agent.RunAsync(ctx, invocationCtx)
	if err != nil {
		log.Printf("Error running sequential agent: %v", err)
		return
	}

	fmt.Printf("Sequential agent %s started, streaming events:\n", agent.Name())
	eventCount := 0
	for event := range stream {
		eventCount++
		fmt.Printf("  Stream Event %d: Author=%s, Branch=%s, Content=%v\n",
			eventCount, event.Author, getBranchString(event.Branch), getEventText(event))
	}
	fmt.Printf("Sequential agent completed with %d total events\n", eventCount)
}

func demonstrateAgentHierarchy(ctx context.Context, rootAgent core.BaseAgent) {
	fmt.Printf("Root agent: %s (%s)\n", rootAgent.Name(), rootAgent.Description())
	fmt.Printf("Sub-agents: %d\n", len(rootAgent.SubAgents()))

	for _, subAgent := range rootAgent.SubAgents() {
		fmt.Printf("  - %s: %s\n", subAgent.Name(), subAgent.Description())
		fmt.Printf("    Parent: %s\n", getParentName(subAgent.ParentAgent()))
	}

	// Test FindAgent functionality
	fmt.Println("\nTesting FindAgent:")
	testCases := []string{"sequential", "sub1", "sub2", "nonexistent"}

	for _, name := range testCases {
		found := rootAgent.FindAgent(name)
		if found != nil {
			fmt.Printf("  FindAgent('%s') -> Found: %s\n", name, found.Name())
		} else {
			fmt.Printf("  FindAgent('%s') -> Not found\n", name)
		}
	}

	// Test FindSubAgent functionality
	fmt.Println("\nTesting FindSubAgent:")
	for _, name := range testCases {
		found := rootAgent.FindSubAgent(name)
		if found != nil {
			fmt.Printf("  FindSubAgent('%s') -> Found: %s\n", name, found.Name())
		} else {
			fmt.Printf("  FindSubAgent('%s') -> Not found\n", name)
		}
	}
}

// Helper functions

func generateInvocationID() string {
	return fmt.Sprintf("inv_%d", time.Now().UnixNano())
}

func stringPtr(s string) *string {
	return &s
}

func getEventText(event *core.Event) string {
	if event.Content == nil {
		return "<no content>"
	}
	for _, part := range event.Content.Parts {
		if part.Text != nil {
			return *part.Text
		}
	}
	return "<no text>"
}

func getBranchString(branch *string) string {
	if branch == nil {
		return "<no branch>"
	}
	return *branch
}

func getParentName(parent core.BaseAgent) string {
	if parent == nil {
		return "<no parent>"
	}
	return parent.Name()
}
