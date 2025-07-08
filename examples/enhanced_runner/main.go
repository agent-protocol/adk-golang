// Package main demonstrates the Runner's async event streaming capabilities.
// This example shows how to create a runner, execute agents, and stream events in real-time.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/runners"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

// StreamingAgent demonstrates an agent that produces multiple events over time.
type StreamingAgent struct {
	name        string
	description string
}

func (s *StreamingAgent) Name() string                         { return s.name }
func (s *StreamingAgent) Description() string                  { return s.description }
func (s *StreamingAgent) Instruction() string                  { return "Stream processing agent" }
func (s *StreamingAgent) SubAgents() []core.BaseAgent          { return nil }
func (s *StreamingAgent) ParentAgent() core.BaseAgent          { return nil }
func (s *StreamingAgent) SetParentAgent(parent core.BaseAgent) {}
func (s *StreamingAgent) FindAgent(name string) core.BaseAgent {
	if s.name == name {
		return s
	}
	return nil
}
func (s *StreamingAgent) FindSubAgent(name string) core.BaseAgent                  { return nil }
func (s *StreamingAgent) GetBeforeAgentCallback() core.BeforeAgentCallback         { return nil }
func (s *StreamingAgent) SetBeforeAgentCallback(callback core.BeforeAgentCallback) {}
func (s *StreamingAgent) GetAfterAgentCallback() core.AfterAgentCallback           { return nil }
func (s *StreamingAgent) SetAfterAgentCallback(callback core.AfterAgentCallback)   {}
func (s *StreamingAgent) Cleanup(ctx context.Context) error                        { return nil }

// RunAsync demonstrates real-time event streaming.
func (s *StreamingAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		steps := []struct {
			message string
			delay   time.Duration
			partial bool
			actions core.EventActions
		}{
			{"🔄 Starting processing...", 500 * time.Millisecond, true, core.EventActions{}},
			{"📊 Analyzing input data...", 800 * time.Millisecond, true, core.EventActions{}},
			{"🔍 Performing search operations...", 1 * time.Second, true, core.EventActions{}},
			{"⚙️ Processing results...", 600 * time.Millisecond, true, core.EventActions{}},
			{"💾 Saving state...", 400 * time.Millisecond, false, core.EventActions{
				StateDelta: map[string]any{
					"processing_complete": true,
					"result_count":        42,
					"last_processed":      time.Now().Format(time.RFC3339),
				},
			}},
			{"✅ Processing complete! Found 42 results.", 0, false, core.EventActions{}},
		}

		for i, step := range steps {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Wait for the specified delay
			if step.delay > 0 {
				time.Sleep(step.delay)
			}

			// Create event
			event := core.NewEvent(invocationCtx.InvocationID, s.name)
			event.Content = &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{
						Type: "text",
						Text: &step.message,
					},
				},
			}
			event.Actions = step.actions

			if step.partial {
				event.Partial = &step.partial
			}

			// Mark last event as turn complete
			if i == len(steps)-1 {
				turnComplete := true
				event.TurnComplete = &turnComplete
			}

			// Send event
			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

// Run is a synchronous wrapper (required by interface).
func (s *StreamingAgent) Run(ctx context.Context, invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	stream, err := s.RunAsync(ctx, invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range stream {
		events = append(events, event)
	}

	return events, nil
}

func main() {
	fmt.Println("🚀 ADK Go Runner - Async Event Streaming Demo")
	fmt.Println(strings.Repeat("=", 50))

	// Create streaming agent
	agent := &StreamingAgent{
		name:        "streaming-processor",
		description: "Demonstrates real-time event streaming",
	}

	// Create session service
	sessionService := sessions.NewInMemorySessionService()

	// Create runner with custom configuration
	config := &runners.RunnerConfig{
		EventBufferSize:       50,   // Larger buffer for smooth streaming
		EnableEventProcessing: true, // Enable state processing
		DefaultTimeout:        30 * time.Second,
	}

	runner := runners.NewRunnerWithConfig("streaming-demo", agent, sessionService, config)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Prepare run request
	req := &core.RunRequest{
		UserID:    "demo-user",
		SessionID: "streaming-session",
		NewMessage: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Please process this data with real-time updates"),
				},
			},
		},
	}

	fmt.Printf("📤 User: %s\n", *req.NewMessage.Parts[0].Text)
	fmt.Println()

	// Start async execution
	eventStream, err := runner.RunAsync(ctx, req)
	if err != nil {
		log.Fatalf("❌ Failed to start runner: %v", err)
	}

	fmt.Println("📡 Streaming events in real-time:")
	fmt.Println(strings.Repeat("-", 40))

	// Process events as they arrive
	eventCount := 0
	startTime := time.Now()

	for event := range eventStream {
		eventCount++
		elapsed := time.Since(startTime)

		// Extract message content
		var message string
		if event.Content != nil && len(event.Content.Parts) > 0 && event.Content.Parts[0].Text != nil {
			message = *event.Content.Parts[0].Text
		}

		// Format output based on event properties
		eventType := "📝"
		if event.Partial != nil && *event.Partial {
			eventType = "⏳"
		} else if event.TurnComplete != nil && *event.TurnComplete {
			eventType = "🏁"
		} else if len(event.Actions.StateDelta) > 0 {
			eventType = "💾"
		}

		fmt.Printf("[%6.2fs] %s %s\n", elapsed.Seconds(), eventType, message)

		// Show state changes
		if len(event.Actions.StateDelta) > 0 {
			fmt.Printf("         📊 State updated: %v\n", event.Actions.StateDelta)
		}

		// Show error if present
		if event.ErrorMessage != nil {
			fmt.Printf("         ❌ Error: %s\n", *event.ErrorMessage)
		}
	}

	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("✅ Streaming complete! Received %d events in %.2fs\n",
		eventCount, time.Since(startTime).Seconds())

	// Demonstrate synchronous execution for comparison
	fmt.Println()
	fmt.Println("🔄 Comparing with synchronous execution:")
	fmt.Println(strings.Repeat("-", 40))

	syncStartTime := time.Now()

	// Use a new session for sync demo
	syncReq := &core.RunRequest{
		UserID:    "demo-user",
		SessionID: "sync-session",
		NewMessage: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Same request, but synchronous"),
				},
			},
		},
	}

	events, err := runner.Run(ctx, syncReq)
	if err != nil {
		log.Fatalf("❌ Synchronous execution failed: %v", err)
	}

	syncElapsed := time.Since(syncStartTime)

	fmt.Printf("⚡ Synchronous execution completed in %.2fs\n", syncElapsed.Seconds())
	fmt.Printf("📊 Received %d events (all at once)\n", len(events))

	// Show final session state
	fmt.Println()
	fmt.Println("📋 Final Session State:")
	fmt.Println(strings.Repeat("-", 40))

	getReq := &core.GetSessionRequest{
		AppName:   "streaming-demo",
		UserID:    "demo-user",
		SessionID: "streaming-session",
	}

	session, err := sessionService.GetSession(ctx, getReq)
	if err != nil {
		log.Printf("⚠️ Could not retrieve session: %v", err)
	} else {
		fmt.Printf("🆔 Session ID: %s\n", session.ID)
		fmt.Printf("👤 User ID: %s\n", session.UserID)
		fmt.Printf("📊 Total Events: %d\n", len(session.Events))
		fmt.Printf("🕒 Last Updated: %s\n", session.LastUpdateTime.Format(time.RFC3339))

		if len(session.State) > 0 {
			fmt.Println("💾 Session State:")
			for key, value := range session.State {
				fmt.Printf("   %s: %v\n", key, value)
			}
		}
	}

	fmt.Println()
	fmt.Println("🎉 Demo completed successfully!")
}

// stringPtr returns a pointer to a string literal.
func stringPtr(s string) *string {
	return &s
}
