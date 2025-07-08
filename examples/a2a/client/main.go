// Package main demonstrates how to use the A2A client to communicate with remote agents.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

func main() {
	ctx := context.Background()

	fmt.Println("🤖 A2A Client Demo")
	fmt.Println("==================")

	// Demo 1: Agent Discovery
	fmt.Println("\n🔍 Demo 1: Agent Discovery")
	agentCard := discoverAgent(ctx)
	if agentCard != nil {
		printAgentInfo(agentCard)
	}

	// Demo 2: Basic Message Sending
	fmt.Println("\n💬 Demo 2: Basic Message Sending")
	if agentCard != nil {
		sendBasicMessage(ctx, agentCard)
	}

	// Demo 3: Streaming Communication
	fmt.Println("\n🌊 Demo 3: Streaming Communication")
	if agentCard != nil {
		sendStreamingMessage(ctx, agentCard)
	}

	// Demo 4: Task Management
	fmt.Println("\n📋 Demo 4: Task Management")
	if agentCard != nil {
		demonstrateTaskManagement(ctx, agentCard)
	}

	// Demo 5: Error Handling
	fmt.Println("\n⚠️  Demo 5: Error Handling")
	demonstrateErrorHandling(ctx)

	fmt.Println("\n✅ A2A Client Demo Complete!")
}

// discoverAgent demonstrates agent discovery using well-known endpoints
func discoverAgent(ctx context.Context) *a2a.AgentCard {
	fmt.Println("Discovering agents at http://localhost:8080...")

	// Create agent card resolver
	resolver := a2a.NewAgentCardResolver("http://localhost:8080", nil)

	// Try to get the well-known agent card
	agentCard, err := resolver.GetWellKnownAgentCard(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to discover agent: %v\n", err)
		fmt.Println("💡 Make sure the A2A server is running (go run examples/a2a/server/main.go)")
		return nil
	}

	fmt.Printf("✅ Discovered agent: %s\n", agentCard.Name)
	return agentCard
}

// printAgentInfo displays information about the discovered agent
func printAgentInfo(agentCard *a2a.AgentCard) {
	fmt.Printf("📊 Agent Information:\n")
	fmt.Printf("  Name: %s\n", agentCard.Name)
	if agentCard.Description != nil {
		fmt.Printf("  Description: %s\n", *agentCard.Description)
	}
	fmt.Printf("  Version: %s\n", agentCard.Version)
	fmt.Printf("  URL: %s\n", agentCard.URL)

	if agentCard.Provider != nil {
		fmt.Printf("  Provider: %s\n", agentCard.Provider.Organization)
	}

	// Display capabilities
	fmt.Printf("  Capabilities:\n")
	fmt.Printf("    Streaming: %t\n", agentCard.Capabilities.Streaming)
	fmt.Printf("    Push Notifications: %t\n", agentCard.Capabilities.PushNotifications)
	fmt.Printf("    State History: %t\n", agentCard.Capabilities.StateTransitionHistory)

	// Display skills
	if len(agentCard.Skills) > 0 {
		fmt.Printf("  Skills:\n")
		for _, skill := range agentCard.Skills {
			fmt.Printf("    - %s: %s\n", skill.Name, getStringValue(skill.Description))
			if len(skill.Examples) > 0 {
				fmt.Printf("      Examples: %v\n", skill.Examples)
			}
		}
	}
}

// sendBasicMessage demonstrates sending a simple message to an agent
func sendBasicMessage(ctx context.Context, agentCard *a2a.AgentCard) {
	// Create client
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	// Create a simple message
	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr("Hello! Can you greet me and tell me a bit about yourself?"),
			},
		},
	}

	// Send message
	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
	}

	fmt.Printf("📤 Sending message: %s\n", *message.Parts[0].Text)

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Failed to send message: %v\n", err)
		return
	}

	fmt.Printf("✅ Message sent successfully!\n")
	fmt.Printf("📋 Task ID: %s\n", task.ID)
	fmt.Printf("📊 Task Status: %s\n", task.Status.State)

	// Poll for completion
	pollForCompletion(ctx, client, task.ID)
}

// sendStreamingMessage demonstrates streaming communication with an agent
func sendStreamingMessage(ctx context.Context, agentCard *a2a.AgentCard) {
	// Create client
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	// Create a message that might generate multiple responses
	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr("Please calculate 15 * 7, then get the weather for Tokyo, and finally greet me with a good morning message."),
			},
		},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
	}

	fmt.Printf("📤 Sending streaming message: %s\n", *message.Parts[0].Text)
	fmt.Println("🌊 Listening for streaming events...")

	// Send streaming message
	eventCount := 0
	err = client.SendMessageStream(ctx, params, func(response *a2a.SendTaskStreamingResponse) error {
		eventCount++

		if response.Error != nil {
			fmt.Printf("❌ Stream error: %s\n", response.Error.Message)
			return nil
		}

		// Try to parse the result as different event types
		if response.Result != nil {
			resultBytes, err := json.Marshal(response.Result)
			if err != nil {
				fmt.Printf("⚠️  Could not marshal result: %v\n", err)
				return nil
			}

			// Try to parse as status update
			var statusUpdate a2a.TaskStatusUpdateEvent
			if err := json.Unmarshal(resultBytes, &statusUpdate); err == nil && statusUpdate.ID != "" {
				fmt.Printf("📊 Status Update [%d]: Task %s -> %s\n",
					eventCount, statusUpdate.ID, statusUpdate.Status.State)

				if statusUpdate.Status.Message != nil && len(statusUpdate.Status.Message.Parts) > 0 {
					for _, part := range statusUpdate.Status.Message.Parts {
						if part.Text != nil {
							fmt.Printf("💬 Message: %s\n", *part.Text)
						}
					}
				}

				if statusUpdate.Final {
					fmt.Println("🏁 Task completed")
					return nil
				}
			} else {
				// Try to parse as artifact update
				var artifactUpdate a2a.TaskArtifactUpdateEvent
				if err := json.Unmarshal(resultBytes, &artifactUpdate); err == nil && artifactUpdate.ID != "" {
					fmt.Printf("📄 Artifact Update [%d]: Task %s\n", eventCount, artifactUpdate.ID)
					if artifactUpdate.Artifact.Name != nil {
						fmt.Printf("📎 Artifact: %s\n", *artifactUpdate.Artifact.Name)
					}
				} else {
					fmt.Printf("📨 Raw Event [%d]: %s\n", eventCount, string(resultBytes))
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("❌ Streaming failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Streaming completed! Received %d events\n", eventCount)
}

// demonstrateTaskManagement shows task lifecycle management
func demonstrateTaskManagement(ctx context.Context, agentCard *a2a.AgentCard) {
	// Create client
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Failed to create client: %v\n", err)
		return
	}
	defer client.Close()

	// Start a long-running task
	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr("Please perform a complex calculation: calculate the factorial of 10, then get weather for multiple cities."),
			},
		},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
	}

	fmt.Printf("📤 Starting task: %s\n", params.ID)

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Failed to send message: %v\n", err)
		return
	}

	fmt.Printf("✅ Task started: %s\n", task.ID)

	// Wait a moment, then check task status
	time.Sleep(1 * time.Second)

	fmt.Println("🔍 Checking task status...")
	queryParams := &a2a.TaskQueryParams{
		ID: task.ID,
	}

	retrievedTask, err := client.GetTask(ctx, queryParams)
	if err != nil {
		fmt.Printf("❌ Failed to get task: %v\n", err)
		return
	}

	fmt.Printf("📊 Current status: %s\n", retrievedTask.Status.State)

	// Demonstrate task cancellation (after some delay)
	time.Sleep(2 * time.Second)

	fmt.Println("🛑 Attempting to cancel task...")
	cancelParams := &a2a.TaskIdParams{
		ID: task.ID,
	}

	canceledTask, err := client.CancelTask(ctx, cancelParams)
	if err != nil {
		fmt.Printf("⚠️  Could not cancel task: %v\n", err)
		// Task might have already completed
	} else {
		fmt.Printf("✅ Task canceled: %s\n", canceledTask.Status.State)
	}
}

// demonstrateErrorHandling shows various error scenarios
func demonstrateErrorHandling(ctx context.Context) {
	fmt.Println("Testing various error scenarios...")

	// Test 1: Invalid agent URL
	fmt.Println("\n1. Invalid agent URL:")
	invalidCard := &a2a.AgentCard{
		Name:    "invalid-agent",
		URL:     "http://localhost:9999/nonexistent",
		Version: "1.0.0",
		Capabilities: a2a.AgentCapabilities{
			Streaming: false,
		},
	}

	client, err := a2a.NewClient(invalidCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr("Hello"),
			},
		},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
	}

	_, err = client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("✅ Expected error caught: %v\n", err)
	}

	client.Close()

	// Test 2: Invalid task ID
	fmt.Println("\n2. Invalid task query:")

	// Try with a valid server but invalid task ID
	validCard := &a2a.AgentCard{
		Name:    "test-agent",
		URL:     "http://localhost:8080/a2a",
		Version: "1.0.0",
		Capabilities: a2a.AgentCapabilities{
			Streaming: false,
		},
	}

	client2, err := a2a.NewClient(validCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client2.Close()

	queryParams := &a2a.TaskQueryParams{
		ID: "nonexistent-task-id",
	}

	_, err = client2.GetTask(ctx, queryParams)
	if err != nil {
		fmt.Printf("✅ Expected error caught: %v\n", err)
	}

	// Test 3: Timeout scenario
	fmt.Println("\n3. Timeout scenario:")

	// Create client with very short timeout
	shortTimeoutConfig := &a2a.ClientConfig{
		Timeout: 100 * time.Millisecond, // Very short timeout
		BaseURL: "http://localhost:8080/a2a",
	}

	client3, err := a2a.NewClient(validCard, shortTimeoutConfig)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client3.Close()

	// This might timeout if the server takes longer than 100ms
	_, err = client3.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("✅ Timeout or other error caught: %v\n", err)
	} else {
		fmt.Printf("⚡ Server responded quickly!\n")
	}
}

// pollForCompletion polls a task until it completes
func pollForCompletion(ctx context.Context, client *a2a.Client, taskID string) {
	fmt.Println("⏳ Polling for task completion...")

	maxPolls := 10
	pollInterval := 1 * time.Second

	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollInterval)

		queryParams := &a2a.TaskQueryParams{
			ID: taskID,
		}

		task, err := client.GetTask(ctx, queryParams)
		if err != nil {
			fmt.Printf("❌ Polling error: %v\n", err)
			return
		}

		fmt.Printf("📊 Poll %d/%d - Status: %s\n", i+1, maxPolls, task.Status.State)

		// Check if task is in final state
		switch task.Status.State {
		case a2a.TaskStateCompleted, a2a.TaskStateFailed, a2a.TaskStateCanceled:
			fmt.Printf("🏁 Task finished with status: %s\n", task.Status.State)

			if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
				fmt.Println("📝 Final message:")
				for _, part := range task.Status.Message.Parts {
					if part.Text != nil {
						fmt.Printf("  %s\n", *part.Text)
					}
				}
			}
			return
		}
	}

	fmt.Printf("⏰ Polling timeout after %d attempts\n", maxPolls)
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("client_task_%d", time.Now().UnixNano())
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
