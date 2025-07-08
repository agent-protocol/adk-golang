// Package sessions provides file-based session persistence.
package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// FileSessionService implements SessionService using file-based storage.
type FileSessionService struct {
	baseDir    string
	config     *SessionConfiguration
	mutex      sync.RWMutex
	handlers   []SessionEventHandler
	stateDir   string
	userStates map[string]map[string]any // app:user -> state
	appStates  map[string]map[string]any // app -> state
	stateMutex sync.RWMutex
}

// FileBackendConfig contains configuration for file-based storage.
type FileBackendConfig struct {
	BaseDirectory string `json:"base_directory"`
	FileMode      string `json:"file_mode"` // e.g., "0644"
}

// NewFileSessionService creates a new file-based session service.
func NewFileSessionService(baseDir string, config *SessionConfiguration) (*FileSessionService, error) {
	if config == nil {
		config = DefaultSessionConfiguration()
	}

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	stateDir := filepath.Join(baseDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	service := &FileSessionService{
		baseDir:    baseDir,
		config:     config,
		handlers:   make([]SessionEventHandler, 0),
		stateDir:   stateDir,
		userStates: make(map[string]map[string]any),
		appStates:  make(map[string]map[string]any),
	}

	// Load existing state
	if err := service.loadState(); err != nil {
		return nil, fmt.Errorf("failed to load existing state: %w", err)
	}

	// Start auto-cleanup if configured
	if config.AutoCleanupInterval > 0 {
		go service.autoCleanupLoop()
	}

	return service, nil
}

// CreateSession creates a new session.
func (f *FileSessionService) CreateSession(ctx context.Context, req *core.CreateSessionRequest) (*core.Session, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	sessionID := req.SessionID
	if sessionID == nil {
		id := generateSessionID()
		sessionID = &id
	}

	// Check if session already exists
	sessionPath := f.getSessionPath(req.AppName, req.UserID, *sessionID)
	if _, err := os.Stat(sessionPath); err == nil {
		return nil, fmt.Errorf("session already exists: %s", *sessionID)
	}

	// Check session limits
	if f.config.MaxSessionsPerUser > 0 {
		existing, err := f.countUserSessions(req.AppName, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to count existing sessions: %w", err)
		}
		if existing >= f.config.MaxSessionsPerUser {
			return nil, fmt.Errorf("user has reached maximum sessions limit: %d", f.config.MaxSessionsPerUser)
		}
	}

	session := &core.Session{
		ID:             *sessionID,
		AppName:        req.AppName,
		UserID:         req.UserID,
		State:          req.State,
		Events:         make([]*core.Event, 0),
		LastUpdateTime: time.Now(),
	}

	if session.State == nil {
		session.State = make(map[string]any)
	}

	// Ensure directory exists
	sessionDir := filepath.Dir(sessionPath)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Save session
	if err := f.saveSession(session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Trigger event handlers
	for _, handler := range f.handlers {
		if err := handler.OnSessionCreated(ctx, session); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Session created handler error: %v\n", err)
		}
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (f *FileSessionService) GetSession(ctx context.Context, req *core.GetSessionRequest) (*core.Session, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	sessionPath := f.getSessionPath(req.AppName, req.UserID, req.SessionID)
	session, err := f.loadSession(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return nil, not an error, when session doesn't exist
		}
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Apply config if provided
	if req.Config != nil {
		if !req.Config.IncludeEvents {
			session.Events = nil
		} else if req.Config.MaxEvents != nil && len(session.Events) > *req.Config.MaxEvents {
			// Keep only the last N events
			start := len(session.Events) - *req.Config.MaxEvents
			session.Events = session.Events[start:]
		}
	}

	return session, nil
}

// AppendEvent adds an event to a session.
func (f *FileSessionService) AppendEvent(ctx context.Context, session *core.Session, event *core.Event) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Load current session from disk to get latest state
	sessionPath := f.getSessionPath(session.AppName, session.UserID, session.ID)
	currentSession, err := f.loadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("failed to load current session: %w", err)
	}

	// Check event limits
	if f.config.MaxEventsPerSession > 0 && len(currentSession.Events) >= f.config.MaxEventsPerSession {
		// Remove oldest events to make room
		excess := len(currentSession.Events) - f.config.MaxEventsPerSession + 1
		currentSession.Events = currentSession.Events[excess:]
	}

	// Add event to session
	currentSession.Events = append(currentSession.Events, event)
	currentSession.LastUpdateTime = time.Now()

	// Apply state changes from event actions
	oldState := copyMap(currentSession.State)
	if len(event.Actions.StateDelta) > 0 {
		if currentSession.State == nil {
			currentSession.State = make(map[string]any)
		}
		for k, v := range event.Actions.StateDelta {
			currentSession.State[k] = v
		}
	}

	// Save updated session
	if err := f.saveSession(currentSession); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Update the passed session object
	session.Events = currentSession.Events
	session.State = copyMap(currentSession.State)
	session.LastUpdateTime = currentSession.LastUpdateTime

	// Trigger event handlers
	for _, handler := range f.handlers {
		if err := handler.OnEventAdded(ctx, session, event); err != nil {
			fmt.Printf("Event added handler error: %v\n", err)
		}
		if len(oldState) > 0 || len(currentSession.State) > 0 {
			if err := handler.OnSessionUpdated(ctx, session, oldState); err != nil {
				fmt.Printf("Session updated handler error: %v\n", err)
			}
		}
	}

	return nil
}

// DeleteSession removes a session.
func (f *FileSessionService) DeleteSession(ctx context.Context, req *core.DeleteSessionRequest) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	sessionPath := f.getSessionPath(req.AppName, req.UserID, req.SessionID)

	// Check if session exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil // Not an error if session doesn't exist
	}

	// Remove session file
	if err := os.Remove(sessionPath); err != nil {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	// Clean up empty directories
	f.cleanupEmptyDirs(filepath.Dir(sessionPath))

	// Trigger event handlers
	for _, handler := range f.handlers {
		if err := handler.OnSessionDeleted(ctx, req.AppName, req.UserID, req.SessionID); err != nil {
			fmt.Printf("Session deleted handler error: %v\n", err)
		}
	}

	return nil
}

// ListSessions returns sessions for a user.
func (f *FileSessionService) ListSessions(ctx context.Context, req *core.ListSessionsRequest) (*core.ListSessionsResponse, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	userDir := f.getUserDir(req.AppName, req.UserID)

	var sessions []*core.Session

	// Walk through user directory to find sessions
	err := filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			session, err := f.loadSession(path)
			if err != nil {
				return nil // Skip files that can't be loaded
			}

			// Create copy without events for listing
			sessionCopy := &core.Session{
				ID:             session.ID,
				AppName:        session.AppName,
				UserID:         session.UserID,
				State:          copyMap(session.State),
				Events:         nil, // Don't include events in list
				LastUpdateTime: session.LastUpdateTime,
			}
			sessions = append(sessions, sessionCopy)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk user directory: %w", err)
	}

	// Apply pagination
	totalCount := len(sessions)

	offset := 0
	if req.Offset != nil {
		offset = *req.Offset
	}

	limit := totalCount
	if req.Limit != nil {
		limit = *req.Limit
	}

	start := offset
	end := offset + limit

	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedSessions := sessions[start:end]
	hasMore := end < totalCount

	return &core.ListSessionsResponse{
		Sessions:   paginatedSessions,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}, nil
}

// GetSessionsByUser returns all sessions for a specific user.
func (f *FileSessionService) GetSessionsByUser(ctx context.Context, appName, userID string) ([]*core.Session, error) {
	req := &core.ListSessionsRequest{
		AppName: appName,
		UserID:  userID,
	}

	response, err := f.ListSessions(ctx, req)
	if err != nil {
		return nil, err
	}

	return response.Sessions, nil
}

// UpdateSessionState updates the state of an existing session.
func (f *FileSessionService) UpdateSessionState(ctx context.Context, appName, userID, sessionID string, state map[string]any) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	sessionPath := f.getSessionPath(appName, userID, sessionID)
	session, err := f.loadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	oldState := copyMap(session.State)
	session.State = copyMap(state)
	session.LastUpdateTime = time.Now()

	if err := f.saveSession(session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Trigger event handlers
	for _, handler := range f.handlers {
		if err := handler.OnSessionUpdated(ctx, session, oldState); err != nil {
			fmt.Printf("Session updated handler error: %v\n", err)
		}
	}

	return nil
}

// GetSessionState retrieves only the state of a session.
func (f *FileSessionService) GetSessionState(ctx context.Context, appName, userID, sessionID string) (map[string]any, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	sessionPath := f.getSessionPath(appName, userID, sessionID)
	session, err := f.loadSession(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	return copyMap(session.State), nil
}

// ClearSessionEvents removes all events from a session while keeping the session and state.
func (f *FileSessionService) ClearSessionEvents(ctx context.Context, appName, userID, sessionID string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	sessionPath := f.getSessionPath(appName, userID, sessionID)
	session, err := f.loadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	session.Events = make([]*core.Event, 0)
	session.LastUpdateTime = time.Now()

	return f.saveSession(session)
}

// GetSessionsModifiedAfter returns sessions modified after the specified time.
func (f *FileSessionService) GetSessionsModifiedAfter(ctx context.Context, appName, userID string, after time.Time) ([]*core.Session, error) {
	sessions, err := f.GetSessionsByUser(ctx, appName, userID)
	if err != nil {
		return nil, err
	}

	var result []*core.Session
	for _, session := range sessions {
		if session.LastUpdateTime.After(after) {
			result = append(result, session)
		}
	}

	return result, nil
}

// CleanupExpiredSessions removes sessions that haven't been updated within the specified duration.
func (f *FileSessionService) CleanupExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	err := filepath.Walk(f.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			// Check if this is a session file by trying to load it
			session, err := f.loadSession(path)
			if err != nil {
				return nil // Skip files that can't be loaded as sessions
			}

			if session.LastUpdateTime.Before(cutoff) {
				if err := os.Remove(path); err == nil {
					deleted++
					// Trigger deletion handlers
					for _, handler := range f.handlers {
						if err := handler.OnSessionDeleted(ctx, session.AppName, session.UserID, session.ID); err != nil {
							fmt.Printf("Session deleted handler error: %v\n", err)
						}
					}
				}
			}
		}
		return nil
	})

	return deleted, err
}

// GetSessionMetadata returns lightweight metadata about a session without loading full content.
func (f *FileSessionService) GetSessionMetadata(ctx context.Context, appName, userID, sessionID string) (*SessionMetadata, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	sessionPath := f.getSessionPath(appName, userID, sessionID)
	session, err := f.loadSession(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	stateKeys := make([]string, 0, len(session.State))
	for key := range session.State {
		stateKeys = append(stateKeys, key)
	}

	hasErrors := false
	for _, event := range session.Events {
		if event.ErrorMessage != nil {
			hasErrors = true
			break
		}
	}

	return &SessionMetadata{
		ID:             session.ID,
		AppName:        session.AppName,
		UserID:         session.UserID,
		EventCount:     len(session.Events),
		LastUpdateTime: session.LastUpdateTime,
		StateKeys:      stateKeys,
		HasErrors:      hasErrors,
	}, nil
}

// BulkDeleteSessions deletes multiple sessions efficiently.
func (f *FileSessionService) BulkDeleteSessions(ctx context.Context, appName, userID string, sessionIDs []string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	for _, sessionID := range sessionIDs {
		sessionPath := f.getSessionPath(appName, userID, sessionID)
		if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete session %s: %w", sessionID, err)
		}

		// Trigger deletion handlers
		for _, handler := range f.handlers {
			if err := handler.OnSessionDeleted(ctx, appName, userID, sessionID); err != nil {
				fmt.Printf("Session deleted handler error: %v\n", err)
			}
		}
	}

	// Clean up empty directories
	userDir := f.getUserDir(appName, userID)
	f.cleanupEmptyDirs(userDir)

	return nil
}

// Close performs cleanup operations and closes resources.
func (f *FileSessionService) Close(ctx context.Context) error {
	// Save state before closing
	return f.saveState()
}

// AddEventHandler adds a session event handler.
func (f *FileSessionService) AddEventHandler(handler SessionEventHandler) {
	f.handlers = append(f.handlers, handler)
}

// Private helper methods

func (f *FileSessionService) getSessionPath(appName, userID, sessionID string) string {
	return filepath.Join(f.getUserDir(appName, userID), sessionID+".json")
}

func (f *FileSessionService) getUserDir(appName, userID string) string {
	return filepath.Join(f.baseDir, "sessions", appName, userID)
}

func (f *FileSessionService) saveSession(session *core.Session) error {
	sessionPath := f.getSessionPath(session.AppName, session.UserID, session.ID)

	// Ensure directory exists
	sessionDir := filepath.Dir(sessionPath)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionPath, data, 0644)
}

func (f *FileSessionService) loadSession(sessionPath string) (*core.Session, error) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}

	var session core.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (f *FileSessionService) countUserSessions(appName, userID string) (int, error) {
	userDir := f.getUserDir(appName, userID)

	count := 0
	err := filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			count++
		}
		return nil
	})

	return count, err
}

func (f *FileSessionService) cleanupEmptyDirs(dir string) {
	// Don't delete base directories
	if dir == f.baseDir || dir == filepath.Join(f.baseDir, "sessions") {
		return
	}

	// Check if directory is empty
	file, err := os.Open(dir)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Readdirnames(1)
	if err == io.EOF {
		// Directory is empty, remove it
		os.Remove(dir)
		// Recursively cleanup parent directories
		f.cleanupEmptyDirs(filepath.Dir(dir))
	}
}

func (f *FileSessionService) autoCleanupLoop() {
	ticker := time.NewTicker(f.config.AutoCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		if f.config.SessionTTL > 0 {
			deleted, err := f.CleanupExpiredSessions(context.Background(), f.config.SessionTTL)
			if err != nil {
				fmt.Printf("Auto cleanup error: %v\n", err)
			} else if deleted > 0 {
				fmt.Printf("Auto cleanup: deleted %d expired sessions\n", deleted)
			}
		}
	}
}

// State management methods

func (f *FileSessionService) loadState() error {
	f.stateMutex.Lock()
	defer f.stateMutex.Unlock()

	// Load user states
	userStatePath := filepath.Join(f.stateDir, "user_states.json")
	if data, err := os.ReadFile(userStatePath); err == nil {
		json.Unmarshal(data, &f.userStates)
	}

	// Load app states
	appStatePath := filepath.Join(f.stateDir, "app_states.json")
	if data, err := os.ReadFile(appStatePath); err == nil {
		json.Unmarshal(data, &f.appStates)
	}

	return nil
}

func (f *FileSessionService) saveState() error {
	f.stateMutex.RLock()
	defer f.stateMutex.RUnlock()

	// Save user states
	if data, err := json.MarshalIndent(f.userStates, "", "  "); err == nil {
		userStatePath := filepath.Join(f.stateDir, "user_states.json")
		os.WriteFile(userStatePath, data, 0644)
	}

	// Save app states
	if data, err := json.MarshalIndent(f.appStates, "", "  "); err == nil {
		appStatePath := filepath.Join(f.stateDir, "app_states.json")
		os.WriteFile(appStatePath, data, 0644)
	}

	return nil
}
