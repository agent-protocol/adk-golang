package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/a2a/executor"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/runners"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

// ExampleAgent is a simple agent that echoes back user messages.
type ExampleAgent struct {
	*agents.BaseAgentImpl
}

func NewExampleAgent() *ExampleAgent {
	base := agents.NewBaseAgent("example-agent", "A simple example agent for A2A demo")
	return &ExampleAgent{
		BaseAgentImpl: base,
	}
}

func (a *ExampleAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 1)

	go func() {
		defer close(eventChan)

		// Create a response event
		event := core.NewEvent(invocationCtx.InvocationID, a.Name())
		event.Content = &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Hello! I received your message and I'm responding via A2A protocol."),
				},
			},
		}
		event.TurnComplete = boolPtr(true)

		select {
		case eventChan <- event:
		case <-ctx.Done():
		}
	}()

	return eventChan, nil
}

func main() {
	fmt.Println("A2A Agent Executor Demo")
	fmt.Println("=======================")

	// Create an example agent
	agent := NewExampleAgent()

	// Create session service
	sessionService := sessions.NewInMemorySessionService()

	// Create runner
	runner := runners.NewRunner("a2a-demo", agent, sessionService)

	// Create A2A agent executor
	config := &executor.A2aAgentExecutorConfig{
		EnableDebugLogging:    true,
		Timeout:               30 * time.Second,
		MaxConcurrentRequests: 5,
	}
	a2aExecutor := executor.NewA2aAgentExecutor(runner, config)

	// Create a test A2A request
	requestCtx := &executor.RequestContext{
		TaskID:    "demo-task-123",
		ContextID: "demo-context-456",
		Message: &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr("Hello, A2A agent! How are you today?"),
				},
			},
		},
		UserID:    "demo-user-789",
		SessionID: "demo-session-abc",
	}

	// Create event queue to capture A2A events
	eventQueue := executor.NewSimpleEventQueue(20)
	defer eventQueue.Close()

	// Start a goroutine to monitor events
	go func() {
		fmt.Println("\nMonitoring A2A Events:")
		fmt.Println("----------------------")

		for event := range eventQueue.Events() {
			switch e := event.(type) {
			case *a2a.TaskStatusUpdateEvent:
				fmt.Printf("📋 Task Status Update: %s\n", e.Status.State)
				if e.Status.Message != nil && len(e.Status.Message.Parts) > 0 {
					for i, part := range e.Status.Message.Parts {
						if part.Type == "text" && part.Text != nil {
							fmt.Printf("   Part %d (text): %s\n", i+1, *part.Text)
						} else if part.Type == "data" {
							fmt.Printf("   Part %d (data): %v\n", i+1, part.Data)
						}
					}
				}
				if e.Final {
					fmt.Printf("   ✅ Final event - task completed\n")
				}
				fmt.Println()

			case *a2a.TaskArtifactUpdateEvent:
				fmt.Printf("📎 Artifact Update: %s\n", *e.Artifact.Name)
				if e.Artifact.Metadata != nil {
					fmt.Printf("   Metadata: %v\n", e.Artifact.Metadata)
				}
				fmt.Println()

			default:
				fmt.Printf("❓ Unknown event type: %T\n", event)
				fmt.Println()
			}
		}
	}()

	// Execute the A2A request
	fmt.Printf("Executing A2A request for task: %s\n", requestCtx.TaskID)
	fmt.Printf("User message: %s\n", *requestCtx.Message.Parts[0].Text)

	ctx := context.Background()
	err := a2aExecutor.Execute(ctx, requestCtx, eventQueue)
	if err != nil {
		log.Fatalf("A2A execution failed: %v", err)
	}

	// Wait a bit for events to be processed
	time.Sleep(2 * time.Second)

	fmt.Println("Demo completed successfully!")
	fmt.Println("\nThis demo showed:")
	fmt.Println("1. Creating an A2A Agent Executor")
	fmt.Println("2. Converting A2A requests to ADK format")
	fmt.Println("3. Executing ADK agents via A2A protocol")
	fmt.Println("4. Converting ADK events back to A2A events")
	fmt.Println("5. Streaming A2A events in real-time")
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
