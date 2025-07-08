// Package core defines the fundamental types and interfaces for the ADK framework.
package core

import (
	"encoding/json"
	"log"
	"time"
)

// Content represents the content of a message or event.
// This mirrors the genai types.Content structure.
type Content struct {
	Role  string `json:"role"`  // "user", "agent", or "model"
	Parts []Part `json:"parts"` // Message parts (text, function calls, etc.)
}

// Part represents a component of a message.
// This is a union type that can be text, function call, function response, etc.
type Part struct {
	Type             string            `json:"type,omitempty"`
	Text             *string           `json:"text,omitempty"`
	FunctionCall     *FunctionCall     `json:"function_call,omitempty"`
	FunctionResponse *FunctionResponse `json:"function_response,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
}

// FunctionCall represents a request to execute a tool.
type FunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// FunctionResponse represents the result of a tool execution.
type FunctionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// FunctionDeclaration describes a tool that can be called by the agent.
type FunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// AuthConfig represents authentication configuration for tools.
type AuthConfig struct {
	Scheme     string         `json:"scheme"`
	Config     map[string]any `json:"config,omitempty"`
	Credential any            `json:"credential,omitempty"`
}

// EventActions represents side effects and control flow from an event.
type EventActions struct {
	SkipSummarization    *bool                 `json:"skip_summarization,omitempty"`
	StateDelta           map[string]any        `json:"state_delta,omitempty"`
	ArtifactDelta        map[string]int        `json:"artifact_delta,omitempty"`
	TransferToAgent      *string               `json:"transfer_to_agent,omitempty"`
	Escalate             *bool                 `json:"escalate,omitempty"`
	RequestedAuthConfigs map[string]AuthConfig `json:"requested_auth_configs,omitempty"`
}

// Event represents a single event in the conversation between agents and users.
type Event struct {
	ID                 string         `json:"id"`
	InvocationID       string         `json:"invocation_id"`
	Author             string         `json:"author"` // 'user' or agent name
	Content            *Content       `json:"content,omitempty"`
	Actions            EventActions   `json:"actions"`
	Branch             *string        `json:"branch,omitempty"`
	Timestamp          time.Time      `json:"timestamp"`
	LongRunningToolIDs []string       `json:"long_running_tool_ids,omitempty"`
	Partial            *bool          `json:"partial,omitempty"`
	TurnComplete       *bool          `json:"turn_complete,omitempty"`
	ErrorCode          *string        `json:"error_code,omitempty"`
	ErrorMessage       *string        `json:"error_message,omitempty"`
	Interrupted        *bool          `json:"interrupted,omitempty"`
	CustomMetadata     map[string]any `json:"custom_metadata,omitempty"`
}

// NewEvent creates a new event with a generated ID and current timestamp.
func NewEvent(invocationID, author string) *Event {
	return &Event{
		ID:           generateEventID(),
		InvocationID: invocationID,
		Author:       author,
		Timestamp:    time.Now(),
		Actions:      EventActions{},
	}
}

// generateEventID creates a unique identifier for an event.
func generateEventID() string {
	// Simple implementation - in production, use UUID or similar
	return "evt_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

// randomString generates a random string of specified length.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// GetFunctionCalls returns all function calls in the event content.
func (e *Event) GetFunctionCalls() []*FunctionCall {
	log.Println("Getting function calls from event...")
	if e.Content == nil {
		return nil
	}

	var calls []*FunctionCall
	for _, part := range e.Content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, part.FunctionCall)
		}
	}
	return calls
}

// GetFunctionResponses returns all function responses in the event content.
func (e *Event) GetFunctionResponses() []*FunctionResponse {
	log.Println("Getting function responses from event...")
	if e.Content == nil {
		return nil
	}

	var responses []*FunctionResponse
	for _, part := range e.Content.Parts {
		if part.FunctionResponse != nil {
			responses = append(responses, part.FunctionResponse)
		}
	}
	return responses
}

// IsFinalResponse determines if this event represents a final response.
func (e *Event) IsFinalResponse() bool {
	log.Println("Checking if event is a final response...")
	if e.Actions.SkipSummarization != nil && *e.Actions.SkipSummarization {
		return true
	}
	if len(e.LongRunningToolIDs) > 0 {
		return true
	}
	if len(e.GetFunctionCalls()) > 0 || len(e.GetFunctionResponses()) > 0 {
		return false
	}
	if e.Partial != nil && *e.Partial {
		return false
	}
	return true
}

// State represents session state with different scopes.
type State struct {
	data map[string]any
}

// NewState creates a new empty state.
func NewState() *State {
	return &State{
		data: make(map[string]any),
	}
}

// Get retrieves a value from state by key.
func (s *State) Get(key string) (any, bool) {
	val, exists := s.data[key]
	return val, exists
}

// Set stores a value in state by key.
func (s *State) Set(key string, value any) {
	s.data[key] = value
}

// Update applies a delta to the state.
func (s *State) Update(delta map[string]any) {
	for k, v := range delta {
		s.data[k] = v
	}
}

// ToMap returns the state as a map.
func (s *State) ToMap() map[string]any {
	result := make(map[string]any)
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// HasDelta checks if there are any pending changes.
func (s *State) HasDelta() bool {
	return len(s.data) > 0
}

// MarshalJSON implements json.Marshaler.
func (s *State) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *State) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &s.data)
}
