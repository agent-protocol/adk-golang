// Package core provides additional session state management utilities.
package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// SessionStateHelper provides utility methods for common state operations.
type SessionStateHelper struct {
	session *Session
}

// NewSessionStateHelper creates a new state helper for the given session.
func NewSessionStateHelper(session *Session) *SessionStateHelper {
	return &SessionStateHelper{session: session}
}

// GetString retrieves a string value from state.
func (h *SessionStateHelper) GetString(key string) (string, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return "", fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// GetStringWithDefault retrieves a string value with a default fallback.
func (h *SessionStateHelper) GetStringWithDefault(key, defaultValue string) string {
	if value, err := h.GetString(key); err == nil {
		return value
	}
	return defaultValue
}

// SetString sets a string value in state.
func (h *SessionStateHelper) SetString(key, value string) {
	h.session.SetState(key, value)
}

// GetInt retrieves an integer value from state.
func (h *SessionStateHelper) GetInt(key string) (int, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return 0, fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// GetIntWithDefault retrieves an integer value with a default fallback.
func (h *SessionStateHelper) GetIntWithDefault(key string, defaultValue int) int {
	if value, err := h.GetInt(key); err == nil {
		return value
	}
	return defaultValue
}

// SetInt sets an integer value in state.
func (h *SessionStateHelper) SetInt(key string, value int) {
	h.session.SetState(key, value)
}

// Increment increments an integer value in state.
func (h *SessionStateHelper) Increment(key string, delta int) (int, error) {
	current := h.GetIntWithDefault(key, 0)
	newValue := current + delta
	h.SetInt(key, newValue)
	return newValue, nil
}

// Decrement decrements an integer value in state.
func (h *SessionStateHelper) Decrement(key string, delta int) (int, error) {
	return h.Increment(key, -delta)
}

// GetBool retrieves a boolean value from state.
func (h *SessionStateHelper) GetBool(key string) (bool, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return false, fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

// GetBoolWithDefault retrieves a boolean value with a default fallback.
func (h *SessionStateHelper) GetBoolWithDefault(key string, defaultValue bool) bool {
	if value, err := h.GetBool(key); err == nil {
		return value
	}
	return defaultValue
}

// SetBool sets a boolean value in state.
func (h *SessionStateHelper) SetBool(key string, value bool) {
	h.session.SetState(key, value)
}

// Toggle toggles a boolean value in state.
func (h *SessionStateHelper) Toggle(key string) bool {
	current := h.GetBoolWithDefault(key, false)
	newValue := !current
	h.SetBool(key, newValue)
	return newValue
}

// GetFloat retrieves a float64 value from state.
func (h *SessionStateHelper) GetFloat(key string) (float64, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return 0, fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// GetFloatWithDefault retrieves a float64 value with a default fallback.
func (h *SessionStateHelper) GetFloatWithDefault(key string, defaultValue float64) float64 {
	if value, err := h.GetFloat(key); err == nil {
		return value
	}
	return defaultValue
}

// SetFloat sets a float64 value in state.
func (h *SessionStateHelper) SetFloat(key string, value float64) {
	h.session.SetState(key, value)
}

// GetTime retrieves a time.Time value from state.
func (h *SessionStateHelper) GetTime(key string) (time.Time, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return time.Time{}, fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		return time.Parse(time.RFC3339, v)
	case int64:
		return time.Unix(v, 0), nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time.Time", value)
	}
}

// GetTimeWithDefault retrieves a time.Time value with a default fallback.
func (h *SessionStateHelper) GetTimeWithDefault(key string, defaultValue time.Time) time.Time {
	if value, err := h.GetTime(key); err == nil {
		return value
	}
	return defaultValue
}

// SetTime sets a time.Time value in state.
func (h *SessionStateHelper) SetTime(key string, value time.Time) {
	h.session.SetState(key, value)
}

// GetSlice retrieves a slice value from state.
func (h *SessionStateHelper) GetSlice(key string) ([]any, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return nil, fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case []any:
		return v, nil
	default:
		// Try to convert using reflection
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice {
			result := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				result[i] = rv.Index(i).Interface()
			}
			return result, nil
		}
		return nil, fmt.Errorf("cannot convert %T to slice", value)
	}
}

// GetSliceWithDefault retrieves a slice value with a default fallback.
func (h *SessionStateHelper) GetSliceWithDefault(key string, defaultValue []any) []any {
	if value, err := h.GetSlice(key); err == nil {
		return value
	}
	return defaultValue
}

// SetSlice sets a slice value in state.
func (h *SessionStateHelper) SetSlice(key string, value []any) {
	h.session.SetState(key, value)
}

// AppendToSlice appends an item to a slice in state.
func (h *SessionStateHelper) AppendToSlice(key string, item any) error {
	current := h.GetSliceWithDefault(key, []any{})
	current = append(current, item)
	h.SetSlice(key, current)
	return nil
}

// PrependToSlice prepends an item to a slice in state.
func (h *SessionStateHelper) PrependToSlice(key string, item any) error {
	current := h.GetSliceWithDefault(key, []any{})
	current = append([]any{item}, current...)
	h.SetSlice(key, current)
	return nil
}

// RemoveFromSlice removes an item from a slice in state by index.
func (h *SessionStateHelper) RemoveFromSlice(key string, index int) error {
	current := h.GetSliceWithDefault(key, []any{})
	if index < 0 || index >= len(current) {
		return fmt.Errorf("index %d out of bounds for slice of length %d", index, len(current))
	}

	current = append(current[:index], current[index+1:]...)
	h.SetSlice(key, current)
	return nil
}

// PopFromSlice removes and returns the last item from a slice in state.
func (h *SessionStateHelper) PopFromSlice(key string) (any, error) {
	current := h.GetSliceWithDefault(key, []any{})
	if len(current) == 0 {
		return nil, fmt.Errorf("cannot pop from empty slice")
	}

	item := current[len(current)-1]
	current = current[:len(current)-1]
	h.SetSlice(key, current)
	return item, nil
}

// GetMap retrieves a map value from state.
func (h *SessionStateHelper) GetMap(key string) (map[string]any, error) {
	value, exists := h.session.GetState(key)
	if !exists {
		return nil, fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case map[string]any:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to map[string]any", value)
	}
}

// GetMapWithDefault retrieves a map value with a default fallback.
func (h *SessionStateHelper) GetMapWithDefault(key string, defaultValue map[string]any) map[string]any {
	if value, err := h.GetMap(key); err == nil {
		return value
	}
	return defaultValue
}

// SetMap sets a map value in state.
func (h *SessionStateHelper) SetMap(key string, value map[string]any) {
	h.session.SetState(key, value)
}

// SetMapKey sets a value in a nested map in state.
func (h *SessionStateHelper) SetMapKey(mapKey, key string, value any) error {
	current := h.GetMapWithDefault(mapKey, make(map[string]any))
	current[key] = value
	h.SetMap(mapKey, current)
	return nil
}

// GetMapKey retrieves a value from a nested map in state.
func (h *SessionStateHelper) GetMapKey(mapKey, key string) (any, error) {
	current, err := h.GetMap(mapKey)
	if err != nil {
		return nil, err
	}

	value, exists := current[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found in map %s", key, mapKey)
	}

	return value, nil
}

// DeleteMapKey removes a key from a nested map in state.
func (h *SessionStateHelper) DeleteMapKey(mapKey, key string) error {
	current, err := h.GetMap(mapKey)
	if err != nil {
		return err
	}

	delete(current, key)
	h.SetMap(mapKey, current)
	return nil
}

// GetJSON unmarshals a JSON value from state into the provided interface.
func (h *SessionStateHelper) GetJSON(key string, target any) error {
	value, exists := h.session.GetState(key)
	if !exists {
		return fmt.Errorf("key %s not found", key)
	}

	switch v := value.(type) {
	case string:
		return json.Unmarshal([]byte(v), target)
	case []byte:
		return json.Unmarshal(v, target)
	default:
		// Try to marshal and unmarshal for type conversion
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		return json.Unmarshal(data, target)
	}
}

// SetJSON marshals a value to JSON and stores it in state.
func (h *SessionStateHelper) SetJSON(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}
	h.session.SetState(key, string(data))
	return nil
}

// SessionMetrics provides metrics about the session.
type SessionMetrics struct {
	EventCount        int            `json:"event_count"`
	StateSize         int            `json:"state_size"`
	ErrorCount        int            `json:"error_count"`
	LastActivity      time.Time      `json:"last_activity"`
	SessionAge        time.Duration  `json:"session_age"`
	FunctionCallCount int            `json:"function_call_count"`
	AuthorEventCounts map[string]int `json:"author_event_counts"`
	EventsByType      map[string]int `json:"events_by_type"`
}

// GetMetrics returns comprehensive metrics about the session.
func (s *Session) GetMetrics() *SessionMetrics {
	metrics := &SessionMetrics{
		EventCount:        len(s.Events),
		StateSize:         len(s.State),
		LastActivity:      s.LastUpdateTime,
		SessionAge:        time.Since(s.LastUpdateTime),
		AuthorEventCounts: make(map[string]int),
		EventsByType:      make(map[string]int),
	}

	functionCallCount := 0
	errorCount := 0

	for _, event := range s.Events {
		// Count by author
		metrics.AuthorEventCounts[event.Author]++

		// Count errors
		if event.ErrorMessage != nil {
			errorCount++
		}

		// Count function calls
		functionCallCount += len(event.GetFunctionCalls())

		// Count by event type
		eventType := getEventTypeFromEvent(event)
		metrics.EventsByType[eventType]++
	}

	metrics.ErrorCount = errorCount
	metrics.FunctionCallCount = functionCallCount

	return metrics
}

// getEventTypeFromEvent determines the type of an event.
func getEventTypeFromEvent(event *Event) string {
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

// SessionSnapshot represents a point-in-time snapshot of session state.
type SessionSnapshot struct {
	SessionID   string         `json:"session_id"`
	Timestamp   time.Time      `json:"timestamp"`
	State       map[string]any `json:"state"`
	EventCount  int            `json:"event_count"`
	LastEventID string         `json:"last_event_id,omitempty"`
}

// CreateSnapshot creates a snapshot of the current session state.
func (s *Session) CreateSnapshot() *SessionSnapshot {
	snapshot := &SessionSnapshot{
		SessionID:  s.ID,
		Timestamp:  time.Now(),
		State:      s.CopyState(),
		EventCount: len(s.Events),
	}

	if lastEvent := s.GetLastEvent(); lastEvent != nil {
		snapshot.LastEventID = lastEvent.ID
	}

	return snapshot
}

// RestoreFromSnapshot restores session state from a snapshot.
func (s *Session) RestoreFromSnapshot(snapshot *SessionSnapshot) error {
	if snapshot.SessionID != s.ID {
		return fmt.Errorf("snapshot session ID %s does not match current session ID %s",
			snapshot.SessionID, s.ID)
	}

	s.State = make(map[string]any)
	for k, v := range snapshot.State {
		s.State[k] = v
	}

	s.LastUpdateTime = time.Now()
	return nil
}

// SessionDiff represents the difference between two session states.
type SessionDiff struct {
	Added    map[string]any `json:"added"`
	Modified map[string]any `json:"modified"`
	Removed  []string       `json:"removed"`
}

// DiffState compares the current session state with another state map.
func (s *Session) DiffState(other map[string]any) *SessionDiff {
	diff := &SessionDiff{
		Added:    make(map[string]any),
		Modified: make(map[string]any),
		Removed:  make([]string, 0),
	}

	// Find added and modified keys
	for k, v := range s.State {
		if otherValue, exists := other[k]; exists {
			if !reflect.DeepEqual(v, otherValue) {
				diff.Modified[k] = v
			}
		} else {
			diff.Added[k] = v
		}
	}

	// Find removed keys
	for k := range other {
		if _, exists := s.State[k]; !exists {
			diff.Removed = append(diff.Removed, k)
		}
	}

	return diff
}
