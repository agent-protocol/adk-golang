// Package core defines request/response types and context structures for the ADK framework.
package core

import (
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
