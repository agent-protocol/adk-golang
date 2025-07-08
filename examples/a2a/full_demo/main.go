// Package main demonstrates a complete A2A communication workflow with both client and server components.
// This example shows how to set up an A2A server, register agents, and communicate with them using an A2A client.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/a2a/server"
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

func main() {
	fmt.Println("🌟 Complete A2A Demo: Client & Server")
	fmt.Println("=====================================")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		startServer(ctx)
	}()

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Run client demonstrations
	runClientDemos(ctx)

	// Graceful shutdown
	fmt.Println("\n🛑 Shutting down...")
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(1 * time.Second)

	fmt.Println("✅ Demo completed!")
}

// startServer creates and runs the A2A server
func startServer(ctx context.Context) {
	fmt.Println("🚀 Starting A2A Server on :8080...")

	// Create tools
	calculatorTool := createCalculatorTool()
	weatherTool := createWeatherTool()
	echoTool := createEchoTool()

	// Create agents
	agentMap := make(map[string]core.BaseAgent)

	// Calculator Agent
	calcAgent := agents.NewLLMAgent("calculator", "Mathematical calculation specialist", "gemini-2.0-flash")
	calcAgent.SetInstruction("You are a mathematical specialist. Provide accurate calculations and explain your work.")
	calcAgent.AddTool(calculatorTool)
	agentMap["calculator"] = calcAgent

	// Weather Agent
	weatherAgent := agents.NewLLMAgent("weather", "Weather information specialist", "gemini-2.0-flash")
	weatherAgent.SetInstruction("You are a weather specialist. Provide detailed weather information.")
	weatherAgent.AddTool(weatherTool)
	agentMap["weather"] = weatherAgent

	// Echo Agent
	echoAgent := agents.NewBaseAgent("echo", "Simple echo agent")
	echoAgent.SetInstruction("You echo back what users say with some helpful commentary.")
	agentMap["echo"] = echoAgent

	// Multi-tool Agent
	multiAgent := agents.NewLLMAgent("multi", "Multi-purpose assistant", "gemini-2.0-flash")
	multiAgent.SetInstruction("You are a versatile assistant with multiple capabilities.")
	multiAgent.AddTool(calculatorTool)
	multiAgent.AddTool(weatherTool)
	multiAgent.AddTool(echoTool)
	agentMap["multi"] = multiAgent

	// Create agent cards
	agentCards := createAgentCards()

	// Create A2A server
	a2aServer := server.NewA2AServer(server.A2AServerConfig{
		Agents:     agentMap,
		AgentCards: agentCards,
	})

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.Handle("/a2a", a2aServer)

	mux.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		if card, exists := agentCards["multi"]; exists {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(card)
		} else {
			http.Error(w, "Agent not found", http.StatusNotFound)
		}
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Graceful shutdown handling
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	fmt.Println("✅ A2A Server is ready on http://localhost:8080")

	// Start server
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Server error: %v", err)
	}
}

// runClientDemos demonstrates various client usage patterns
func runClientDemos(ctx context.Context) {
	fmt.Println("\n🤖 Starting Client Demonstrations")
	fmt.Println("=================================")

	// Discover the agent
	agentCard := discoverAgent(ctx)
	if agentCard == nil {
		fmt.Println("❌ Cannot proceed without agent discovery")
		return
	}

	// Demo scenarios
	demos := []struct {
		name string
		fn   func(context.Context, *a2a.AgentCard)
	}{
		{"Basic Calculator", demoCalculation},
		{"Weather Query", demoWeather},
		{"Echo Test", demoEcho},
		{"Multi-step Task", demoMultiStep},
		{"Streaming Response", demoStreaming},
		{"Task Lifecycle", demoTaskLifecycle},
		{"Concurrent Requests", demoConcurrentRequests},
		{"Error Scenarios", demoErrorHandling},
	}

	for i, demo := range demos {
		fmt.Printf("\n📋 Demo %d/%d: %s\n", i+1, len(demos), demo.name)
		fmt.Println(strings.Repeat("-", 40))

		demoCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		demo.fn(demoCtx, agentCard)
		cancel()

		if i < len(demos)-1 {
			time.Sleep(1 * time.Second)
		}
	}
}

// discoverAgent discovers the agent using the well-known endpoint
func discoverAgent(ctx context.Context) *a2a.AgentCard {
	resolver := a2a.NewAgentCardResolver("http://localhost:8080", nil)

	agentCard, err := resolver.GetWellKnownAgentCard(ctx)
	if err != nil {
		fmt.Printf("❌ Agent discovery failed: %v\n", err)
		return nil
	}

	fmt.Printf("✅ Discovered agent: %s v%s\n", agentCard.Name, agentCard.Version)
	return agentCard
}

// demoCalculation demonstrates mathematical calculations
func demoCalculation(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("Please calculate 15 * 23 + 47"),
		}},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
		Metadata: map[string]any{
			"agent_name": "calculator",
			"demo_type":  "calculation",
		},
	}

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Message send failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Calculation request sent (Task: %s)\n", task.ID)
	waitForCompletion(ctx, client, task.ID)
}

// demoWeather demonstrates weather information queries
func demoWeather(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("What's the weather like in Tokyo?"),
		}},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
		Metadata: map[string]any{
			"agent_name": "weather",
			"demo_type":  "weather",
		},
	}

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Message send failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Weather request sent (Task: %s)\n", task.ID)
	waitForCompletion(ctx, client, task.ID)
}

// demoEcho demonstrates simple echo functionality
func demoEcho(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("Hello, this is a test message for echo functionality!"),
		}},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
		Metadata: map[string]any{
			"agent_name": "echo",
			"demo_type":  "echo",
		},
	}

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Message send failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Echo request sent (Task: %s)\n", task.ID)
	waitForCompletion(ctx, client, task.ID)
}

// demoMultiStep demonstrates a complex multi-step task
func demoMultiStep(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("Please: 1) Calculate 25 * 4, 2) Get weather for London, 3) Echo back 'Task completed successfully'"),
		}},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
		Metadata: map[string]any{
			"agent_name": "multi",
			"demo_type":  "multi_step",
		},
	}

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Message send failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Multi-step request sent (Task: %s)\n", task.ID)
	waitForCompletion(ctx, client, task.ID)
}

// demoStreaming demonstrates streaming communication
func demoStreaming(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("Stream me the results of: fibonacci of 10, weather in Paris, and echo 'streaming complete'"),
		}},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
		Metadata: map[string]any{
			"agent_name": "multi",
			"demo_type":  "streaming",
		},
	}

	fmt.Printf("🌊 Starting streaming request...\n")

	eventCount := 0
	err = client.SendMessageStream(ctx, params, func(response *a2a.SendTaskStreamingResponse) error {
		eventCount++

		if response.Error != nil {
			fmt.Printf("❌ Stream error: %s\n", response.Error.Message)
			return nil
		}

		fmt.Printf("📨 Event %d received\n", eventCount)

		// Handle different event types
		if response.Result != nil {
			resultBytes, _ := json.Marshal(response.Result)

			var statusUpdate a2a.TaskStatusUpdateEvent
			if err := json.Unmarshal(resultBytes, &statusUpdate); err == nil && statusUpdate.ID != "" {
				fmt.Printf("  📊 Status: %s\n", statusUpdate.Status.State)
				if statusUpdate.Final {
					fmt.Printf("  🏁 Task completed\n")
				}
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("❌ Streaming failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Streaming completed with %d events\n", eventCount)
}

// demoTaskLifecycle demonstrates complete task lifecycle management
func demoTaskLifecycle(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	// Start a task
	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("This is a task for lifecycle demonstration"),
		}},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
		Metadata: map[string]any{
			"agent_name": "echo",
			"demo_type":  "lifecycle",
		},
	}

	fmt.Printf("📋 Creating task: %s\n", params.ID)

	task, err := client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("❌ Task creation failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Task created with status: %s\n", task.Status.State)

	// Query task status
	time.Sleep(500 * time.Millisecond)
	queryParams := &a2a.TaskQueryParams{ID: task.ID}

	queriedTask, err := client.GetTask(ctx, queryParams)
	if err != nil {
		fmt.Printf("❌ Task query failed: %v\n", err)
	} else {
		fmt.Printf("🔍 Queried task status: %s\n", queriedTask.Status.State)
	}

	// Attempt cancellation (might not work if task already completed)
	time.Sleep(500 * time.Millisecond)
	cancelParams := &a2a.TaskIdParams{ID: task.ID}

	canceledTask, err := client.CancelTask(ctx, cancelParams)
	if err != nil {
		fmt.Printf("⚠️  Task cancellation failed (might be completed): %v\n", err)
	} else {
		fmt.Printf("🛑 Task canceled with status: %s\n", canceledTask.Status.State)
	}
}

// demoConcurrentRequests demonstrates handling multiple concurrent requests
func demoConcurrentRequests(ctx context.Context, agentCard *a2a.AgentCard) {
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("🔄 Starting 3 concurrent requests...\n")

	var wg sync.WaitGroup
	results := make(chan string, 3)

	// Request 1: Calculator
	wg.Add(1)
	go func() {
		defer wg.Done()

		message := &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{{
				Type: "text",
				Text: stringPtr("Calculate 100 / 4"),
			}},
		}

		params := &a2a.TaskSendParams{
			ID:       generateTaskID(),
			Message:  *message,
			Metadata: map[string]any{"agent_name": "calculator"},
		}

		task, err := client.SendMessage(ctx, params)
		if err != nil {
			results <- fmt.Sprintf("❌ Calc task failed: %v", err)
		} else {
			results <- fmt.Sprintf("✅ Calc task: %s", task.ID)
		}
	}()

	// Request 2: Weather
	wg.Add(1)
	go func() {
		defer wg.Done()

		message := &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{{
				Type: "text",
				Text: stringPtr("Weather in New York"),
			}},
		}

		params := &a2a.TaskSendParams{
			ID:       generateTaskID(),
			Message:  *message,
			Metadata: map[string]any{"agent_name": "weather"},
		}

		task, err := client.SendMessage(ctx, params)
		if err != nil {
			results <- fmt.Sprintf("❌ Weather task failed: %v", err)
		} else {
			results <- fmt.Sprintf("✅ Weather task: %s", task.ID)
		}
	}()

	// Request 3: Echo
	wg.Add(1)
	go func() {
		defer wg.Done()

		message := &a2a.Message{
			Role: "user",
			Parts: []a2a.Part{{
				Type: "text",
				Text: stringPtr("Concurrent request test"),
			}},
		}

		params := &a2a.TaskSendParams{
			ID:       generateTaskID(),
			Message:  *message,
			Metadata: map[string]any{"agent_name": "echo"},
		}

		task, err := client.SendMessage(ctx, params)
		if err != nil {
			results <- fmt.Sprintf("❌ Echo task failed: %v", err)
		} else {
			results <- fmt.Sprintf("✅ Echo task: %s", task.ID)
		}
	}()

	// Wait for all requests to complete
	wg.Wait()
	close(results)

	// Collect results
	for result := range results {
		fmt.Printf("  %s\n", result)
	}
}

// demoErrorHandling demonstrates various error scenarios
func demoErrorHandling(ctx context.Context, agentCard *a2a.AgentCard) {
	fmt.Printf("🚨 Testing error handling scenarios...\n")

	// Test 1: Invalid task ID
	client, err := a2a.NewClient(agentCard, nil)
	if err != nil {
		fmt.Printf("❌ Client creation failed: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("1. Invalid task query: ")
	queryParams := &a2a.TaskQueryParams{ID: "invalid-task-id"}
	_, err = client.GetTask(ctx, queryParams)
	if err != nil {
		fmt.Printf("✅ Error caught: %v\n", err)
	} else {
		fmt.Printf("⚠️  Expected error but got success\n")
	}

	// Test 2: Malformed message
	fmt.Printf("2. Invalid message format: ")
	invalidMessage := &a2a.Message{
		Role:  "", // Invalid empty role
		Parts: []a2a.Part{},
	}

	params := &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *invalidMessage,
	}

	_, err = client.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("✅ Error caught: %v\n", err)
	} else {
		fmt.Printf("⚠️  Expected error but got success\n")
	}

	// Test 3: Network timeout
	fmt.Printf("3. Timeout scenario: ")

	shortTimeoutConfig := &a2a.ClientConfig{
		Timeout: 50 * time.Millisecond,
		BaseURL: agentCard.URL,
	}

	timeoutClient, err := a2a.NewClient(agentCard, shortTimeoutConfig)
	if err != nil {
		fmt.Printf("❌ Timeout client creation failed: %v\n", err)
		return
	}
	defer timeoutClient.Close()

	message := &a2a.Message{
		Role: "user",
		Parts: []a2a.Part{{
			Type: "text",
			Text: stringPtr("Test timeout"),
		}},
	}

	params = &a2a.TaskSendParams{
		ID:      generateTaskID(),
		Message: *message,
	}

	_, err = timeoutClient.SendMessage(ctx, params)
	if err != nil {
		fmt.Printf("✅ Timeout caught: %v\n", err)
	} else {
		fmt.Printf("⚡ Server responded quickly!\n")
	}
}

// waitForCompletion waits for a task to complete and prints the result
func waitForCompletion(ctx context.Context, client *a2a.Client, taskID string) {
	maxWait := 10 * time.Second
	checkInterval := 500 * time.Millisecond

	timeoutCtx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			fmt.Printf("⏰ Task %s timed out\n", taskID)
			return
		case <-ticker.C:
			queryParams := &a2a.TaskQueryParams{ID: taskID}
			task, err := client.GetTask(ctx, queryParams)
			if err != nil {
				fmt.Printf("❌ Query error: %v\n", err)
				return
			}

			switch task.Status.State {
			case a2a.TaskStateCompleted:
				fmt.Printf("✅ Task completed successfully\n")
				if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
					for _, part := range task.Status.Message.Parts {
						if part.Text != nil {
							fmt.Printf("📝 Result: %s\n", *part.Text)
						}
					}
				}
				return
			case a2a.TaskStateFailed:
				fmt.Printf("❌ Task failed\n")
				if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
					for _, part := range task.Status.Message.Parts {
						if part.Text != nil {
							fmt.Printf("💥 Error: %s\n", *part.Text)
						}
					}
				}
				return
			case a2a.TaskStateCanceled:
				fmt.Printf("🛑 Task was canceled\n")
				return
			default:
				// Task still in progress, continue polling
			}
		}
	}
}

// Tool creation functions

func createCalculatorTool() core.BaseTool {
	tool, err := tools.NewFunctionTool(
		"calculator",
		"Performs mathematical calculations",
		func(operation string, a, b float64) (float64, error) {
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

func createWeatherTool() core.BaseTool {
	tool, err := tools.NewFunctionTool(
		"weather",
		"Gets weather information for a location",
		func(location string) map[string]interface{} {
			// Mock weather data
			weatherData := map[string]map[string]interface{}{
				"tokyo":    {"temperature": 22, "condition": "sunny"},
				"london":   {"temperature": 15, "condition": "rainy"},
				"new york": {"temperature": 18, "condition": "cloudy"},
				"paris":    {"temperature": 20, "condition": "partly cloudy"},
			}

			if data, exists := weatherData[strings.ToLower(location)]; exists {
				return data
			}

			return map[string]interface{}{
				"temperature": 20,
				"condition":   "unknown",
				"note":        fmt.Sprintf("No data available for %s", location),
			}
		},
	)
	if err != nil {
		log.Fatalf("Failed to create weather tool: %v", err)
	}
	return tool
}

func createEchoTool() core.BaseTool {
	tool, err := tools.NewFunctionTool(
		"echo",
		"Echoes back the input message",
		func(message string) string {
			return fmt.Sprintf("Echo: %s", message)
		},
	)
	if err != nil {
		log.Fatalf("Failed to create echo tool: %v", err)
	}
	return tool
}

// createAgentCards creates agent metadata cards
func createAgentCards() map[string]*a2a.AgentCard {
	cards := make(map[string]*a2a.AgentCard)

	cards["multi"] = &a2a.AgentCard{
		Name:        "multi",
		Description: stringPtr("Multi-purpose agent with calculator, weather, and echo capabilities"),
		URL:         "http://localhost:8080/a2a",
		Version:     "1.0.0",
		Provider: &a2a.AgentProvider{
			Organization: "A2A Demo",
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
				Name:        "Calculator",
				Description: stringPtr("Mathematical calculations"),
				Examples:    []string{"Calculate 2+2", "What is 15*7?"},
			},
			{
				ID:          "weather",
				Name:        "Weather",
				Description: stringPtr("Weather information"),
				Examples:    []string{"Weather in Tokyo", "Temperature in Paris"},
			},
			{
				ID:          "echo",
				Name:        "Echo",
				Description: stringPtr("Echo messages back"),
				Examples:    []string{"Echo hello", "Repeat this message"},
			},
		},
	}

	// Add individual agent cards for other agents
	cards["calculator"] = &a2a.AgentCard{
		Name:               "calculator",
		Description:        stringPtr("Mathematical calculation specialist"),
		URL:                "http://localhost:8080/a2a",
		Version:            "1.0.0",
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{{
			ID:          "calculation",
			Name:        "Calculator",
			Description: stringPtr("Mathematical calculations"),
		}},
	}

	cards["weather"] = &a2a.AgentCard{
		Name:               "weather",
		Description:        stringPtr("Weather information specialist"),
		URL:                "http://localhost:8080/a2a",
		Version:            "1.0.0",
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{{
			ID:          "weather",
			Name:        "Weather",
			Description: stringPtr("Weather information"),
		}},
	}

	cards["echo"] = &a2a.AgentCard{
		Name:               "echo",
		Description:        stringPtr("Simple echo agent"),
		URL:                "http://localhost:8080/a2a",
		Version:            "1.0.0",
		Capabilities:       a2a.AgentCapabilities{Streaming: false},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Skills: []a2a.AgentSkill{{
			ID:          "echo",
			Name:        "Echo",
			Description: stringPtr("Echo messages"),
		}},
	}

	return cards
}

// Utility functions

func generateTaskID() string {
	return fmt.Sprintf("demo_task_%d", time.Now().UnixNano())
}

func stringPtr(s string) *string {
	return &s
}
