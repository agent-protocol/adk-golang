package main

import (
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

func main() {
	fmt.Println("=== Enhanced Session Management Demo ===")
	fmt.Println()

	// Create a new session
	session := core.NewSession("demo-session", "chat-app", "user123")
	fmt.Printf("Created session: %s for user: %s in app: %s\n", session.ID, session.UserID, session.AppName)

	// Create a state helper for convenient operations
	helper := core.NewSessionStateHelper(session)

	// Demonstrate basic state operations
	fmt.Println("\n--- Basic State Operations ---")
	helper.SetString("username", "Alice")
	helper.SetInt("score", 100)
	helper.SetBool("premium", true)
	helper.SetFloat("rating", 4.5)
	helper.SetTime("last_login", time.Now())

	username, _ := helper.GetString("username")
	score, _ := helper.GetInt("score")
	premium, _ := helper.GetBool("premium")
	rating, _ := helper.GetFloat("rating")
	lastLogin, _ := helper.GetTime("last_login")

	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Score: %d\n", score)
	fmt.Printf("Premium: %t\n", premium)
	fmt.Printf("Rating: %.1f\n", rating)
	fmt.Printf("Last Login: %s\n", lastLogin.Format("15:04:05"))

	// Demonstrate numeric operations
	fmt.Println("\n--- Numeric Operations ---")
	newScore, _ := helper.Increment("score", 25)
	fmt.Printf("Score after increment: %d\n", newScore)

	helper.Toggle("premium")
	premium, _ = helper.GetBool("premium")
	fmt.Printf("Premium after toggle: %t\n", premium)

	// Demonstrate slice operations
	fmt.Println("\n--- Slice Operations ---")
	helper.SetSlice("tags", []any{"beginner", "active"})
	helper.AppendToSlice("tags", "premium")
	helper.PrependToSlice("tags", "verified")

	tags, _ := helper.GetSlice("tags")
	fmt.Printf("Tags: %v\n", tags)

	item, _ := helper.PopFromSlice("tags")
	fmt.Printf("Popped item: %v\n", item)

	tags, _ = helper.GetSlice("tags")
	fmt.Printf("Tags after pop: %v\n", tags)

	// Demonstrate map operations
	fmt.Println("\n--- Map Operations ---")
	helper.SetMap("preferences", map[string]any{
		"theme":    "dark",
		"language": "en",
	})
	helper.SetMapKey("preferences", "notifications", true)

	preferences, _ := helper.GetMap("preferences")
	fmt.Printf("Preferences: %v\n", preferences)

	theme, _ := helper.GetMapKey("preferences", "theme")
	fmt.Printf("Theme preference: %v\n", theme)

	// Demonstrate JSON operations
	fmt.Println("\n--- JSON Operations ---")
	type UserProfile struct {
		Name     string          `json:"name"`
		Age      int             `json:"age"`
		Hobbies  []string        `json:"hobbies"`
		Settings map[string]bool `json:"settings"`
	}

	profile := UserProfile{
		Name:     "Alice Johnson",
		Age:      28,
		Hobbies:  []string{"reading", "coding", "gaming"},
		Settings: map[string]bool{"notifications": true, "darkMode": true},
	}

	err := helper.SetJSON("profile", profile)
	if err != nil {
		log.Printf("Error setting JSON: %v", err)
	}

	var retrievedProfile UserProfile
	err = helper.GetJSON("profile", &retrievedProfile)
	if err != nil {
		log.Printf("Error getting JSON: %v", err)
	} else {
		fmt.Printf("Retrieved profile: %+v\n", retrievedProfile)
	}

	// Add some events to the session
	fmt.Println("\n--- Event Management ---")
	event1 := &core.Event{
		ID:        "evt-1",
		Author:    "user",
		Timestamp: time.Now(),
		Content: &core.Content{
			Parts: []core.Part{
				{Text: stringPtr("Hello, how are you?")},
			},
		},
	}
	session.AddEvent(event1)

	event2 := &core.Event{
		ID:        "evt-2",
		Author:    "assistant",
		Timestamp: time.Now().Add(time.Second),
		Content: &core.Content{
			Parts: []core.Part{
				{Text: stringPtr("I'm doing well, thank you! How can I help you today?")},
			},
		},
	}
	session.AddEvent(event2)

	event3 := &core.Event{
		ID:           "evt-3",
		Author:       "system",
		Timestamp:    time.Now().Add(2 * time.Second),
		ErrorMessage: stringPtr("Connection timeout"),
	}
	session.AddEvent(event3)

	fmt.Printf("Added %d events to session\n", len(session.Events))

	// Get session metrics
	fmt.Println("\n--- Session Metrics ---")
	metrics := session.GetMetrics()
	fmt.Printf("Event count: %d\n", metrics.EventCount)
	fmt.Printf("State size: %d keys\n", metrics.StateSize)
	fmt.Printf("Error count: %d\n", metrics.ErrorCount)
	fmt.Printf("Function call count: %d\n", metrics.FunctionCallCount)
	fmt.Printf("Author event counts: %v\n", metrics.AuthorEventCounts)
	fmt.Printf("Events by type: %v\n", metrics.EventsByType)

	// Create a snapshot
	fmt.Println("\n--- Session Snapshots ---")
	snapshot := session.CreateSnapshot()
	fmt.Printf("Created snapshot at %s\n", snapshot.Timestamp.Format("15:04:05"))
	fmt.Printf("Snapshot contains %d state keys and %d events\n",
		len(snapshot.State), snapshot.EventCount)

	// Modify session state
	helper.SetString("username", "Bob")
	helper.SetInt("score", 200)
	session.DeleteState("rating")

	fmt.Printf("Modified session: username=%s, score=%d\n",
		helper.GetStringWithDefault("username", ""),
		helper.GetIntWithDefault("score", 0))

	// Restore from snapshot
	err = session.RestoreFromSnapshot(snapshot)
	if err != nil {
		log.Printf("Error restoring snapshot: %v", err)
	} else {
		fmt.Printf("Restored session: username=%s, score=%d\n",
			helper.GetStringWithDefault("username", ""),
			helper.GetIntWithDefault("score", 0))
	}

	// Demonstrate state diffing
	fmt.Println("\n--- State Diffing ---")
	otherState := map[string]any{
		"username": "Alice",
		"score":    150,
		"newfield": "value",
	}

	diff := session.DiffState(otherState)
	fmt.Printf("Added: %v\n", diff.Added)
	fmt.Printf("Modified: %v\n", diff.Modified)
	fmt.Printf("Removed: %v\n", diff.Removed)

	// Demonstrate session validation
	fmt.Println("\n--- Session Validation ---")
	validationError := session.Validate()
	if validationError == nil {
		fmt.Println("Session is valid")
	} else {
		fmt.Printf("Session validation error: %v\n", validationError)
	}

	// Clone session
	fmt.Println("\n--- Session Cloning ---")
	cloned := session.Clone()
	cloned.ID = "cloned-session"
	fmt.Printf("Original session ID: %s\n", session.ID)
	fmt.Printf("Cloned session ID: %s\n", cloned.ID)
	fmt.Printf("Cloned session has %d events and %d state keys\n",
		len(cloned.Events), len(cloned.State))

	// Demonstrate scoped state operations
	fmt.Println("\n--- Scoped State Operations ---")
	session.SetState("app:global_config", "production")
	session.SetState("user:preferences", map[string]any{"theme": "dark"})
	session.SetState("temp:processing", true)

	appConfig, exists := session.GetState("app:global_config")
	fmt.Printf("App config (exists=%t): %v\n", exists, appConfig)

	userPrefs, exists := session.GetState("user:preferences")
	fmt.Printf("User preferences (exists=%t): %v\n", exists, userPrefs)

	tempProcessing, exists := session.GetState("temp:processing")
	fmt.Printf("Temp processing (exists=%t): %v\n", exists, tempProcessing)

	// Show final session summary
	fmt.Println("\n--- Final Session Summary ---")
	fmt.Printf("Session ID: %s\n", session.ID)
	fmt.Printf("User ID: %s\n", session.UserID)
	fmt.Printf("App Name: %s\n", session.AppName)
	fmt.Printf("Total Events: %d\n", len(session.Events))
	fmt.Printf("Total State Keys: %d\n", len(session.State))
	fmt.Printf("Last Update: %s\n", session.LastUpdateTime.Format("15:04:05"))

	// List all state keys
	fmt.Println("\nAll state keys:")
	for key, value := range session.State {
		fmt.Printf("  %s: %v\n", key, value)
	}

	fmt.Println("\n=== Demo Complete ===")
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
