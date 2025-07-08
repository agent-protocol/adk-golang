// Package runners provides orchestration implementations for agent execution.
// The Runner orchestrates agent execution with session management and artifact services,
// providing async event streaming similar to Python's AsyncGenerator pattern.
package runners

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// RunnerImpl implements the Runner interface.
// It orchestrates agent execution, manages sessions, and provides real-time event streaming.
type RunnerImpl struct {
	appName           string
	agent             core.BaseAgent
	sessionService    core.SessionService
	artifactService   core.ArtifactService
	memoryService     core.MemoryService
	credentialService core.CredentialService

	// Configuration options
	config *RunnerConfig

	// Synchronization
	mu sync.RWMutex
}

// RunnerConfig contains configuration options for the Runner.
type RunnerConfig struct {
	// EventBufferSize controls the size of the event channel buffer.
	// Larger buffers can handle bursts better but use more memory.
	EventBufferSize int

	// EnableEventProcessing enables automatic processing of event actions
	// such as state changes and artifact management.
	EnableEventProcessing bool

	// MaxConcurrentSessions limits the number of concurrent sessions
	// that can be processed simultaneously (0 = unlimited).
	MaxConcurrentSessions int

	// DefaultTimeout is the default timeout for operations.
	DefaultTimeout time.Duration
}

// DefaultRunnerConfig returns default configuration for a Runner.
func DefaultRunnerConfig() *RunnerConfig {
	return &RunnerConfig{
		EventBufferSize:       100,
		EnableEventProcessing: true,
		MaxConcurrentSessions: 0, // unlimited
		DefaultTimeout:        30 * time.Second,
	}
}

// NewRunner creates a new runner implementation with default configuration.
func NewRunner(
	appName string,
	agent core.BaseAgent,
	sessionService core.SessionService,
) *RunnerImpl {
	return NewRunnerWithConfig(appName, agent, sessionService, DefaultRunnerConfig())
}

// NewRunnerWithConfig creates a new runner implementation with custom configuration.
func NewRunnerWithConfig(
	appName string,
	agent core.BaseAgent,
	sessionService core.SessionService,
	config *RunnerConfig,
) *RunnerImpl {
	if config == nil {
		config = DefaultRunnerConfig()
	}

	return &RunnerImpl{
		appName:        appName,
		agent:          agent,
		sessionService: sessionService,
		config:         config,
	}
}

// SetArtifactService sets the artifact service.
func (r *RunnerImpl) SetArtifactService(service core.ArtifactService) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.artifactService = service
}

// SetMemoryService sets the memory service.
func (r *RunnerImpl) SetMemoryService(service core.MemoryService) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memoryService = service
}

// SetCredentialService sets the credential service.
func (r *RunnerImpl) SetCredentialService(service core.CredentialService) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.credentialService = service
}

// GetConfig returns the current runner configuration.
func (r *RunnerImpl) GetConfig() *RunnerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modifications
	configCopy := *r.config
	return &configCopy
}

// RunAsync executes an agent with the given input and returns an event stream.
// This provides real-time streaming of events similar to Python's AsyncGenerator pattern.
func (r *RunnerImpl) RunAsync(ctx context.Context, req *core.RunRequest) (core.EventStream, error) {
	r.mu.RLock()
	eventBufferSize := r.config.EventBufferSize
	enableEventProcessing := r.config.EnableEventProcessing
	r.mu.RUnlock()

	// Get or create session
	session, err := r.getOrCreateSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Create invocation context
	invocationCtx := r.createInvocationContext(req, session)

	// Append new message to session if provided
	if req.NewMessage != nil {
		if err := r.appendNewMessageToSession(ctx, session, req.NewMessage, invocationCtx); err != nil {
			return nil, fmt.Errorf("failed to append message to session: %w", err)
		}
	}

	// Determine which agent should handle the request
	agentToRun := r.findAgentToRun(session, r.agent)
	invocationCtx.Agent = agentToRun

	// Create event channel with configurable buffer size
	eventChan := make(chan *core.Event, eventBufferSize)

	// Start asynchronous processing
	go func() {
		defer close(eventChan)

		// Execute before-agent callback if present
		if callback := agentToRun.GetBeforeAgentCallback(); callback != nil {
			if err := callback(ctx, invocationCtx); err != nil {
				r.sendErrorEvent(eventChan, ctx, invocationCtx, agentToRun.Name(),
					fmt.Sprintf("Before-agent callback failed: %v", err))
				return
			}
		}

		// Run the agent and get event stream
		agentStream, err := agentToRun.RunAsync(ctx, invocationCtx)
		if err != nil {
			r.sendErrorEvent(eventChan, ctx, invocationCtx, agentToRun.Name(),
				fmt.Sprintf("Agent execution failed: %v", err))
			return
		}

		// Collect events for after-agent callback
		var collectedEvents []*core.Event

		// Process events from agent stream
		for event := range agentStream {
			// Process event actions if enabled
			if enableEventProcessing {
				if err := r.processEventActions(ctx, session, event, invocationCtx); err != nil {
					// Log error but continue processing
					fmt.Printf("Failed to process event actions: %v\n", err)
				}
			}

			// Append event to session (if not partial)
			if event.Partial == nil || !*event.Partial {
				if appendErr := r.sessionService.AppendEvent(ctx, session, event); appendErr != nil {
					// Log error but continue processing
					fmt.Printf("Failed to append event to session: %v\n", appendErr)
				}
			}

			// Collect for callback
			collectedEvents = append(collectedEvents, event)

			// Forward event to stream
			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}
		}

		// Execute after-agent callback if present
		if callback := agentToRun.GetAfterAgentCallback(); callback != nil {
			if err := callback(ctx, invocationCtx, collectedEvents); err != nil {
				// Send error as final event but don't block the stream
				r.sendErrorEvent(eventChan, ctx, invocationCtx, agentToRun.Name(),
					fmt.Sprintf("After-agent callback failed: %v", err))
			}
		}
	}()

	return eventChan, nil
}

// Run is a synchronous wrapper around RunAsync that collects all events.
func (r *RunnerImpl) Run(ctx context.Context, req *core.RunRequest) ([]*core.Event, error) {
	eventStream, err := r.RunAsync(ctx, req)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
	}

	return events, nil
}

// Close performs cleanup operations.
func (r *RunnerImpl) Close(ctx context.Context) error {
	return r.agent.Cleanup(ctx)
}

// getOrCreateSession gets an existing session or creates a new one.
func (r *RunnerImpl) getOrCreateSession(ctx context.Context, req *core.RunRequest) (*core.Session, error) {
	// Try to get existing session
	getReq := &core.GetSessionRequest{
		AppName:   r.appName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Config: &core.GetSessionConfig{
			IncludeEvents: true,
		},
	}

	session, err := r.sessionService.GetSession(ctx, getReq)
	if err != nil {
		return nil, err
	}

	if session != nil {
		return session, nil
	}

	// Create new session
	createReq := &core.CreateSessionRequest{
		AppName:   r.appName,
		UserID:    req.UserID,
		SessionID: &req.SessionID,
		State:     make(map[string]any),
	}

	return r.sessionService.CreateSession(ctx, createReq)
}

// createInvocationContext creates a new invocation context.
func (r *RunnerImpl) createInvocationContext(req *core.RunRequest, session *core.Session) *core.InvocationContext {
	invocationID := generateInvocationID()

	ctx := core.NewInvocationContext(invocationID, r.agent, session, r.sessionService)
	ctx.ArtifactService = r.artifactService
	ctx.MemoryService = r.memoryService
	ctx.CredentialService = r.credentialService
	ctx.UserContent = req.NewMessage

	if req.RunConfig != nil {
		ctx.RunConfig = req.RunConfig
	}

	return ctx
}

// appendNewMessageToSession adds a new user message to the session.
func (r *RunnerImpl) appendNewMessageToSession(
	ctx context.Context,
	session *core.Session,
	newMessage *core.Content,
	invocationCtx *core.InvocationContext,
) error {
	userEvent := core.NewEvent(invocationCtx.InvocationID, "user")
	userEvent.Content = newMessage

	return r.sessionService.AppendEvent(ctx, session, userEvent)
}

// findAgentToRun determines which agent should handle the request.
func (r *RunnerImpl) findAgentToRun(session *core.Session, rootAgent core.BaseAgent) core.BaseAgent {
	// Simple implementation: check for transfer_to_agent in the last event
	if len(session.Events) > 0 {
		lastEvent := session.Events[len(session.Events)-1]
		if lastEvent.Actions.TransferToAgent != nil {
			if agent := rootAgent.FindAgent(*lastEvent.Actions.TransferToAgent); agent != nil {
				return agent
			}
		}

		// Check if the last event author is a known agent
		if lastEvent.Author != "user" {
			if agent := rootAgent.FindAgent(lastEvent.Author); agent != nil {
				return agent
			}
		}
	}

	// Default to root agent
	return rootAgent
}

// generateInvocationID creates a unique invocation identifier.
func generateInvocationID() string {
	return fmt.Sprintf("inv_%d", time.Now().UnixNano())
}

// stringPtr returns a pointer to a string literal.
func stringPtr(s string) *string {
	return &s
}

// sendErrorEvent sends an error event to the event channel.
func (r *RunnerImpl) sendErrorEvent(eventChan chan<- *core.Event, ctx context.Context,
	invocationCtx *core.InvocationContext, author, message string) {
	errorEvent := core.NewEvent(invocationCtx.InvocationID, author)
	errorEvent.ErrorMessage = stringPtr(message)

	select {
	case eventChan <- errorEvent:
	case <-ctx.Done():
	}
}

// processEventActions processes actions contained in an event.
// This includes applying state changes, managing artifacts, and handling agent transfers.
func (r *RunnerImpl) processEventActions(ctx context.Context, session *core.Session,
	event *core.Event, invocationCtx *core.InvocationContext) error {

	actions := event.Actions

	// Apply state changes
	if len(actions.StateDelta) > 0 {
		if err := r.applyStateDelta(ctx, session, actions.StateDelta); err != nil {
			return fmt.Errorf("failed to apply state delta: %w", err)
		}
	}

	// Handle artifact changes
	if len(actions.ArtifactDelta) > 0 && r.artifactService != nil {
		if err := r.processArtifactDelta(ctx, session, actions.ArtifactDelta); err != nil {
			return fmt.Errorf("failed to process artifact delta: %w", err)
		}
	}

	// Handle memory operations
	if r.memoryService != nil {
		// Add session to memory if this is a significant event
		if event.IsFinalResponse() || len(event.GetFunctionCalls()) > 0 {
			if err := r.memoryService.AddSessionToMemory(ctx, session); err != nil {
				// Log but don't fail - memory is auxiliary
				fmt.Printf("Failed to add session to memory: %v\n", err)
			}
		}
	}

	return nil
}

// applyStateDelta applies state changes to the session.
func (r *RunnerImpl) applyStateDelta(ctx context.Context, session *core.Session,
	stateDelta map[string]any) error {

	// Update session state
	for key, value := range stateDelta {
		session.State[key] = value
	}

	// Update last update time
	session.LastUpdateTime = time.Now()

	return nil
}

// processArtifactDelta processes artifact version updates.
func (r *RunnerImpl) processArtifactDelta(ctx context.Context, session *core.Session,
	artifactDelta map[string]int) error {

	// For each artifact in the delta, update its version tracking
	for filename, version := range artifactDelta {
		// Store the artifact version in session state for tracking
		versionKey := fmt.Sprintf("artifact_version_%s", filename)
		session.State[versionKey] = version
	}

	return nil
}
