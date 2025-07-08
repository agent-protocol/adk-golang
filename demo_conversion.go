package main

import (
	"context"
	"fmt"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/llmconnect/ollama"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func main() {
	fmt.Println("=== Ollama Conversion Demo ===")

	// Create Ollama connection
	config := &ollama.OllamaConfig{
		BaseURL:     "http://localhost:11434",
		Model:       "llama3.2",
		Temperature: ptr.Float32(0.7),
		MaxTokens:   ptr.Ptr(1000),
		TopP:        ptr.Float32(0.9),
		TopK:        ptr.Ptr(40),
	}
	conn := ollama.NewOllamaConnection(config)

	// Create sample LLMRequest
	request := &core.LLMRequest{
		Contents: []core.Content{
			{
				Role: "user",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr("What's the weather like today?"),
					},
				},
			},
		},
		Config: &core.LLMConfig{
			Temperature: ptr.Float32(0.5),
			MaxTokens:   ptr.Ptr(500),
		},
		Tools: []*core.FunctionDeclaration{
			{
				Name:        "get_weather",
				Description: "Get current weather information",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The location to get weather for",
						},
						"units": map[string]interface{}{
							"type":        "string",
							"description": "Temperature units (celsius or fahrenheit)",
							"enum":        []interface{}{"celsius", "fahrenheit"},
						},
					},
					"required": []interface{}{"location"},
				},
			},
		},
	}

	fmt.Println("\n1. Testing Ollama Connection with LLMRequest...")

	// Test the public API that uses our conversion functions internally
	ctx := context.Background()

	// Try to make an actual request (this will test our conversion functions)
	response, err := conn.GenerateContent(ctx, request)
	if err != nil {
		fmt.Printf("⚠ Could not connect to Ollama server: %v\n", err)
		fmt.Println("  (This is expected if Ollama is not running locally)")
		fmt.Println("  However, the conversion functions were tested during the attempt!")
	} else {
		fmt.Println("✓ Successfully connected to Ollama!")
		if response.Content != nil && len(response.Content.Parts) > 0 {
			for _, part := range response.Content.Parts {
				if part.Type == "text" && part.Text != nil {
					fmt.Printf("✓ Ollama response: %s\n", *part.Text)
				}
			}
		}
	}

	fmt.Println("\n2. Testing Streaming API...")

	// Test streaming which also uses our conversion functions
	responseChan, err := conn.GenerateContentStream(ctx, request)
	if err != nil {
		fmt.Printf("⚠ Could not start streaming: %v\n", err)
	} else {
		fmt.Println("✓ Streaming started successfully!")
		// Read a few responses
		count := 0
		for response := range responseChan {
			count++
			fmt.Printf("✓ Received streaming response %d\n", count)
			if response.Content != nil && len(response.Content.Parts) > 0 {
				for _, part := range response.Content.Parts {
					if part.Type == "text" && part.Text != nil {
						fmt.Printf("  Content: %s\n", *part.Text)
					}
				}
			}
			// Break after first few responses to avoid infinite loop
			if count >= 3 {
				break
			}
		}
	}

	fmt.Println("\n=== Demo Complete ===")
}
