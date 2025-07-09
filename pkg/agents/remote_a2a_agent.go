package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// AgentCardResolutionError is raised when agent card resolution fails
type AgentCardResolutionError struct {
	message string
	cause   error
}

func (e *AgentCardResolutionError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *AgentCardResolutionError) Unwrap() error {
	return e.cause
}

// A2AClientError is raised when A2A client operations fail
type A2AClientError struct {
	message string
	cause   error
}

func (e *A2AClientError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *A2AClientError) Unwrap() error {
	return e.cause
}

// RemoteA2aAgentConfig holds configuration for RemoteA2aAgent
type RemoteA2aAgentConfig struct {
	// HTTP client timeout
	Timeout time.Duration
	// Custom HTTP client (optional)
	HTTPClient *http.Client
	// Additional headers for A2A requests
	Headers map[string]string
}

// DefaultRemoteA2aAgentConfig returns default configuration
func DefaultRemoteA2aAgentConfig() *RemoteA2aAgentConfig {
	return &RemoteA2aAgentConfig{
		Timeout: 600 * time.Second, // 10 minutes
		Headers: make(map[string]string),
	}
}

// AgentCardSource represents different ways to specify an agent card
type AgentCardSource interface {
	// isAgentCardSource is a marker method
	isAgentCardSource()
}

// AgentCardDirect represents a direct agent card object
type AgentCardDirect struct {
	Card *a2a.AgentCard
}

func (AgentCardDirect) isAgentCardSource() {}

// AgentCardURL represents an agent card specified by URL
type AgentCardURL struct {
	URL string
}

func (AgentCardURL) isAgentCardSource() {}

// AgentCardFile represents an agent card specified by file path
type AgentCardFile struct {
	Path string
}

func (AgentCardFile) isAgentCardSource() {}

// RemoteA2aAgent represents an agent that communicates with a remote A2A agent
type RemoteA2aAgent struct {
	*CustomAgent

	// Configuration
	config *RemoteA2aAgentConfig

	// Agent card source and resolved card
	agentCardSource AgentCardSource
	agentCard       *a2a.AgentCard

	// A2A client for communication
	a2aClient *a2a.Client
	rpcURL    string

	// Resolution state
	isResolved bool
}

// NewRemoteA2aAgent creates a new remote A2A agent
func NewRemoteA2aAgent(name string, agentCardSource AgentCardSource, config *RemoteA2aAgentConfig) (*RemoteA2aAgent, error) {
	if name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	if agentCardSource == nil {
		return nil, fmt.Errorf("agent card source cannot be nil")
	}

	if config == nil {
		config = DefaultRemoteA2aAgentConfig()
	}

	return &RemoteA2aAgent{
		CustomAgent:     NewBaseAgent(name, ""),
		config:          config,
		agentCardSource: agentCardSource,
		isResolved:      false,
	}, nil
}

// NewRemoteA2aAgentFromCard creates a remote A2A agent from a direct agent card
func NewRemoteA2aAgentFromCard(name string, agentCard *a2a.AgentCard, config *RemoteA2aAgentConfig) (*RemoteA2aAgent, error) {
	if agentCard == nil {
		return nil, fmt.Errorf("agent card cannot be nil")
	}

	return NewRemoteA2aAgent(name, &AgentCardDirect{Card: agentCard}, config)
}

// NewRemoteA2aAgentFromURL creates a remote A2A agent from an agent card URL
func NewRemoteA2aAgentFromURL(name string, agentCardURL string, config *RemoteA2aAgentConfig) (*RemoteA2aAgent, error) {
	if agentCardURL == "" {
		return nil, fmt.Errorf("agent card URL cannot be empty")
	}

	return NewRemoteA2aAgent(name, &AgentCardURL{URL: agentCardURL}, config)
}

// NewRemoteA2aAgentFromFile creates a remote A2A agent from an agent card file
func NewRemoteA2aAgentFromFile(name string, agentCardPath string, config *RemoteA2aAgentConfig) (*RemoteA2aAgent, error) {
	if agentCardPath == "" {
		return nil, fmt.Errorf("agent card file path cannot be empty")
	}

	return NewRemoteA2aAgent(name, &AgentCardFile{Path: agentCardPath}, config)
}

// ensureResolved ensures the agent card is resolved and the A2A client is initialized
func (r *RemoteA2aAgent) ensureResolved(ctx context.Context) error {
	if r.isResolved {
		return nil
	}

	// Resolve agent card if needed
	if r.agentCard == nil {
		agentCard, err := r.resolveAgentCard(ctx)
		if err != nil {
			return &AgentCardResolutionError{
				message: fmt.Sprintf("failed to resolve agent card for %s", r.Name()),
				cause:   err,
			}
		}
		r.agentCard = agentCard
	}

	// Validate agent card
	if err := r.validateAgentCard(r.agentCard); err != nil {
		return &AgentCardResolutionError{
			message: fmt.Sprintf("invalid agent card for %s", r.Name()),
			cause:   err,
		}
	}

	// Set RPC URL
	r.rpcURL = r.agentCard.URL

	// Update description if empty
	if r.Description() == "" && r.agentCard.Description != nil {
		r.CustomAgent.description = *r.agentCard.Description
	}

	// Initialize A2A client
	if r.a2aClient == nil {
		clientConfig := &a2a.ClientConfig{
			Timeout:    r.config.Timeout,
			HTTPClient: r.config.HTTPClient,
			BaseURL:    r.rpcURL,
			Headers:    r.config.Headers,
		}

		client, err := a2a.NewClient(r.agentCard, clientConfig)
		if err != nil {
			return &AgentCardResolutionError{
				message: fmt.Sprintf("failed to create A2A client for %s", r.Name()),
				cause:   err,
			}
		}
		r.a2aClient = client
	}

	r.isResolved = true
	return nil
}

// resolveAgentCard resolves the agent card from the configured source
func (r *RemoteA2aAgent) resolveAgentCard(ctx context.Context) (*a2a.AgentCard, error) {
	switch source := r.agentCardSource.(type) {
	case *AgentCardDirect:
		return source.Card, nil

	case *AgentCardURL:
		return r.resolveAgentCardFromURL(ctx, source.URL)

	case *AgentCardFile:
		return r.resolveAgentCardFromFile(source.Path)

	default:
		return nil, fmt.Errorf("unsupported agent card source type: %T", source)
	}
}

// resolveAgentCardFromURL resolves an agent card from a URL
func (r *RemoteA2aAgent) resolveAgentCardFromURL(ctx context.Context, agentCardURL string) (*a2a.AgentCard, error) {
	// Parse URL
	parsedURL, err := url.Parse(agentCardURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %s", agentCardURL)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL format: %s", agentCardURL)
	}

	// Extract base URL and relative path
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	relativePath := parsedURL.Path

	// Create resolver
	resolver := a2a.NewAgentCardResolver(baseURL, r.config.HTTPClient)

	// Fetch agent card
	agentCard, err := resolver.GetAgentCard(ctx, relativePath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card from %s: %w", agentCardURL, err)
	}

	return agentCard, nil
}

// resolveAgentCardFromFile resolves an agent card from a file
func (r *RemoteA2aAgent) resolveAgentCardFromFile(filePath string) (*a2a.AgentCard, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("agent card file not found: %s", filePath)
	}

	// Check if it's actually a file
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// Read and parse file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent card file %s: %w", filePath, err)
	}

	var agentCard a2a.AgentCard
	if err := json.Unmarshal(data, &agentCard); err != nil {
		return nil, fmt.Errorf("invalid JSON in agent card file %s: %w", filePath, err)
	}

	return &agentCard, nil
}

// validateAgentCard validates a resolved agent card
func (r *RemoteA2aAgent) validateAgentCard(agentCard *a2a.AgentCard) error {
	if agentCard.URL == "" {
		return fmt.Errorf("agent card must have a valid URL for RPC communication")
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(agentCard.URL)
	if err != nil {
		return fmt.Errorf("invalid RPC URL in agent card: %s", agentCard.URL)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid RPC URL format in agent card: %s", agentCard.URL)
	}

	return nil
}

// RunAsync executes the agent with the given context
func (r *RemoteA2aAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	// Ensure agent is resolved
	if err := r.ensureResolved(ctx); err != nil {
		return nil, err
	}

	// Create a channel for events
	eventChan := make(chan *core.Event, 1)

	// Start a goroutine to handle the A2A request
	go func() {
		defer close(eventChan)

		// Convert session events to A2A message
		message, err := r.constructA2AMessageFromSession(invocationCtx)
		if err != nil {
			event := core.NewEvent(invocationCtx.InvocationID, r.Name())
			event.Content = &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr(fmt.Sprintf("Error constructing A2A message: %v", err)),
					},
				},
			}
			event.Actions = core.EventActions{}
			if invocationCtx.Branch != nil {
				event.Branch = invocationCtx.Branch
			}

			eventChan <- event
			return
		}

		// Send message to remote agent
		taskParams := &a2a.TaskSendParams{
			ID:      generateTaskID(),
			Message: *message,
		}

		task, err := r.a2aClient.SendMessage(ctx, taskParams)
		if err != nil {
			event := core.NewEvent(invocationCtx.InvocationID, r.Name())
			event.Content = &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr(fmt.Sprintf("Error sending message to remote agent: %v", err)),
					},
				},
			}
			event.Actions = core.EventActions{}
			if invocationCtx.Branch != nil {
				event.Branch = invocationCtx.Branch
			}

			eventChan <- event
			return
		}

		// Convert A2A task to event
		event, err := r.convertA2ATaskToEvent(task, invocationCtx)
		if err != nil {
			event = core.NewEvent(invocationCtx.InvocationID, r.Name())
			event.Content = &core.Content{
				Role: "agent",
				Parts: []core.Part{
					{
						Type: "text",
						Text: ptr.Ptr(fmt.Sprintf("Error converting A2A response: %v", err)),
					},
				},
			}
			event.Actions = core.EventActions{}
			if invocationCtx.Branch != nil {
				event.Branch = invocationCtx.Branch
			}
		}

		eventChan <- event
	}()

	return eventChan, nil
}

// constructA2AMessageFromSession constructs an A2A message from the session context
func (r *RemoteA2aAgent) constructA2AMessageFromSession(invocationCtx *core.InvocationContext) (*a2a.Message, error) {
	if invocationCtx.UserContent == nil {
		return nil, fmt.Errorf("no user content available")
	}

	// Create parts from user content
	var parts []a2a.Part

	// Handle content parts
	for _, part := range invocationCtx.UserContent.Parts {
		if part.Text != nil {
			parts = append(parts, a2a.Part{
				Type: "text",
				Text: part.Text,
			})
		}
		// TODO: Handle other part types (function calls, files, etc.)
	}

	// Create message
	message := &a2a.Message{
		Role:  "user",
		Parts: parts,
	}

	return message, nil
}

// convertA2ATaskToEvent converts an A2A task response to an ADK event
func (r *RemoteA2aAgent) convertA2ATaskToEvent(task *a2a.Task, invocationCtx *core.InvocationContext) (*core.Event, error) {
	event := core.NewEvent(invocationCtx.InvocationID, r.Name())

	// Extract message from task status
	var content *core.Content
	if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
		// Convert A2A parts to ADK parts
		var parts []core.Part
		for _, part := range task.Status.Message.Parts {
			if part.Text != nil {
				parts = append(parts, core.Part{
					Type: "text",
					Text: part.Text,
				})
			}
			// TODO: Handle other part types
		}

		if len(parts) > 0 {
			content = &core.Content{
				Role:  "agent",
				Parts: parts,
			}
		}
	}

	if content == nil {
		content = &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr(fmt.Sprintf("Task %s completed with status: %s", task.ID, task.Status.State)),
				},
			},
		}
	}

	event.Content = content
	event.Actions = core.EventActions{}
	if invocationCtx.Branch != nil {
		event.Branch = invocationCtx.Branch
	}

	return event, nil
}

// Close cleans up resources
func (r *RemoteA2aAgent) Close() error {
	if r.a2aClient != nil {
		return r.a2aClient.Close()
	}
	return nil
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// GetAgentCard returns the resolved agent card (if available)
func (r *RemoteA2aAgent) GetAgentCard() *a2a.AgentCard {
	return r.agentCard
}

// GetRPCURL returns the RPC URL (if resolved)
func (r *RemoteA2aAgent) GetRPCURL() string {
	return r.rpcURL
}

// IsResolved returns whether the agent has been resolved
func (r *RemoteA2aAgent) IsResolved() bool {
	return r.isResolved
}
