// Package main demonstrates the session management system.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

func main() {
	ctx := context.Background()

	// Example 1: Basic session management with in-memory storage
	fmt.Println("=== Example 1: In-Memory Session Service ===")
	runInMemoryExample(ctx)

	// Example 2: File-based session persistence
	fmt.Println("\n=== Example 2: File-Based Session Service ===")
	runFileBasedExample(ctx)

	// Example 3: State management with scoped keys
	fmt.Println("\n=== Example 3: State Management ===")
	runStateManagementExample(ctx)

	// Example 4: Event handlers for session lifecycle
	fmt.Println("\n=== Example 4: Event Handlers ===")
	runEventHandlersExample(ctx)

	// Example 5: Session utilities and helpers
	fmt.Println("\n=== Example 5: Session Utilities ===")
	runUtilitiesExample(ctx)

	// Example 6: Session backup and restore
	fmt.Println("\n=== Example 6: Backup and Restore ===")
	runBackupRestoreExample(ctx)
}

func runInMemoryExample(ctx context.Context) {
	// Create an in-memory session service
	service := sessions.NewInMemorySessionService()
	defer service.Close(ctx)

	appName := "chat_app"
	userID := "user123"

	// Create a new session
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{"conversation_started": time.Now()},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	fmt.Printf("Created session: %s\n", session.ID)

	// Add some events to the session
	events := []*core.Event{
		{
			ID:           "evt_001",
			InvocationID: "inv_001",
			Author:       "user",
			Content: &core.Content{
				Role: "user",
				Parts: []core.Part{
					{Type: "text", Text: stringPtr("Hello, I need help with my order")},
				},
			},
			Actions:   core.EventActions{},
			Timestamp: time.Now(),
		},
		{
			ID:           "evt_002",
			InvocationID: "inv_001",
			Author:       "assistant",
			Content: &core.Content{
				Role: "assistant",
				Parts: []core.Part{
					{Type: "text", Text: stringPtr("I'd be happy to help you with your order. Can you provide your order number?")},
				},
			},
			Actions: core.EventActions{
				StateDelta: map[string]any{"help_topic": "order_inquiry"},
			},
			Timestamp: time.Now(),
		},
	}

	for _, event := range events {
		err = service.AppendEvent(ctx, session, event)
		if err != nil {
			log.Fatalf("Failed to append event: %v", err)
		}
	}

	// Retrieve and display the session
	getReq := &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: session.ID,
	}

	retrievedSession, err := service.GetSession(ctx, getReq)
	if err != nil {
		log.Fatalf("Failed to get session: %v", err)
	}

	fmt.Printf("Session has %d events\n", len(retrievedSession.Events))
	fmt.Printf("Session state: %v\n", retrievedSession.State)

	// List all sessions for the user
	listReq := &core.ListSessionsRequest{
		AppName: appName,
		UserID:  userID,
	}

	listResp, err := service.ListSessions(ctx, listReq)
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}

	fmt.Printf("User has %d sessions\n", len(listResp.Sessions))
}

func runFileBasedExample(ctx context.Context) {
	// Create a file-based session service with configuration
	config := &sessions.SessionConfiguration{
		MaxSessionsPerUser:  5,
		MaxEventsPerSession: 100,
		SessionTTL:          24 * time.Hour,
		AutoCleanupInterval: time.Hour,
		PersistenceBackend:  "file",
		BackendConfig: map[string]any{
			"base_directory": "./demo_sessions",
		},
		EnableEventHandlers: true,
		EnableMetrics:       true,
	}

	service, err := sessions.NewFileSessionService("./demo_sessions", config)
	if err != nil {
		log.Fatalf("Failed to create file session service: %v", err)
	}
	defer service.Close(ctx)

	appName := "file_demo"
	userID := "user456"

	// Create session with initial state
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State: map[string]any{
			"preferences": map[string]any{
				"language":      "en",
				"theme":         "dark",
				"notifications": true,
			},
			"session_type": "demo",
		},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		log.Fatalf("Failed to create file session: %v", err)
	}
	fmt.Printf("Created file-based session: %s\n", session.ID)

	// Update session state
	newState := map[string]any{
		"preferences": map[string]any{
			"language":      "en",
			"theme":         "light", // Changed theme
			"notifications": true,
		},
		"session_type":  "demo",
		"last_activity": time.Now(),
	}

	err = service.UpdateSessionState(ctx, appName, userID, session.ID, newState)
	if err != nil {
		log.Fatalf("Failed to update session state: %v", err)
	}
	fmt.Println("Updated session state")

	// Get session metadata
	metadata, err := service.GetSessionMetadata(ctx, appName, userID, session.ID)
	if err != nil {
		log.Fatalf("Failed to get session metadata: %v", err)
	}
	fmt.Printf("Session metadata: %d events, %d state keys\n", metadata.EventCount, len(metadata.StateKeys))
}

func runStateManagementExample(ctx context.Context) {
	// Create state manager and session service
	stateManager := sessions.NewDefaultStateManager()
	service := sessions.NewInMemorySessionService()
	defer service.Close(ctx)

	appName := "state_demo"
	userID := "user789"

	// Create session
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{"session_start": time.Now()},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	// Set different scoped state values
	err = stateManager.SetAppState(ctx, appName, "app_version", "1.0.0")
	if err != nil {
		log.Fatalf("Failed to set app state: %v", err)
	}

	err = stateManager.SetUserState(ctx, appName, userID, "user_preferences", map[string]any{
		"timezone": "UTC",
		"language": "en",
	})
	if err != nil {
		log.Fatalf("Failed to set user state: %v", err)
	}

	err = stateManager.SetState(ctx, session, "current_conversation", "greeting")
	if err != nil {
		log.Fatalf("Failed to set session state: %v", err)
	}

	// Demonstrate scoped key access
	appVersion, exists, err := stateManager.GetState(ctx, session, "app:app_version")
	if err != nil {
		log.Fatalf("Failed to get app state: %v", err)
	}
	if exists {
		fmt.Printf("App version: %v\n", appVersion)
	}

	userPrefs, exists, err := stateManager.GetState(ctx, session, "user:user_preferences")
	if err != nil {
		log.Fatalf("Failed to get user state: %v", err)
	}
	if exists {
		fmt.Printf("User preferences: %v\n", userPrefs)
	}

	// Get effective state (combined from all scopes)
	effectiveState, err := stateManager.GetEffectiveState(ctx, session)
	if err != nil {
		log.Fatalf("Failed to get effective state: %v", err)
	}
	fmt.Printf("Effective state has %d keys\n", len(effectiveState))

	// Use state helper for common operations
	helper := sessions.NewStateHelper(stateManager)

	// Increment a counter
	count, err := helper.Increment(ctx, session, "message_count", 1)
	if err != nil {
		log.Fatalf("Failed to increment counter: %v", err)
	}
	fmt.Printf("Message count: %d\n", count)

	// Toggle a flag
	isActive, err := helper.Toggle(ctx, session, "conversation_active")
	if err != nil {
		log.Fatalf("Failed to toggle flag: %v", err)
	}
	fmt.Printf("Conversation active: %v\n", isActive)

	// Add items to a list
	err = helper.Push(ctx, session, "message_history", "Hello")
	if err != nil {
		log.Fatalf("Failed to push to list: %v", err)
	}

	err = helper.Push(ctx, session, "message_history", "How can I help you?")
	if err != nil {
		log.Fatalf("Failed to push to list: %v", err)
	}

	// Get default value
	theme, err := helper.GetOrDefault(ctx, session, "theme", "default")
	if err != nil {
		log.Fatalf("Failed to get or default: %v", err)
	}
	fmt.Printf("Theme: %v\n", theme)
}

func runEventHandlersExample(ctx context.Context) {
	// Create session service
	service := sessions.NewInMemorySessionService()
	defer service.Close(ctx)

	// Create event handlers
	logger := &sessions.DefaultLogger{}
	loggingHandler := sessions.NewLoggingEventHandler(logger)

	metrics := &sessions.DefaultMetricsCollector{}
	metricsHandler := sessions.NewMetricsEventHandler(metrics)

	validationConfig := &sessions.ValidationConfig{
		MaxStateSize:   1024 * 1024, // 1MB
		MaxEventSize:   512 * 1024,  // 512KB
		AllowedAuthors: []string{"user", "assistant", "system"},
		ForbiddenKeys:  []string{"password", "secret", "api_key"},
	}
	validationHandler := sessions.NewValidationEventHandler(validationConfig)

	// Combine handlers
	compositeHandler := sessions.NewCompositeEventHandler(
		loggingHandler,
		metricsHandler,
		validationHandler,
	)

	// Add handler to file service (if it were a FileSessionService)
	// For demonstration, we'll manually trigger the handlers

	appName := "handler_demo"
	userID := "user999"

	// Create session and trigger handlers
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{"demo": "handlers"},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	// Manually trigger handlers for demonstration
	err = compositeHandler.OnSessionCreated(ctx, session)
	if err != nil {
		log.Fatalf("Handler error: %v", err)
	}

	// Add an event and trigger handlers
	event := &core.Event{
		ID:           "evt_handler_demo",
		InvocationID: "inv_handler_demo",
		Author:       "user",
		Content: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{Type: "text", Text: stringPtr("Test message for handlers")},
			},
		},
		Actions:   core.EventActions{},
		Timestamp: time.Now(),
	}

	err = service.AppendEvent(ctx, session, event)
	if err != nil {
		log.Fatalf("Failed to append event: %v", err)
	}

	err = compositeHandler.OnEventAdded(ctx, session, event)
	if err != nil {
		log.Fatalf("Handler error: %v", err)
	}

	fmt.Println("Event handlers executed successfully")
}

func runUtilitiesExample(ctx context.Context) {
	// Create session service and utilities
	service := sessions.NewInMemorySessionService()
	defer service.Close(ctx)

	stateManager := sessions.NewDefaultStateManager()
	utils := sessions.NewSessionServiceUtils(service, stateManager)

	appName := "utils_demo"
	userID := "user_utils"

	// Create session with defaults
	defaults := map[string]any{
		"created_at":     time.Now(),
		"default_theme":  "light",
		"tutorial_shown": false,
		"feature_flags": map[string]bool{
			"new_ui":        true,
			"beta_features": false,
		},
	}

	session, err := utils.CreateSessionWithDefaults(ctx, appName, userID, defaults)
	if err != nil {
		log.Fatalf("Failed to create session with defaults: %v", err)
	}
	fmt.Printf("Created session with defaults: %s\n", session.ID)

	// Get or create session (should return existing)
	session2, err := utils.GetOrCreateSession(ctx, appName, userID, session.ID)
	if err != nil {
		log.Fatalf("Failed to get or create session: %v", err)
	}
	fmt.Printf("Got existing session: %s (same as %s: %v)\n", session2.ID, session.ID, session2.ID == session.ID)

	// Duplicate session
	newSessionID := "duplicated_session"
	duplicatedSession, err := utils.DuplicateSession(ctx, appName, userID, session.ID, newSessionID)
	if err != nil {
		log.Fatalf("Failed to duplicate session: %v", err)
	}
	fmt.Printf("Duplicated session: %s\n", duplicatedSession.ID)

	// Add events to original session
	event := &core.Event{
		ID:           "evt_utils_demo",
		InvocationID: "inv_utils_demo",
		Author:       "user",
		Content: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{Type: "text", Text: stringPtr("Original session message")},
			},
		},
		Actions:   core.EventActions{StateDelta: map[string]any{"messages_sent": 1}},
		Timestamp: time.Now(),
	}

	err = service.AppendEvent(ctx, session, event)
	if err != nil {
		log.Fatalf("Failed to append event: %v", err)
	}

	// Filter events
	filter := sessions.EventFilter{
		Authors:  []string{"user"},
		FromTime: &[]time.Time{time.Now().Add(-time.Hour)}[0],
	}

	filteredEvents, err := utils.GetSessionHistory(ctx, appName, userID, session.ID, filter)
	if err != nil {
		log.Fatalf("Failed to get filtered events: %v", err)
	}
	fmt.Printf("Filtered events: %d\n", len(filteredEvents))

	// Merge session state
	err = utils.MergeSessionState(ctx, appName, userID, duplicatedSession.ID, []string{session.ID})
	if err != nil {
		log.Fatalf("Failed to merge session state: %v", err)
	}
	fmt.Println("Merged session state")
}

func runBackupRestoreExample(ctx context.Context) {
	// Create session service
	service := sessions.NewInMemorySessionService()
	defer service.Close(ctx)

	backup := sessions.NewSessionBackupManager(service)

	appName := "backup_demo"
	userID := "user_backup"

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		createReq := &core.CreateSessionRequest{
			AppName: appName,
			UserID:  userID,
			State: map[string]any{
				"session_number": i + 1,
				"created_at":     time.Now(),
			},
		}

		session, err := service.CreateSession(ctx, createReq)
		if err != nil {
			log.Fatalf("Failed to create session %d: %v", i+1, err)
		}

		// Add some events
		for j := 0; j < 2; j++ {
			event := &core.Event{
				ID:           fmt.Sprintf("evt_%s_%d", session.ID, j+1),
				InvocationID: fmt.Sprintf("inv_%s", session.ID),
				Author:       "user",
				Content: &core.Content{
					Role: "user",
					Parts: []core.Part{
						{Type: "text", Text: stringPtr(fmt.Sprintf("Message %d in session %d", j+1, i+1))},
					},
				},
				Actions:   core.EventActions{},
				Timestamp: time.Now(),
			}

			err = service.AppendEvent(ctx, session, event)
			if err != nil {
				log.Fatalf("Failed to append event: %v", err)
			}
		}
	}

	// Backup sessions
	backupPath := "./demo_backup.json"
	err := backup.BackupSessions(ctx, appName, userID, backupPath)
	if err != nil {
		log.Fatalf("Failed to backup sessions: %v", err)
	}
	fmt.Printf("Backed up sessions to %s\n", backupPath)

	// Count sessions before clearing
	sessionsBefore, err := service.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		log.Fatalf("Failed to get sessions before clear: %v", err)
	}
	fmt.Printf("Sessions before clear: %d\n", len(sessionsBefore))

	// Clear all sessions
	err = service.BulkDeleteSessions(ctx, appName, userID, []string{
		sessionsBefore[0].ID,
		sessionsBefore[1].ID,
		sessionsBefore[2].ID,
	})
	if err != nil {
		log.Fatalf("Failed to bulk delete sessions: %v", err)
	}

	// Verify sessions are cleared
	sessionsAfterClear, err := service.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		log.Fatalf("Failed to get sessions after clear: %v", err)
	}
	fmt.Printf("Sessions after clear: %d\n", len(sessionsAfterClear))

	// Restore sessions
	err = backup.RestoreSessions(ctx, backupPath, false)
	if err != nil {
		log.Fatalf("Failed to restore sessions: %v", err)
	}

	// Verify sessions are restored
	sessionsAfterRestore, err := service.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		log.Fatalf("Failed to get sessions after restore: %v", err)
	}
	fmt.Printf("Sessions after restore: %d\n", len(sessionsAfterRestore))

	// Verify content of restored sessions
	for _, session := range sessionsAfterRestore {
		sessionDetails, err := service.GetSession(ctx, &core.GetSessionRequest{
			AppName:   appName,
			UserID:    userID,
			SessionID: session.ID,
		})
		if err != nil {
			log.Fatalf("Failed to get restored session details: %v", err)
		}
		fmt.Printf("Restored session %s has %d events\n", session.ID, len(sessionDetails.Events))
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
