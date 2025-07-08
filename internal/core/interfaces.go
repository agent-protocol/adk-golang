// Package core defines the core interfaces for the ADK framework.
package core

import (
	"context"
)

// EventStream represents a stream of events from agent execution.
type EventStream <-chan *Event

// BeforeAgentCallback is called before an agent starts executing.
type BeforeAgentCallback func(ctx context.Context, invocationCtx *InvocationContext) error

// AfterAgentCallback is called after an agent finishes executing.
type AfterAgentCallback func(ctx context.Context, invocationCtx *InvocationContext, events []*Event) error

// BaseAgent defines the interface that all agents must implement.
type BaseAgent interface {
	// Name returns the agent's unique identifier.
	Name() string

	// Description returns a description of the agent's purpose.
	Description() string

	// Instruction returns optional system instructions for the agent.
	Instruction() string

	// SubAgents returns the list of child agents in the hierarchy.
	SubAgents() []BaseAgent

	// ParentAgent returns the parent agent, if any.
	ParentAgent() BaseAgent

	// SetParentAgent sets the parent agent.
	SetParentAgent(parent BaseAgent)

	// RunAsync executes the agent with the given context and returns an event stream.
	// This is the main entry point for agent execution.
	RunAsync(ctx context.Context, invocationCtx *InvocationContext) (EventStream, error)

	// Run is a synchronous wrapper around RunAsync that collects all events.
	Run(ctx context.Context, invocationCtx *InvocationContext) ([]*Event, error)

	// FindAgent searches for an agent by name in the hierarchy.
	// Returns nil if not found.
	FindAgent(name string) BaseAgent

	// FindSubAgent searches for a direct sub-agent by name.
	// Returns nil if not found.
	FindSubAgent(name string) BaseAgent

	// GetBeforeAgentCallback returns the before-agent callback.
	GetBeforeAgentCallback() BeforeAgentCallback

	// SetBeforeAgentCallback sets the before-agent callback.
	SetBeforeAgentCallback(callback BeforeAgentCallback)

	// GetAfterAgentCallback returns the after-agent callback.
	GetAfterAgentCallback() AfterAgentCallback

	// SetAfterAgentCallback sets the after-agent callback.
	SetAfterAgentCallback(callback AfterAgentCallback)

	// Cleanup performs any necessary cleanup operations.
	Cleanup(ctx context.Context) error
}

// BaseTool defines the interface that all tools must implement.
type BaseTool interface {
	// Name returns the tool's unique identifier.
	Name() string

	// Description returns a description of the tool's purpose.
	Description() string

	// IsLongRunning indicates if this is a long-running operation.
	IsLongRunning() bool

	// GetDeclaration returns the function declaration for LLM integration.
	// Returns nil if the tool doesn't support LLM function calling.
	GetDeclaration() *FunctionDeclaration

	// RunAsync executes the tool with the given arguments and context.
	RunAsync(ctx context.Context, args map[string]any, toolCtx *ToolContext) (any, error)

	// ProcessLLMRequest allows the tool to modify LLM requests.
	// This is used for built-in tools that need to be added to the LLM config.
	ProcessLLMRequest(ctx context.Context, toolCtx *ToolContext, request *LLMRequest) error
}

// BaseToolset defines the interface for toolsets that provide multiple tools.
type BaseToolset interface {
	// GetTools returns all tools in the toolset based on the provided context.
	GetTools(ctx context.Context, readonlyCtx *ReadonlyContext) ([]BaseTool, error)

	// Close performs any necessary cleanup operations.
	Close(ctx context.Context) error
}

// SessionService defines the interface for session management.
type SessionService interface {
	// CreateSession creates a new session.
	CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error)

	// GetSession retrieves a session by ID.
	GetSession(ctx context.Context, req *GetSessionRequest) (*Session, error)

	// AppendEvent adds an event to a session.
	AppendEvent(ctx context.Context, session *Session, event *Event) error

	// DeleteSession removes a session.
	DeleteSession(ctx context.Context, req *DeleteSessionRequest) error

	// ListSessions returns sessions for a user.
	ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error)
}

// ArtifactService defines the interface for artifact management.
type ArtifactService interface {
	// SaveArtifact stores an artifact and returns its version.
	SaveArtifact(ctx context.Context, req *SaveArtifactRequest) (int, error)

	// LoadArtifact retrieves an artifact by filename and optional version.
	LoadArtifact(ctx context.Context, req *LoadArtifactRequest) ([]byte, error)

	// ListArtifactKeys returns all artifact filenames for a session.
	ListArtifactKeys(ctx context.Context, req *ListArtifactKeysRequest) ([]string, error)

	// DeleteArtifact removes an artifact.
	DeleteArtifact(ctx context.Context, req *DeleteArtifactRequest) error

	// ListVersions returns all versions of an artifact.
	ListVersions(ctx context.Context, req *ListVersionsRequest) ([]int, error)
}

// MemoryService defines the interface for long-term memory across sessions.
type MemoryService interface {
	// AddSessionToMemory stores session information for future retrieval.
	AddSessionToMemory(ctx context.Context, session *Session) error

	// RetrieveRelevantEvents searches for relevant events based on a query.
	RetrieveRelevantEvents(ctx context.Context, req *RetrieveMemoryRequest) ([]*Event, error)
}

// CredentialService defines the interface for credential management.
type CredentialService interface {
	// GetCredential retrieves a credential by ID.
	GetCredential(ctx context.Context, credentialID string) (*Credential, error)

	// StoreCredential saves a credential.
	StoreCredential(ctx context.Context, credential *Credential) error

	// DeleteCredential removes a credential.
	DeleteCredential(ctx context.Context, credentialID string) error
}

// Runner orchestrates agent execution and manages the overall workflow.
type Runner interface {
	// RunAsync executes an agent with the given input and returns an event stream.
	RunAsync(ctx context.Context, req *RunRequest) (EventStream, error)

	// Run is a synchronous wrapper around RunAsync that collects all events.
	Run(ctx context.Context, req *RunRequest) ([]*Event, error)

	// Close performs cleanup operations.
	Close(ctx context.Context) error
}

// LLMConnection defines the interface for LLM integrations.
type LLMConnection interface {
	// GenerateContent sends a request to the LLM and returns the response.
	GenerateContent(ctx context.Context, request *LLMRequest) (*LLMResponse, error)

	// GenerateContentStream sends a request and returns a streaming response.
	GenerateContentStream(ctx context.Context, request *LLMRequest) (<-chan *LLMResponse, error)

	// Close closes the connection.
	Close(ctx context.Context) error
}
