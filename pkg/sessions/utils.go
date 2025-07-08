// Package sessions provides utilities for creating and managing session services.
package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// SessionServiceFactory creates session service instances based on configuration.
type SessionServiceFactory struct {
	defaultConfig *SessionConfiguration
}

// NewSessionServiceFactory creates a new session service factory.
func NewSessionServiceFactory(defaultConfig *SessionConfiguration) *SessionServiceFactory {
	if defaultConfig == nil {
		defaultConfig = DefaultSessionConfiguration()
	}
	return &SessionServiceFactory{
		defaultConfig: defaultConfig,
	}
}

// CreateSessionService creates a session service based on the configuration.
func (f *SessionServiceFactory) CreateSessionService(config *SessionConfiguration) (SessionService, error) {
	if config == nil {
		config = f.defaultConfig
	}

	switch strings.ToLower(config.PersistenceBackend) {
	case "memory":
		return NewInMemorySessionService(), nil

	case "file":
		baseDir := "./sessions"
		if config.BackendConfig != nil {
			if dir, ok := config.BackendConfig["base_directory"].(string); ok {
				baseDir = dir
			}
		}
		return NewFileSessionService(baseDir, config)

	default:
		return nil, fmt.Errorf("unsupported persistence backend: %s", config.PersistenceBackend)
	}
}

// SessionServiceBuilder provides a fluent interface for creating session services.
type SessionServiceBuilder struct {
	config *SessionConfiguration
}

// NewSessionServiceBuilder creates a new session service builder.
func NewSessionServiceBuilder() *SessionServiceBuilder {
	return &SessionServiceBuilder{
		config: DefaultSessionConfiguration(),
	}
}

// WithMemoryBackend configures the service to use in-memory storage.
func (b *SessionServiceBuilder) WithMemoryBackend() *SessionServiceBuilder {
	b.config.PersistenceBackend = "memory"
	return b
}

// WithFileBackend configures the service to use file-based storage.
func (b *SessionServiceBuilder) WithFileBackend(baseDirectory string) *SessionServiceBuilder {
	b.config.PersistenceBackend = "file"
	if b.config.BackendConfig == nil {
		b.config.BackendConfig = make(map[string]any)
	}
	b.config.BackendConfig["base_directory"] = baseDirectory
	return b
}

// WithMaxSessionsPerUser sets the maximum number of sessions per user.
func (b *SessionServiceBuilder) WithMaxSessionsPerUser(max int) *SessionServiceBuilder {
	b.config.MaxSessionsPerUser = max
	return b
}

// WithMaxEventsPerSession sets the maximum number of events per session.
func (b *SessionServiceBuilder) WithMaxEventsPerSession(max int) *SessionServiceBuilder {
	b.config.MaxEventsPerSession = max
	return b
}

// WithSessionTTL sets the session time-to-live.
func (b *SessionServiceBuilder) WithSessionTTL(ttl time.Duration) *SessionServiceBuilder {
	b.config.SessionTTL = ttl
	return b
}

// WithAutoCleanup enables automatic cleanup of expired sessions.
func (b *SessionServiceBuilder) WithAutoCleanup(interval time.Duration) *SessionServiceBuilder {
	b.config.AutoCleanupInterval = interval
	return b
}

// WithEventHandlers enables session event handlers.
func (b *SessionServiceBuilder) WithEventHandlers() *SessionServiceBuilder {
	b.config.EnableEventHandlers = true
	return b
}

// WithMetrics enables metrics collection.
func (b *SessionServiceBuilder) WithMetrics() *SessionServiceBuilder {
	b.config.EnableMetrics = true
	return b
}

// Build creates the session service with the configured options.
func (b *SessionServiceBuilder) Build() (SessionService, error) {
	factory := NewSessionServiceFactory(nil)
	return factory.CreateSessionService(b.config)
}

// SessionServiceUtils provides utility functions for working with sessions.
type SessionServiceUtils struct {
	service SessionService
	state   StateManager
}

// NewSessionServiceUtils creates new session service utilities.
func NewSessionServiceUtils(service SessionService, state StateManager) *SessionServiceUtils {
	return &SessionServiceUtils{
		service: service,
		state:   state,
	}
}

// CreateSessionWithDefaults creates a session with default state values.
func (u *SessionServiceUtils) CreateSessionWithDefaults(ctx context.Context, appName, userID string, defaults map[string]any) (*core.Session, error) {
	req := &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   defaults,
	}
	return u.service.CreateSession(ctx, req)
}

// GetOrCreateSession gets an existing session or creates a new one.
func (u *SessionServiceUtils) GetOrCreateSession(ctx context.Context, appName, userID, sessionID string) (*core.Session, error) {
	// Try to get existing session
	getReq := &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	}

	session, err := u.service.GetSession(ctx, getReq)
	if err != nil {
		return nil, err
	}

	if session != nil {
		return session, nil
	}

	// Create new session
	createReq := &core.CreateSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: &sessionID,
	}

	return u.service.CreateSession(ctx, createReq)
}

// DuplicateSession creates a copy of an existing session.
func (u *SessionServiceUtils) DuplicateSession(ctx context.Context, appName, userID, sourceSessionID, newSessionID string) (*core.Session, error) {
	// Get source session
	getReq := &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sourceSessionID,
	}

	sourceSession, err := u.service.GetSession(ctx, getReq)
	if err != nil {
		return nil, err
	}
	if sourceSession == nil {
		return nil, fmt.Errorf("source session not found: %s", sourceSessionID)
	}

	// Create new session with same state
	createReq := &core.CreateSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: &newSessionID,
		State:     copySessionState(sourceSession.State),
	}

	return u.service.CreateSession(ctx, createReq)
}

// MergeSessionState merges state from multiple sessions into a target session.
func (u *SessionServiceUtils) MergeSessionState(ctx context.Context, appName, userID, targetSessionID string, sourceSessionIDs []string) error {
	// Get target session
	targetSession, err := u.service.GetSession(ctx, &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: targetSessionID,
	})
	if err != nil {
		return err
	}
	if targetSession == nil {
		return fmt.Errorf("target session not found: %s", targetSessionID)
	}

	// Merge state from source sessions
	mergedState := copySessionState(targetSession.State)
	if mergedState == nil {
		mergedState = make(map[string]any)
	}

	for _, sourceSessionID := range sourceSessionIDs {
		sourceSession, err := u.service.GetSession(ctx, &core.GetSessionRequest{
			AppName:   appName,
			UserID:    userID,
			SessionID: sourceSessionID,
		})
		if err != nil {
			return err
		}
		if sourceSession == nil {
			continue // Skip missing sessions
		}

		// Merge state
		for k, v := range sourceSession.State {
			mergedState[k] = v
		}
	}

	// Update target session
	return u.service.UpdateSessionState(ctx, appName, userID, targetSessionID, mergedState)
}

// GetSessionHistory returns events from a session with optional filtering.
func (u *SessionServiceUtils) GetSessionHistory(ctx context.Context, appName, userID, sessionID string, filter EventFilter) ([]*core.Event, error) {
	session, err := u.service.GetSession(ctx, &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var filteredEvents []*core.Event
	for _, event := range session.Events {
		if filter.Include(event) {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return filteredEvents, nil
}

// EventFilter defines criteria for filtering events.
type EventFilter struct {
	Authors    []string // Filter by event authors
	FromTime   *time.Time
	ToTime     *time.Time
	HasErrors  *bool    // Filter events with/without errors
	EventTypes []string // Filter by event types (function_call, response, etc.)
}

// Include determines if an event should be included based on the filter.
func (f *EventFilter) Include(event *core.Event) bool {
	// Filter by authors
	if len(f.Authors) > 0 {
		found := false
		for _, author := range f.Authors {
			if event.Author == author {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by time range
	if f.FromTime != nil && event.Timestamp.Before(*f.FromTime) {
		return false
	}
	if f.ToTime != nil && event.Timestamp.After(*f.ToTime) {
		return false
	}

	// Filter by error presence
	if f.HasErrors != nil {
		hasError := event.ErrorMessage != nil
		if *f.HasErrors != hasError {
			return false
		}
	}

	// Filter by event types
	if len(f.EventTypes) > 0 {
		eventType := getEventType(event)
		found := false
		for _, t := range f.EventTypes {
			if eventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// getEventType determines the type of an event based on its content.
func getEventType(event *core.Event) string {
	if event.ErrorMessage != nil {
		return "error"
	}

	if event.Content != nil {
		for _, part := range event.Content.Parts {
			if part.FunctionCall != nil {
				return "function_call"
			}
			if part.FunctionResponse != nil {
				return "function_response"
			}
			if part.Text != nil {
				return "text"
			}
		}
	}

	return "unknown"
}

// SessionBackupManager handles backup and restore operations.
type SessionBackupManager struct {
	service SessionService
}

// NewSessionBackupManager creates a new backup manager.
func NewSessionBackupManager(service SessionService) *SessionBackupManager {
	return &SessionBackupManager{service: service}
}

// BackupSessions exports sessions to a backup file.
func (b *SessionBackupManager) BackupSessions(ctx context.Context, appName, userID, backupPath string) error {
	sessions, err := b.service.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		return fmt.Errorf("failed to get sessions: %w", err)
	}

	// Ensure backup directory exists
	backupDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create backup data
	backup := SessionBackup{
		AppName:   appName,
		UserID:    userID,
		Sessions:  sessions,
		Timestamp: time.Now(),
		Version:   "1.0",
	}

	// Write backup file
	return writeJSONFile(backupPath, backup)
}

// RestoreSessions imports sessions from a backup file.
func (b *SessionBackupManager) RestoreSessions(ctx context.Context, backupPath string, overwrite bool) error {
	var backup SessionBackup
	if err := readJSONFile(backupPath, &backup); err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	for _, session := range backup.Sessions {
		// Check if session exists
		existing, err := b.service.GetSession(ctx, &core.GetSessionRequest{
			AppName:   session.AppName,
			UserID:    session.UserID,
			SessionID: session.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to check existing session: %w", err)
		}

		if existing != nil && !overwrite {
			continue // Skip existing sessions if not overwriting
		}

		if existing != nil {
			// Delete existing session
			if err := b.service.DeleteSession(ctx, &core.DeleteSessionRequest{
				AppName:   session.AppName,
				UserID:    session.UserID,
				SessionID: session.ID,
			}); err != nil {
				return fmt.Errorf("failed to delete existing session: %w", err)
			}
		}

		// Create restored session
		_, err = b.service.CreateSession(ctx, &core.CreateSessionRequest{
			AppName:   session.AppName,
			UserID:    session.UserID,
			SessionID: &session.ID,
			State:     session.State,
		})
		if err != nil {
			return fmt.Errorf("failed to create restored session: %w", err)
		}

		// Add events
		for _, event := range session.Events {
			if err := b.service.AppendEvent(ctx, session, event); err != nil {
				return fmt.Errorf("failed to add event to restored session: %w", err)
			}
		}
	}

	return nil
}

// SessionBackup represents a backup of session data.
type SessionBackup struct {
	AppName   string          `json:"app_name"`
	UserID    string          `json:"user_id"`
	Sessions  []*core.Session `json:"sessions"`
	Timestamp time.Time       `json:"timestamp"`
	Version   string          `json:"version"`
}

// Helper functions

func copySessionState(state map[string]any) map[string]any {
	if state == nil {
		return nil
	}
	copied := make(map[string]any)
	for k, v := range state {
		copied[k] = v
	}
	return copied
}

func writeJSONFile(path string, data any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func readJSONFile(path string, data any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(data)
}
