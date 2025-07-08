// Package runners provides orchestration implementations for agent execution.
package runners

import (
	"context"
	"fmt"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// RunnerImpl implements the Runner interface.
type RunnerImpl struct {
	appName           string
	agent             core.BaseAgent
	sessionService    core.SessionService
	artifactService   core.ArtifactService
	memoryService     core.MemoryService
	credentialService core.CredentialService
}

// NewRunner creates a new runner implementation.
func NewRunner(
	appName string,
	agent core.BaseAgent,
	sessionService core.SessionService,
) *RunnerImpl {
	return &RunnerImpl{
		appName:        appName,
		agent:          agent,
		sessionService: sessionService,
	}
}

// SetArtifactService sets the artifact service.
func (r *RunnerImpl) SetArtifactService(service core.ArtifactService) {
	r.artifactService = service
}

// SetMemoryService sets the memory service.
func (r *RunnerImpl) SetMemoryService(service core.MemoryService) {
	r.memoryService = service
}

// SetCredentialService sets the credential service.
func (r *RunnerImpl) SetCredentialService(service core.CredentialService) {
	r.credentialService = service
}

// RunAsync executes an agent with the given input and returns an event stream.
func (r *RunnerImpl) RunAsync(ctx context.Context, req *core.RunRequest) (core.EventStream, error) {
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

	// Create event channel
	eventChan := make(chan *core.Event, 100)

	go func() {
		defer close(eventChan)

		// Run the agent
		agentStream, err := agentToRun.RunAsync(ctx, invocationCtx)
		if err != nil {
			// Send error event
			errorEvent := core.NewEvent(invocationCtx.InvocationID, agentToRun.Name())
			errorEvent.ErrorMessage = stringPtr(fmt.Sprintf("Agent execution failed: %v", err))

			select {
			case eventChan <- errorEvent:
			case <-ctx.Done():
				return
			}
			return
		}

		// Process events from agent
		for event := range agentStream {
			// Append event to session (if not partial)
			if event.Partial == nil || !*event.Partial {
				if appendErr := r.sessionService.AppendEvent(ctx, session, event); appendErr != nil {
					// Log error but continue processing
					fmt.Printf("Failed to append event to session: %v\n", appendErr)
				}
			}

			// Forward event to stream
			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
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
