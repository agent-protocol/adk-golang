// Package core defines request/response types and context structures for the ADK framework.
package core

import (
	"fmt"
	"time"
)

// Session represents a conversation session between users and agents.
type Session struct {
	ID             string         `json:"id"`
	AppName        string         `json:"app_name"`
	UserID         string         `json:"user_id"`
	State          map[string]any `json:"state"`
	Events         []*Event       `json:"events"`
	LastUpdateTime time.Time      `json:"last_update_time"`
}

// NewSession creates a new session with the given parameters.
func NewSession(id, appName, userID string) *Session {
	return &Session{
		ID:             id,
		AppName:        appName,
		UserID:         userID,
		State:          make(map[string]any),
		Events:         make([]*Event, 0),
		LastUpdateTime: time.Now(),
	}
}

// SetState sets a value in the session state.
func (s *Session) SetState(key string, value any) {
	if s.State == nil {
		s.State = make(map[string]any)
	}
	s.State[key] = value
	s.LastUpdateTime = time.Now()
}

// GetState retrieves a value from the session state.
func (s *Session) GetState(key string) (any, bool) {
	if s.State == nil {
		return nil, false
	}
	value, exists := s.State[key]
	return value, exists
}

// GetStateWithDefault retrieves a value from the session state with a default fallback.
func (s *Session) GetStateWithDefault(key string, defaultValue any) any {
	if value, exists := s.GetState(key); exists {
		return value
	}
	return defaultValue
}

// DeleteState removes a key from the session state.
func (s *Session) DeleteState(key string) {
	if s.State != nil {
		delete(s.State, key)
		s.LastUpdateTime = time.Now()
	}
}

// HasState checks if a key exists in the session state.
func (s *Session) HasState(key string) bool {
	_, exists := s.GetState(key)
	return exists
}

// ClearState removes all state keys.
func (s *Session) ClearState() {
	s.State = make(map[string]any)
	s.LastUpdateTime = time.Now()
}

// UpdateState merges the provided state delta into the current state.
func (s *Session) UpdateState(delta map[string]any) {
	if s.State == nil {
		s.State = make(map[string]any)
	}
	for k, v := range delta {
		s.State[k] = v
	}
	s.LastUpdateTime = time.Now()
}

// GetStateKeys returns all state keys.
func (s *Session) GetStateKeys() []string {
	if s.State == nil {
		return nil
	}
	keys := make([]string, 0, len(s.State))
	for k := range s.State {
		keys = append(keys, k)
	}
	return keys
}

// GetStateSize returns the number of state keys.
func (s *Session) GetStateSize() int {
	if s.State == nil {
		return 0
	}
	return len(s.State)
}

// CopyState returns a copy of the current state.
func (s *Session) CopyState() map[string]any {
	if s.State == nil {
		return make(map[string]any)
	}
	copied := make(map[string]any, len(s.State))
	for k, v := range s.State {
		copied[k] = v
	}
	return copied
}

// AddEvent appends an event to the session.
func (s *Session) AddEvent(event *Event) {
	if s.Events == nil {
		s.Events = make([]*Event, 0)
	}
	s.Events = append(s.Events, event)
	s.LastUpdateTime = time.Now()

	// Apply state delta from event actions
	if len(event.Actions.StateDelta) > 0 {
		s.UpdateState(event.Actions.StateDelta)
	}
}

// GetLastEvent returns the most recent event, or nil if no events exist.
func (s *Session) GetLastEvent() *Event {
	if len(s.Events) == 0 {
		return nil
	}
	return s.Events[len(s.Events)-1]
}

// GetEventCount returns the number of events in the session.
func (s *Session) GetEventCount() int {
	return len(s.Events)
}

// GetEventsByAuthor returns all events by a specific author.
func (s *Session) GetEventsByAuthor(author string) []*Event {
	var events []*Event
	for _, event := range s.Events {
		if event.Author == author {
			events = append(events, event)
		}
	}
	return events
}

// GetEventsAfter returns all events after the specified time.
func (s *Session) GetEventsAfter(after time.Time) []*Event {
	var events []*Event
	for _, event := range s.Events {
		if event.Timestamp.After(after) {
			events = append(events, event)
		}
	}
	return events
}

// GetEventsByInvocation returns all events for a specific invocation.
func (s *Session) GetEventsByInvocation(invocationID string) []*Event {
	var events []*Event
	for _, event := range s.Events {
		if event.InvocationID == invocationID {
			events = append(events, event)
		}
	}
	return events
}

// ClearEvents removes all events from the session.
func (s *Session) ClearEvents() {
	s.Events = make([]*Event, 0)
	s.LastUpdateTime = time.Now()
}

// TrimEvents keeps only the last N events.
func (s *Session) TrimEvents(maxEvents int) {
	if len(s.Events) > maxEvents {
		s.Events = s.Events[len(s.Events)-maxEvents:]
		s.LastUpdateTime = time.Now()
	}
}

// HasErrors checks if any events in the session contain errors.
func (s *Session) HasErrors() bool {
	for _, event := range s.Events {
		if event.ErrorMessage != nil {
			return true
		}
	}
	return false
}

// GetErrorEvents returns all events that contain errors.
func (s *Session) GetErrorEvents() []*Event {
	var errorEvents []*Event
	for _, event := range s.Events {
		if event.ErrorMessage != nil {
			errorEvents = append(errorEvents, event)
		}
	}
	return errorEvents
}

// GetFunctionCalls returns all function calls across all events.
func (s *Session) GetFunctionCalls() []*FunctionCall {
	var calls []*FunctionCall
	for _, event := range s.Events {
		calls = append(calls, event.GetFunctionCalls()...)
	}
	return calls
}

// GetFunctionResponses returns all function responses across all events.
func (s *Session) GetFunctionResponses() []*FunctionResponse {
	var responses []*FunctionResponse
	for _, event := range s.Events {
		responses = append(responses, event.GetFunctionResponses()...)
	}
	return responses
}

// Clone creates a deep copy of the session.
func (s *Session) Clone() *Session {
	clone := &Session{
		ID:             s.ID,
		AppName:        s.AppName,
		UserID:         s.UserID,
		State:          s.CopyState(),
		Events:         make([]*Event, len(s.Events)),
		LastUpdateTime: s.LastUpdateTime,
	}

	// Deep copy events
	copy(clone.Events, s.Events)

	return clone
}

// IsEmpty checks if the session has no state and no events.
func (s *Session) IsEmpty() bool {
	return len(s.State) == 0 && len(s.Events) == 0
}

// GetAge returns the duration since the session was last updated.
func (s *Session) GetAge() time.Duration {
	return time.Since(s.LastUpdateTime)
}

// Touch updates the LastUpdateTime to the current time.
func (s *Session) Touch() {
	s.LastUpdateTime = time.Now()
}

// Validate performs basic validation on the session.
func (s *Session) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if s.AppName == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if s.UserID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	return nil
}

// InvocationContext represents the context for a single agent invocation.
type InvocationContext struct {
	InvocationID      string
	Agent             BaseAgent
	Session           *Session
	SessionService    SessionService
	ArtifactService   ArtifactService
	MemoryService     MemoryService
	CredentialService CredentialService
	UserContent       *Content
	Branch            *string
	RunConfig         *RunConfig
	EndInvocation     bool
}

// ToolContext provides context for tool execution.
type ToolContext struct {
	InvocationContext *InvocationContext
	State             *State
	Actions           *EventActions
	FunctionCallID    *string
}

// ReadonlyContext provides read-only access to context information.
type ReadonlyContext struct {
	Session *Session
	UserID  string
	AppName string
	State   map[string]any
}

// RunConfig contains configuration options for agent execution.
type RunConfig struct {
	SaveInputBlobsAsArtifacts bool           `json:"save_input_blobs_as_artifacts"`
	MaxTurns                  *int           `json:"max_turns,omitempty"`
	Timeout                   *time.Duration `json:"timeout,omitempty"`
}

// LLMRequest represents a request to a language model.
type LLMRequest struct {
	Contents []Content              `json:"contents"`
	Config   *LLMConfig             `json:"config,omitempty"`
	Tools    []*FunctionDeclaration `json:"tools,omitempty"`
}

// LLMResponse represents a response from a language model.
type LLMResponse struct {
	Content  *Content       `json:"content,omitempty"`
	Partial  *bool          `json:"partial,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// LLMConfig contains configuration for LLM requests.
type LLMConfig struct {
	Model             string                 `json:"model"`
	Temperature       *float32               `json:"temperature,omitempty"`
	MaxTokens         *int                   `json:"max_tokens,omitempty"`
	TopP              *float32               `json:"top_p,omitempty"`
	TopK              *int                   `json:"top_k,omitempty"`
	Tools             []*FunctionDeclaration `json:"tools,omitempty"`
	SystemInstruction *string                `json:"system_instruction,omitempty"`
}

// Credential represents authentication credentials.
type Credential struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
}

// Request types for services

// CreateSessionRequest contains parameters for creating a new session.
type CreateSessionRequest struct {
	AppName   string         `json:"app_name"`
	UserID    string         `json:"user_id"`
	State     map[string]any `json:"state,omitempty"`
	SessionID *string        `json:"session_id,omitempty"`
}

// GetSessionRequest contains parameters for retrieving a session.
type GetSessionRequest struct {
	AppName   string            `json:"app_name"`
	UserID    string            `json:"user_id"`
	SessionID string            `json:"session_id"`
	Config    *GetSessionConfig `json:"config,omitempty"`
}

// GetSessionConfig contains configuration for session retrieval.
type GetSessionConfig struct {
	IncludeEvents bool `json:"include_events"`
	MaxEvents     *int `json:"max_events,omitempty"`
}

// DeleteSessionRequest contains parameters for deleting a session.
type DeleteSessionRequest struct {
	AppName   string `json:"app_name"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

// ListSessionsRequest contains parameters for listing sessions.
type ListSessionsRequest struct {
	AppName string `json:"app_name"`
	UserID  string `json:"user_id"`
	Limit   *int   `json:"limit,omitempty"`
	Offset  *int   `json:"offset,omitempty"`
}

// ListSessionsResponse contains the result of listing sessions.
type ListSessionsResponse struct {
	Sessions   []*Session `json:"sessions"`
	TotalCount int        `json:"total_count"`
	HasMore    bool       `json:"has_more"`
}

// SaveArtifactRequest contains parameters for saving an artifact.
type SaveArtifactRequest struct {
	AppName   string `json:"app_name"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
	Content   []byte `json:"content"`
	MimeType  string `json:"mime_type"`
}

// LoadArtifactRequest contains parameters for loading an artifact.
type LoadArtifactRequest struct {
	AppName   string `json:"app_name"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
	Version   *int   `json:"version,omitempty"`
}

// ListArtifactKeysRequest contains parameters for listing artifact keys.
type ListArtifactKeysRequest struct {
	AppName   string `json:"app_name"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

// DeleteArtifactRequest contains parameters for deleting an artifact.
type DeleteArtifactRequest struct {
	AppName   string `json:"app_name"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
}

// ListVersionsRequest contains parameters for listing artifact versions.
type ListVersionsRequest struct {
	AppName   string `json:"app_name"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
}

// RetrieveMemoryRequest contains parameters for memory retrieval.
type RetrieveMemoryRequest struct {
	AppName string `json:"app_name"`
	UserID  string `json:"user_id"`
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
}

// RunRequest contains parameters for running an agent.
type RunRequest struct {
	UserID     string     `json:"user_id"`
	SessionID  string     `json:"session_id"`
	NewMessage *Content   `json:"new_message"`
	RunConfig  *RunConfig `json:"run_config,omitempty"`
}

// NewInvocationContext creates a new invocation context.
func NewInvocationContext(
	invocationID string,
	agent BaseAgent,
	session *Session,
	sessionService SessionService,
) *InvocationContext {
	return &InvocationContext{
		InvocationID:   invocationID,
		Agent:          agent,
		Session:        session,
		SessionService: sessionService,
		RunConfig:      &RunConfig{},
	}
}

// NewToolContext creates a new tool context.
func NewToolContext(invocationCtx *InvocationContext) *ToolContext {
	return &ToolContext{
		InvocationContext: invocationCtx,
		State:             NewState(),
		Actions:           &EventActions{},
	}
}

// NewReadonlyContext creates a new readonly context.
func NewReadonlyContext(session *Session) *ReadonlyContext {
	return &ReadonlyContext{
		Session: session,
		UserID:  session.UserID,
		AppName: session.AppName,
		State:   session.State,
	}
}
