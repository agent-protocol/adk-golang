// Package sessions provides session management implementations.
package sessions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

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

// sessionKey creates a unique key for session storage.
func (s *InMemorySessionService) sessionKey(appName, userID, sessionID string) string {
	return fmt.Sprintf("%s:%s:%s", appName, userID, sessionID)
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
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
