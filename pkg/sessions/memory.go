// Package sessions provides session management implementations.
package sessions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

var _ core.SessionService = (*InMemorySessionService)(nil)

// InMemorySessionService implements SessionService using in-memory storage.
type InMemorySessionService struct {
	sessions map[string]*core.Session
	mutex    sync.RWMutex
}

// NewInMemorySessionService creates a new in-memory session service.
func NewInMemorySessionService() *InMemorySessionService {
	return &InMemorySessionService{
		sessions: make(map[string]*core.Session),
	}
}

// CreateSession creates a new session.
func (s *InMemorySessionService) CreateSession(ctx context.Context, req *core.CreateSessionRequest) (*core.Session, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sessionID := req.SessionID
	if sessionID == nil {
		id := generateSessionID()
		sessionID = &id
	}

	// Check if session already exists
	key := s.sessionKey(req.AppName, req.UserID, *sessionID)
	if _, exists := s.sessions[key]; exists {
		return nil, fmt.Errorf("session already exists: %s", *sessionID)
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

	s.sessions[key] = session
	return session, nil
}

// GetSession retrieves a session by ID.
func (s *InMemorySessionService) GetSession(ctx context.Context, req *core.GetSessionRequest) (*core.Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.sessionKey(req.AppName, req.UserID, req.SessionID)
	session, exists := s.sessions[key]
	if !exists {
		return nil, nil // Return nil, not an error, when session doesn't exist
	}

	// Make a copy to avoid external modifications
	sessionCopy := &core.Session{
		ID:             session.ID,
		AppName:        session.AppName,
		UserID:         session.UserID,
		State:          copyMap(session.State),
		Events:         make([]*core.Event, len(session.Events)),
		LastUpdateTime: session.LastUpdateTime,
	}

	copy(sessionCopy.Events, session.Events)

	// Apply config if provided
	if req.Config != nil {
		if !req.Config.IncludeEvents {
			sessionCopy.Events = nil
		} else if req.Config.MaxEvents != nil && len(sessionCopy.Events) > *req.Config.MaxEvents {
			// Keep only the last N events
			start := len(sessionCopy.Events) - *req.Config.MaxEvents
			sessionCopy.Events = sessionCopy.Events[start:]
		}
	}

	return sessionCopy, nil
}

// AppendEvent adds an event to a session.
func (s *InMemorySessionService) AppendEvent(ctx context.Context, session *core.Session, event *core.Event) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := s.sessionKey(session.AppName, session.UserID, session.ID)
	storedSession, exists := s.sessions[key]
	if !exists {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	// Add event to session
	storedSession.Events = append(storedSession.Events, event)
	storedSession.LastUpdateTime = time.Now()

	// Apply state changes from event actions
	if len(event.Actions.StateDelta) > 0 {
		if storedSession.State == nil {
			storedSession.State = make(map[string]any)
		}
		for k, v := range event.Actions.StateDelta {
			storedSession.State[k] = v
		}
	}

	// Update the passed session object
	session.Events = storedSession.Events
	session.State = copyMap(storedSession.State)
	session.LastUpdateTime = storedSession.LastUpdateTime

	return nil
}

// DeleteSession removes a session.
func (s *InMemorySessionService) DeleteSession(ctx context.Context, req *core.DeleteSessionRequest) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := s.sessionKey(req.AppName, req.UserID, req.SessionID)
	delete(s.sessions, key)
	return nil
}

// ListSessions returns sessions for a user.
func (s *InMemorySessionService) ListSessions(ctx context.Context, req *core.ListSessionsRequest) (*core.ListSessionsResponse, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var sessions []*core.Session

	// Find sessions for the user
	for _, session := range s.sessions {
		if session.AppName == req.AppName && session.UserID == req.UserID {
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
func (s *InMemorySessionService) GetSessionsByUser(ctx context.Context, appName, userID string) ([]*core.Session, error) {
	req := &core.ListSessionsRequest{
		AppName: appName,
		UserID:  userID,
	}

	response, err := s.ListSessions(ctx, req)
	if err != nil {
		return nil, err
	}

	return response.Sessions, nil
}

// UpdateSessionState updates the state of an existing session.
func (s *InMemorySessionService) UpdateSessionState(ctx context.Context, appName, userID, sessionID string, state map[string]any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := s.sessionKey(appName, userID, sessionID)
	session, exists := s.sessions[key]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.State = copyMap(state)
	session.LastUpdateTime = time.Now()
	return nil
}

// GetSessionState retrieves only the state of a session.
func (s *InMemorySessionService) GetSessionState(ctx context.Context, appName, userID, sessionID string) (map[string]any, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.sessionKey(appName, userID, sessionID)
	session, exists := s.sessions[key]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return copyMap(session.State), nil
}

// ClearSessionEvents removes all events from a session while keeping the session and state.
func (s *InMemorySessionService) ClearSessionEvents(ctx context.Context, appName, userID, sessionID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := s.sessionKey(appName, userID, sessionID)
	session, exists := s.sessions[key]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Events = make([]*core.Event, 0)
	session.LastUpdateTime = time.Now()
	return nil
}

// GetSessionsModifiedAfter returns sessions modified after the specified time.
func (s *InMemorySessionService) GetSessionsModifiedAfter(ctx context.Context, appName, userID string, after time.Time) ([]*core.Session, error) {
	sessions, err := s.GetSessionsByUser(ctx, appName, userID)
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
func (s *InMemorySessionService) CleanupExpiredSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for key, session := range s.sessions {
		if session.LastUpdateTime.Before(cutoff) {
			delete(s.sessions, key)
			deleted++
		}
	}

	return deleted, nil
}

// GetSessionMetadata returns lightweight metadata about a session without loading full content.
func (s *InMemorySessionService) GetSessionMetadata(ctx context.Context, appName, userID, sessionID string) (*SessionMetadata, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.sessionKey(appName, userID, sessionID)
	session, exists := s.sessions[key]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
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
func (s *InMemorySessionService) BulkDeleteSessions(ctx context.Context, appName, userID string, sessionIDs []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, sessionID := range sessionIDs {
		key := s.sessionKey(appName, userID, sessionID)
		delete(s.sessions, key)
	}

	return nil
}

// Close performs cleanup operations and closes resources.
func (s *InMemorySessionService) Close(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clear all sessions
	s.sessions = make(map[string]*core.Session)
	return nil
}

// sessionKey creates a unique key for session storage.
func (s *InMemorySessionService) sessionKey(appName, userID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s", appName, userID, sessionID)
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	return fmt.Sprintf("session_%d_%d", time.Now().UnixNano(), time.Now().Unix()%1000)
}

// copyMap creates a deep copy of a map.
func copyMap(original map[string]any) map[string]any {
	if original == nil {
		return nil
	}

	copied := make(map[string]any)
	for k, v := range original {
		copied[k] = v
	}
	return copied
}
