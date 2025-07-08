// Package sessions provides session management implementations and utilities.
package sessions

import (
	"context"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// SessionService extends the core SessionService with additional methods.
type SessionService interface {
	core.SessionService

	// GetSessionsByUser returns all sessions for a specific user.
	GetSessionsByUser(ctx context.Context, appName, userID string) ([]*core.Session, error)

	// UpdateSessionState updates the state of an existing session.
	UpdateSessionState(ctx context.Context, appName, userID, sessionID string, state map[string]any) error

	// GetSessionState retrieves only the state of a session.
	GetSessionState(ctx context.Context, appName, userID, sessionID string) (map[string]any, error)

	// ClearSessionEvents removes all events from a session while keeping the session and state.
	ClearSessionEvents(ctx context.Context, appName, userID, sessionID string) error

	// GetSessionsModifiedAfter returns sessions modified after the specified time.
	GetSessionsModifiedAfter(ctx context.Context, appName, userID string, after time.Time) ([]*core.Session, error)

	// CleanupExpiredSessions removes sessions that haven't been updated within the specified duration.
	CleanupExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error)

	// GetSessionMetadata returns lightweight metadata about a session without loading full content.
	GetSessionMetadata(ctx context.Context, appName, userID, sessionID string) (*SessionMetadata, error)

	// BulkDeleteSessions deletes multiple sessions efficiently.
	BulkDeleteSessions(ctx context.Context, appName, userID string, sessionIDs []string) error

	// Close performs cleanup operations and closes resources.
	Close(ctx context.Context) error
}

// SessionMetadata contains lightweight information about a session.
type SessionMetadata struct {
	ID             string    `json:"id"`
	AppName        string    `json:"app_name"`
	UserID         string    `json:"user_id"`
	EventCount     int       `json:"event_count"`
	LastUpdateTime time.Time `json:"last_update_time"`
	StateKeys      []string  `json:"state_keys"`
	HasErrors      bool      `json:"has_errors"`
}

// SessionStats provides statistics about sessions.
type SessionStats struct {
	TotalSessions   int    `json:"total_sessions"`
	ActiveSessions  int    `json:"active_sessions"`
	TotalEvents     int    `json:"total_events"`
	TotalApps       int    `json:"total_apps"`
	TotalUsers      int    `json:"total_users"`
	LargestSession  int    `json:"largest_session_events"`
	OldestSessionID string `json:"oldest_session_id"`
}

// StateManager handles session state operations.
type StateManager interface {
	// GetState retrieves state value by key with support for scoped keys.
	GetState(ctx context.Context, session *core.Session, key string) (any, bool, error)

	// SetState sets a state value with support for scoped keys.
	SetState(ctx context.Context, session *core.Session, key string, value any) error

	// DeleteState removes a state value.
	DeleteState(ctx context.Context, session *core.Session, key string) error

	// GetUserState gets user-scoped state that persists across sessions.
	GetUserState(ctx context.Context, appName, userID, key string) (any, bool, error)

	// SetUserState sets user-scoped state.
	SetUserState(ctx context.Context, appName, userID, key string, value any) error

	// GetAppState gets app-scoped state that's shared across all users.
	GetAppState(ctx context.Context, appName, key string) (any, bool, error)

	// SetAppState sets app-scoped state.
	SetAppState(ctx context.Context, appName, key string, value any) error

	// ApplyStateDelta applies a state delta to a session.
	ApplyStateDelta(ctx context.Context, session *core.Session, delta map[string]any) error

	// GetEffectiveState gets the combined state including app, user, and session scopes.
	GetEffectiveState(ctx context.Context, session *core.Session) (map[string]any, error)
}

// SessionEventHandler handles session lifecycle events.
type SessionEventHandler interface {
	// OnSessionCreated is called when a new session is created.
	OnSessionCreated(ctx context.Context, session *core.Session) error

	// OnSessionUpdated is called when a session is updated.
	OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error

	// OnSessionDeleted is called when a session is deleted.
	OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error

	// OnEventAdded is called when an event is added to a session.
	OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error
}

// SessionPersistence defines the interface for session storage backends.
type SessionPersistence interface {
	// Store saves a session to the persistence layer.
	Store(ctx context.Context, session *core.Session) error

	// Load retrieves a session from the persistence layer.
	Load(ctx context.Context, appName, userID, sessionID string) (*core.Session, error)

	// Delete removes a session from the persistence layer.
	Delete(ctx context.Context, appName, userID, sessionID string) error

	// List returns sessions matching the criteria.
	List(ctx context.Context, appName, userID string, limit, offset int) ([]*core.Session, error)

	// Exists checks if a session exists.
	Exists(ctx context.Context, appName, userID, sessionID string) (bool, error)

	// GetStats returns statistics about stored sessions.
	GetStats(ctx context.Context) (*SessionStats, error)

	// Close closes the persistence layer.
	Close(ctx context.Context) error
}

// SessionConfiguration contains configuration options for session services.
type SessionConfiguration struct {
	// MaxSessionsPerUser limits the number of sessions per user (0 = unlimited).
	MaxSessionsPerUser int `json:"max_sessions_per_user"`

	// MaxEventsPerSession limits the number of events per session (0 = unlimited).
	MaxEventsPerSession int `json:"max_events_per_session"`

	// SessionTTL is the time-to-live for sessions (0 = no expiration).
	SessionTTL time.Duration `json:"session_ttl"`

	// AutoCleanupInterval is how often to run cleanup operations (0 = no auto cleanup).
	AutoCleanupInterval time.Duration `json:"auto_cleanup_interval"`

	// PersistenceBackend specifies the storage backend ("memory", "file", "database").
	PersistenceBackend string `json:"persistence_backend"`

	// BackendConfig contains backend-specific configuration.
	BackendConfig map[string]any `json:"backend_config"`

	// EnableEventHandlers enables session lifecycle event handlers.
	EnableEventHandlers bool `json:"enable_event_handlers"`

	// EnableMetrics enables metrics collection.
	EnableMetrics bool `json:"enable_metrics"`
}

// DefaultSessionConfiguration returns default configuration.
func DefaultSessionConfiguration() *SessionConfiguration {
	return &SessionConfiguration{
		MaxSessionsPerUser:  0, // unlimited
		MaxEventsPerSession: 1000,
		SessionTTL:          24 * time.Hour,
		AutoCleanupInterval: time.Hour,
		PersistenceBackend:  "memory",
		BackendConfig:       make(map[string]any),
		EnableEventHandlers: false,
		EnableMetrics:       false,
	}
}
