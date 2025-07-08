// Package sessions provides tests for session management.
package sessions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func TestInMemorySessionService(t *testing.T) {
	service := NewInMemorySessionService()
	testSessionService(t, service)
}

func TestFileSessionService(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultSessionConfiguration()
	config.PersistenceBackend = "file"

	service, err := NewFileSessionService(tempDir, config)
	if err != nil {
		t.Fatalf("Failed to create file session service: %v", err)
	}
	defer service.Close(context.Background())

	testSessionService(t, service)

	// Test file persistence
	testFilePersistence(t, service, tempDir)
}

func testSessionService(t *testing.T, service SessionService) {
	ctx := context.Background()
	appName := "test_app"
	userID := "test_user"

	// Test creating a session
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{"key1": "value1"},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.AppName != appName {
		t.Errorf("Expected app name %s, got %s", appName, session.AppName)
	}
	if session.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, session.UserID)
	}
	if session.State["key1"] != "value1" {
		t.Errorf("Expected state key1=value1, got %v", session.State["key1"])
	}

	sessionID := session.ID

	// Test getting a session
	getReq := &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	}

	retrievedSession, err := service.GetSession(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if retrievedSession == nil {
		t.Fatal("Retrieved session is nil")
	}
	if retrievedSession.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, retrievedSession.ID)
	}

	// Test appending an event
	event := &core.Event{
		ID:           "test_event_1",
		InvocationID: "test_invocation",
		Author:       "test_agent",
		Content: &core.Content{
			Role: "assistant",
			Parts: []core.Part{
				{Type: "text", Text: ptr.Ptr("Hello, world!")},
			},
		},
		Actions:   core.EventActions{StateDelta: map[string]any{"key2": "value2"}},
		Timestamp: time.Now(),
	}

	err = service.AppendEvent(ctx, session, event)
	if err != nil {
		t.Fatalf("Failed to append event: %v", err)
	}

	// Verify event was added and state was updated
	updatedSession, err := service.GetSession(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}
	if len(updatedSession.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(updatedSession.Events))
	}
	if updatedSession.State["key2"] != "value2" {
		t.Errorf("Expected state key2=value2, got %v", updatedSession.State["key2"])
	}

	// Test listing sessions
	listReq := &core.ListSessionsRequest{
		AppName: appName,
		UserID:  userID,
	}

	listResp, err := service.ListSessions(ctx, listReq)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}
	if len(listResp.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(listResp.Sessions))
	}

	// Test deleting a session
	deleteReq := &core.DeleteSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	}

	err = service.DeleteSession(ctx, deleteReq)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session was deleted
	deletedSession, err := service.GetSession(ctx, getReq)
	if err != nil {
		t.Fatalf("Error getting deleted session: %v", err)
	}
	if deletedSession != nil {
		t.Error("Session should be nil after deletion")
	}
}

func testFilePersistence(t *testing.T, service *FileSessionService, tempDir string) {
	ctx := context.Background()
	appName := "persistence_test"
	userID := "test_user"

	// Create a session
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{"persistent": "data"},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Close the service
	service.Close(ctx)

	// Create a new service instance
	config := DefaultSessionConfiguration()
	newService, err := NewFileSessionService(tempDir, config)
	if err != nil {
		t.Fatalf("Failed to create new file session service: %v", err)
	}
	defer newService.Close(ctx)

	// Verify the session still exists
	getReq := &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: session.ID,
	}

	persistedSession, err := newService.GetSession(ctx, getReq)
	if err != nil {
		t.Fatalf("Failed to get persisted session: %v", err)
	}
	if persistedSession == nil {
		t.Fatal("Persisted session is nil")
	}
	if persistedSession.State["persistent"] != "data" {
		t.Errorf("Expected persistent data, got %v", persistedSession.State["persistent"])
	}
}

func TestStateManager(t *testing.T) {
	ctx := context.Background()
	stateManager := NewDefaultStateManager()

	session := &core.Session{
		ID:      "test_session",
		AppName: "test_app",
		UserID:  "test_user",
		State:   map[string]any{"session_key": "session_value"},
	}

	// Test session state
	value, exists, err := stateManager.GetState(ctx, session, "session_key")
	if err != nil {
		t.Fatalf("Failed to get session state: %v", err)
	}
	if !exists {
		t.Error("Session key should exist")
	}
	if value != "session_value" {
		t.Errorf("Expected session_value, got %v", value)
	}

	// Test setting session state
	err = stateManager.SetState(ctx, session, "new_key", "new_value")
	if err != nil {
		t.Fatalf("Failed to set session state: %v", err)
	}
	if session.State["new_key"] != "new_value" {
		t.Errorf("Expected new_value, got %v", session.State["new_key"])
	}

	// Test user state
	err = stateManager.SetUserState(ctx, session.AppName, session.UserID, "user_key", "user_value")
	if err != nil {
		t.Fatalf("Failed to set user state: %v", err)
	}

	value, exists, err = stateManager.GetUserState(ctx, session.AppName, session.UserID, "user_key")
	if err != nil {
		t.Fatalf("Failed to get user state: %v", err)
	}
	if !exists {
		t.Error("User key should exist")
	}
	if value != "user_value" {
		t.Errorf("Expected user_value, got %v", value)
	}

	// Test scoped keys
	value, exists, err = stateManager.GetState(ctx, session, "user:user_key")
	if err != nil {
		t.Fatalf("Failed to get scoped user state: %v", err)
	}
	if !exists {
		t.Error("Scoped user key should exist")
	}
	if value != "user_value" {
		t.Errorf("Expected user_value, got %v", value)
	}

	// Test app state
	err = stateManager.SetAppState(ctx, session.AppName, "app_key", "app_value")
	if err != nil {
		t.Fatalf("Failed to set app state: %v", err)
	}

	value, exists, err = stateManager.GetState(ctx, session, "app:app_key")
	if err != nil {
		t.Fatalf("Failed to get scoped app state: %v", err)
	}
	if !exists {
		t.Error("Scoped app key should exist")
	}
	if value != "app_value" {
		t.Errorf("Expected app_value, got %v", value)
	}

	// Test effective state
	effectiveState, err := stateManager.GetEffectiveState(ctx, session)
	if err != nil {
		t.Fatalf("Failed to get effective state: %v", err)
	}

	if effectiveState["session_key"] != "session_value" {
		t.Errorf("Expected session_value in effective state, got %v", effectiveState["session_key"])
	}
	if effectiveState["user:user_key"] != "user_value" {
		t.Errorf("Expected user_value in effective state, got %v", effectiveState["user:user_key"])
	}
	if effectiveState["app:app_key"] != "app_value" {
		t.Errorf("Expected app_value in effective state, got %v", effectiveState["app:app_key"])
	}
}

func TestStateHelper(t *testing.T) {
	ctx := context.Background()
	stateManager := NewDefaultStateManager()
	helper := NewStateHelper(stateManager)

	session := &core.Session{
		ID:      "test_session",
		AppName: "test_app",
		UserID:  "test_user",
		State:   make(map[string]any),
	}

	// Test GetOrDefault
	value, err := helper.GetOrDefault(ctx, session, "missing_key", "default_value")
	if err != nil {
		t.Fatalf("Failed to get or default: %v", err)
	}
	if value != "default_value" {
		t.Errorf("Expected default_value, got %v", value)
	}

	// Test Increment
	newValue, err := helper.Increment(ctx, session, "counter", 5)
	if err != nil {
		t.Fatalf("Failed to increment: %v", err)
	}
	if newValue != 5 {
		t.Errorf("Expected 5, got %d", newValue)
	}

	newValue, err = helper.Increment(ctx, session, "counter", 3)
	if err != nil {
		t.Fatalf("Failed to increment: %v", err)
	}
	if newValue != 8 {
		t.Errorf("Expected 8, got %d", newValue)
	}

	// Test Toggle
	toggled, err := helper.Toggle(ctx, session, "flag")
	if err != nil {
		t.Fatalf("Failed to toggle: %v", err)
	}
	if !toggled {
		t.Error("Expected true after first toggle")
	}

	toggled, err = helper.Toggle(ctx, session, "flag")
	if err != nil {
		t.Fatalf("Failed to toggle: %v", err)
	}
	if toggled {
		t.Error("Expected false after second toggle")
	}

	// Test Push and Pop
	err = helper.Push(ctx, session, "list", "item1")
	if err != nil {
		t.Fatalf("Failed to push: %v", err)
	}

	err = helper.Push(ctx, session, "list", "item2")
	if err != nil {
		t.Fatalf("Failed to push: %v", err)
	}

	item, err := helper.Pop(ctx, session, "list")
	if err != nil {
		t.Fatalf("Failed to pop: %v", err)
	}
	if item != "item2" {
		t.Errorf("Expected item2, got %v", item)
	}

	item, err = helper.Pop(ctx, session, "list")
	if err != nil {
		t.Fatalf("Failed to pop: %v", err)
	}
	if item != "item1" {
		t.Errorf("Expected item1, got %v", item)
	}
}

func TestSessionServiceBuilder(t *testing.T) {
	// Test memory backend
	service, err := NewSessionServiceBuilder().
		WithMemoryBackend().
		WithMaxSessionsPerUser(10).
		WithMaxEventsPerSession(100).
		Build()
	if err != nil {
		t.Fatalf("Failed to build memory session service: %v", err)
	}
	defer service.Close(context.Background())

	// Test file backend
	tempDir := t.TempDir()
	service2, err := NewSessionServiceBuilder().
		WithFileBackend(tempDir).
		WithSessionTTL(time.Hour).
		WithAutoCleanup(time.Minute).
		Build()
	if err != nil {
		t.Fatalf("Failed to build file session service: %v", err)
	}
	defer service2.Close(context.Background())
}

func TestEventHandlers(t *testing.T) {
	ctx := context.Background()

	// Test logging handler
	logger := &DefaultLogger{}
	loggingHandler := NewLoggingEventHandler(logger)

	session := &core.Session{
		ID:      "test_session",
		AppName: "test_app",
		UserID:  "test_user",
		State:   map[string]any{"key": "value"},
		Events:  []*core.Event{},
	}

	err := loggingHandler.OnSessionCreated(ctx, session)
	if err != nil {
		t.Errorf("Logging handler should not return error: %v", err)
	}

	// Test metrics handler
	metrics := &DefaultMetricsCollector{}
	metricsHandler := NewMetricsEventHandler(metrics)

	err = metricsHandler.OnSessionCreated(ctx, session)
	if err != nil {
		t.Errorf("Metrics handler should not return error: %v", err)
	}

	// Test validation handler
	validationConfig := &ValidationConfig{
		MaxStateSize: 1000,
		MaxEventSize: 500,
	}
	validationHandler := NewValidationEventHandler(validationConfig)

	err = validationHandler.OnSessionCreated(ctx, session)
	if err != nil {
		t.Errorf("Validation handler should not return error for valid session: %v", err)
	}

	// Test composite handler
	composite := NewCompositeEventHandler(loggingHandler, metricsHandler, validationHandler)

	err = composite.OnSessionCreated(ctx, session)
	if err != nil {
		t.Errorf("Composite handler should not return error: %v", err)
	}
}

func TestSessionBackup(t *testing.T) {
	ctx := context.Background()
	service := NewInMemorySessionService()
	defer service.Close(ctx)

	backup := NewSessionBackupManager(service)
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "backup.json")

	appName := "backup_test"
	userID := "test_user"

	// Create some test sessions
	for i := 0; i < 3; i++ {
		createReq := &core.CreateSessionRequest{
			AppName: appName,
			UserID:  userID,
			State:   map[string]any{"index": i},
		}

		session, err := service.CreateSession(ctx, createReq)
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Add an event
		event := &core.Event{
			ID:           "event_" + session.ID,
			InvocationID: "test_invocation",
			Author:       "test_agent",
			Timestamp:    time.Now(),
		}

		err = service.AppendEvent(ctx, session, event)
		if err != nil {
			t.Fatalf("Failed to add event: %v", err)
		}
	}

	// Backup sessions
	err := backup.BackupSessions(ctx, appName, userID, backupPath)
	if err != nil {
		t.Fatalf("Failed to backup sessions: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("Backup file does not exist")
	}

	// Clear sessions
	sessions, err := service.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		t.Fatalf("Failed to get sessions: %v", err)
	}

	for _, session := range sessions {
		err = service.DeleteSession(ctx, &core.DeleteSessionRequest{
			AppName:   appName,
			UserID:    userID,
			SessionID: session.ID,
		})
		if err != nil {
			t.Fatalf("Failed to delete session: %v", err)
		}
	}

	// Restore sessions
	err = backup.RestoreSessions(ctx, backupPath, false)
	if err != nil {
		t.Fatalf("Failed to restore sessions: %v", err)
	}

	// Verify sessions were restored
	restoredSessions, err := service.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		t.Fatalf("Failed to get restored sessions: %v", err)
	}

	if len(restoredSessions) != 3 {
		t.Errorf("Expected 3 restored sessions, got %d", len(restoredSessions))
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	ctx := context.Background()
	service := NewInMemorySessionService()
	defer service.Close(ctx)

	appName := "cleanup_test"
	userID := "test_user"

	// Create test sessions
	createReq := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   map[string]any{"test": "data"},
	}

	session, err := service.CreateSession(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Simulate old session by manually adjusting timestamp
	service.mutex.Lock()
	key := service.sessionKey(appName, userID, session.ID)
	if storedSession, exists := service.sessions[key]; exists {
		storedSession.LastUpdateTime = time.Now().Add(-2 * time.Hour)
	}
	service.mutex.Unlock()

	// Clean up expired sessions
	deleted, err := service.CleanupExpiredSessions(ctx, time.Hour)
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted session, got %d", deleted)
	}

	// Verify session was deleted
	getReq := &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: session.ID,
	}

	deletedSession, err := service.GetSession(ctx, getReq)
	if err != nil {
		t.Fatalf("Error getting deleted session: %v", err)
	}
	if deletedSession != nil {
		t.Error("Session should be nil after cleanup")
	}
}
