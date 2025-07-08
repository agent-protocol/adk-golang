package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

func main() {
	// Create a sample event with all requested fields
	event := &core.Event{
		ID:           "evt_demo_123",
		InvocationID: "inv_456",
		Author:       "demo_agent",
		Branch:       stringPtr("main.sub_agent"),
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("This is a demo message"),
				},
				{
					Type: "function_call",
					FunctionCall: &core.FunctionCall{
						ID:   "call_123",
						Name: "demo_tool",
						Args: map[string]any{"param": "value"},
					},
				},
			},
		},
		Actions: core.EventActions{
			StateDelta: map[string]any{
				"user:preference": "value1",
				"temp:session":    "value2",
			},
			TransferToAgent: stringPtr("specialized_agent"),
		},
		Timestamp: time.Now(),
		CustomMetadata: map[string]any{
			"source":   "demo",
			"priority": 1,
			"tags":     []string{"demo", "test"},
		},
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Event JSON representation:")
	fmt.Println(string(jsonData))

	// Unmarshal back from JSON
	var newEvent core.Event
	err = json.Unmarshal(jsonData, &newEvent)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the data
	fmt.Printf("\nVerification:\n")
	fmt.Printf("ID: %s\n", newEvent.ID)
	fmt.Printf("InvocationID: %s\n", newEvent.InvocationID)
	fmt.Printf("Author: %s\n", newEvent.Author)
	fmt.Printf("Branch: %s\n", *newEvent.Branch)
	fmt.Printf("Content Parts: %d\n", len(newEvent.Content.Parts))
	fmt.Printf("StateDelta entries: %d\n", len(newEvent.Actions.StateDelta))
	fmt.Printf("CustomMetadata entries: %d\n", len(newEvent.CustomMetadata))

	fmt.Println("\n✅ Event struct with JSON marshaling working perfectly!")
}

func stringPtr(s string) *string {
	return &s
}
