// Package sessions provides state management implementations.
package sessions

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// DefaultStateManager implements StateManager with support for scoped state.
type DefaultStateManager struct {
	userStates map[string]map[string]any // app:user -> state
	appStates  map[string]map[string]any // app -> state
	mutex      sync.RWMutex
}

// NewDefaultStateManager creates a new state manager.
func NewDefaultStateManager() *DefaultStateManager {
	return &DefaultStateManager{
		userStates: make(map[string]map[string]any),
		appStates:  make(map[string]map[string]any),
	}
}

// GetState retrieves state value by key with support for scoped keys.
func (s *DefaultStateManager) GetState(ctx context.Context, session *core.Session, key string) (any, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Handle scoped keys
	if strings.HasPrefix(key, "app:") {
		appKey := strings.TrimPrefix(key, "app:")
		return s.getAppStateInternal(session.AppName, appKey)
	}

	if strings.HasPrefix(key, "user:") {
		userKey := strings.TrimPrefix(key, "user:")
		return s.getUserStateInternal(session.AppName, session.UserID, userKey)
	}

	if strings.HasPrefix(key, "temp:") {
		// Temp state is not persisted, only available during processing
		// This would typically be handled by the runner or invocation context
		return nil, false, nil
	}

	// Regular session state
	if session.State == nil {
		return nil, false, nil
	}

	value, exists := session.State[key]
	return value, exists, nil
}

// SetState sets a state value with support for scoped keys.
func (s *DefaultStateManager) SetState(ctx context.Context, session *core.Session, key string, value any) error {
	// Handle scoped keys
	if strings.HasPrefix(key, "app:") {
		appKey := strings.TrimPrefix(key, "app:")
		return s.SetAppState(ctx, session.AppName, appKey, value)
	}

	if strings.HasPrefix(key, "user:") {
		userKey := strings.TrimPrefix(key, "user:")
		return s.SetUserState(ctx, session.AppName, session.UserID, userKey, value)
	}

	if strings.HasPrefix(key, "temp:") {
		// Temp state is not persisted
		// This would typically be handled by the runner or invocation context
		return nil
	}

	// Regular session state
	if session.State == nil {
		session.State = make(map[string]any)
	}

	session.State[key] = value
	return nil
}

// DeleteState removes a state value.
func (s *DefaultStateManager) DeleteState(ctx context.Context, session *core.Session, key string) error {
	// Handle scoped keys
	if strings.HasPrefix(key, "app:") {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		appKey := strings.TrimPrefix(key, "app:")
		if appState, exists := s.appStates[session.AppName]; exists {
			delete(appState, appKey)
		}
		return nil
	}

	if strings.HasPrefix(key, "user:") {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		userKey := strings.TrimPrefix(key, "user:")
		userStateKey := s.userStateKey(session.AppName, session.UserID)
		if userState, exists := s.userStates[userStateKey]; exists {
			delete(userState, userKey)
		}
		return nil
	}

	if strings.HasPrefix(key, "temp:") {
		// Temp state is not persisted
		return nil
	}

	// Regular session state
	if session.State != nil {
		delete(session.State, key)
	}

	return nil
}

// GetUserState gets user-scoped state that persists across sessions.
func (s *DefaultStateManager) GetUserState(ctx context.Context, appName, userID, key string) (any, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getUserStateInternal(appName, userID, key)
}

// SetUserState sets user-scoped state.
func (s *DefaultStateManager) SetUserState(ctx context.Context, appName, userID, key string, value any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	userStateKey := s.userStateKey(appName, userID)
	if s.userStates[userStateKey] == nil {
		s.userStates[userStateKey] = make(map[string]any)
	}

	s.userStates[userStateKey][key] = value
	return nil
}

// GetAppState gets app-scoped state that's shared across all users.
func (s *DefaultStateManager) GetAppState(ctx context.Context, appName, key string) (any, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getAppStateInternal(appName, key)
}

// SetAppState sets app-scoped state.
func (s *DefaultStateManager) SetAppState(ctx context.Context, appName, key string, value any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.appStates[appName] == nil {
		s.appStates[appName] = make(map[string]any)
	}

	s.appStates[appName][key] = value
	return nil
}

// ApplyStateDelta applies a state delta to a session.
func (s *DefaultStateManager) ApplyStateDelta(ctx context.Context, session *core.Session, delta map[string]any) error {
	for key, value := range delta {
		if err := s.SetState(ctx, session, key, value); err != nil {
			return fmt.Errorf("failed to apply state delta for key %s: %w", key, err)
		}
	}
	return nil
}

// GetEffectiveState gets the combined state including app, user, and session scopes.
func (s *DefaultStateManager) GetEffectiveState(ctx context.Context, session *core.Session) (map[string]any, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	effective := make(map[string]any)

	// Start with app state (lowest priority)
	if appState, exists := s.appStates[session.AppName]; exists {
		for k, v := range appState {
			effective["app:"+k] = v
		}
	}

	// Add user state (medium priority)
	userStateKey := s.userStateKey(session.AppName, session.UserID)
	if userState, exists := s.userStates[userStateKey]; exists {
		for k, v := range userState {
			effective["user:"+k] = v
		}
	}

	// Add session state (highest priority)
	if session.State != nil {
		for k, v := range session.State {
			effective[k] = v
		}
	}

	return effective, nil
}

// GetUserStates returns all user states for debugging/admin purposes.
func (s *DefaultStateManager) GetUserStates() map[string]map[string]any {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make(map[string]map[string]any)
	for k, v := range s.userStates {
		result[k] = copyStateMap(v)
	}
	return result
}

// GetAppStates returns all app states for debugging/admin purposes.
func (s *DefaultStateManager) GetAppStates() map[string]map[string]any {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make(map[string]map[string]any)
	for k, v := range s.appStates {
		result[k] = copyStateMap(v)
	}
	return result
}

// ClearUserState removes all state for a specific user.
func (s *DefaultStateManager) ClearUserState(ctx context.Context, appName, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	userStateKey := s.userStateKey(appName, userID)
	delete(s.userStates, userStateKey)
	return nil
}

// ClearAppState removes all state for a specific app.
func (s *DefaultStateManager) ClearAppState(ctx context.Context, appName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.appStates, appName)
	return nil
}

// ImportState imports state data (useful for migration/backup).
func (s *DefaultStateManager) ImportState(userStates, appStates map[string]map[string]any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if userStates != nil {
		s.userStates = userStates
	}

	if appStates != nil {
		s.appStates = appStates
	}

	return nil
}

// ExportState exports state data (useful for migration/backup).
func (s *DefaultStateManager) ExportState() (userStates, appStates map[string]map[string]any) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	userStates = make(map[string]map[string]any)
	for k, v := range s.userStates {
		userStates[k] = copyStateMap(v)
	}

	appStates = make(map[string]map[string]any)
	for k, v := range s.appStates {
		appStates[k] = copyStateMap(v)
	}

	return userStates, appStates
}

// Private helper methods

func (s *DefaultStateManager) userStateKey(appName, userID string) string {
	return fmt.Sprintf("%s:%s", appName, userID)
}

func (s *DefaultStateManager) getUserStateInternal(appName, userID, key string) (any, bool, error) {
	userStateKey := s.userStateKey(appName, userID)
	userState, exists := s.userStates[userStateKey]
	if !exists {
		return nil, false, nil
	}

	value, exists := userState[key]
	return value, exists, nil
}

func (s *DefaultStateManager) getAppStateInternal(appName, key string) (any, bool, error) {
	appState, exists := s.appStates[appName]
	if !exists {
		return nil, false, nil
	}

	value, exists := appState[key]
	return value, exists, nil
}

func copyStateMap(original map[string]any) map[string]any {
	if original == nil {
		return nil
	}

	copied := make(map[string]any)
	for k, v := range original {
		copied[k] = v
	}
	return copied
}

// StateScope represents different state scopes.
type StateScope string

const (
	// SessionScope is for session-specific state.
	SessionScope StateScope = "session"
	// UserScope is for user-specific state that persists across sessions.
	UserScope StateScope = "user"
	// AppScope is for application-wide state shared across all users.
	AppScope StateScope = "app"
	// TempScope is for temporary state that's not persisted.
	TempScope StateScope = "temp"
)

// ScopedKey creates a scoped state key.
func ScopedKey(scope StateScope, key string) string {
	if scope == SessionScope {
		return key
	}
	return fmt.Sprintf("%s:%s", scope, key)
}

// ParseScopedKey parses a scoped state key into scope and key components.
func ParseScopedKey(scopedKey string) (StateScope, string) {
	parts := strings.SplitN(scopedKey, ":", 2)
	if len(parts) == 1 {
		return SessionScope, parts[0]
	}

	switch parts[0] {
	case "app":
		return AppScope, parts[1]
	case "user":
		return UserScope, parts[1]
	case "temp":
		return TempScope, parts[1]
	default:
		return SessionScope, scopedKey
	}
}

// StateHelper provides utility functions for working with state.
type StateHelper struct {
	manager StateManager
}

// NewStateHelper creates a new state helper.
func NewStateHelper(manager StateManager) *StateHelper {
	return &StateHelper{manager: manager}
}

// GetOrDefault gets a state value with a default fallback.
func (h *StateHelper) GetOrDefault(ctx context.Context, session *core.Session, key string, defaultValue any) (any, error) {
	value, exists, err := h.manager.GetState(ctx, session, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return defaultValue, nil
	}
	return value, nil
}

// Increment increments a numeric state value.
func (h *StateHelper) Increment(ctx context.Context, session *core.Session, key string, delta int) (int, error) {
	value, exists, err := h.manager.GetState(ctx, session, key)
	if err != nil {
		return 0, err
	}

	var current int
	if exists {
		if intVal, ok := value.(int); ok {
			current = intVal
		} else if floatVal, ok := value.(float64); ok {
			current = int(floatVal)
		}
	}

	newValue := current + delta
	err = h.manager.SetState(ctx, session, key, newValue)
	return newValue, err
}

// Toggle toggles a boolean state value.
func (h *StateHelper) Toggle(ctx context.Context, session *core.Session, key string) (bool, error) {
	value, exists, err := h.manager.GetState(ctx, session, key)
	if err != nil {
		return false, err
	}

	var current bool
	if exists {
		if boolVal, ok := value.(bool); ok {
			current = boolVal
		}
	}

	newValue := !current
	err = h.manager.SetState(ctx, session, key, newValue)
	return newValue, err
}

// Push adds an item to a list state value.
func (h *StateHelper) Push(ctx context.Context, session *core.Session, key string, item any) error {
	value, exists, err := h.manager.GetState(ctx, session, key)
	if err != nil {
		return err
	}

	var list []any
	if exists {
		if listVal, ok := value.([]any); ok {
			list = listVal
		} else if sliceVal, ok := value.([]interface{}); ok {
			list = sliceVal
		}
	}

	list = append(list, item)
	return h.manager.SetState(ctx, session, key, list)
}

// Pop removes and returns the last item from a list state value.
func (h *StateHelper) Pop(ctx context.Context, session *core.Session, key string) (any, error) {
	value, exists, err := h.manager.GetState(ctx, session, key)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	var list []any
	if listVal, ok := value.([]any); ok {
		list = listVal
	} else if sliceVal, ok := value.([]interface{}); ok {
		list = sliceVal
	} else {
		return nil, fmt.Errorf("state value is not a list")
	}

	if len(list) == 0 {
		return nil, nil
	}

	item := list[len(list)-1]
	list = list[:len(list)-1]
	err = h.manager.SetState(ctx, session, key, list)
	return item, err
}
