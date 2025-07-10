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

// TaskWaitingStrategy defines how to handle task completion waiting
type TaskWaitingStrategy int

const (
	// TaskWaitingNone - Don't wait for task completion, return immediately
	TaskWaitingNone TaskWaitingStrategy = iota
	// TaskWaitingPoll - Poll for task completion using GetTask
	TaskWaitingPoll
	// TaskWaitingStream - Use streaming if supported by agent
	TaskWaitingStream
	// TaskWaitingAuto - Automatically choose based on agent capabilities
	TaskWaitingAuto
)

// RemoteA2aAgentConfig holds configuration for RemoteA2aAgent
type RemoteA2aAgentConfig struct {
	// HTTP client timeout
	Timeout time.Duration
	// Custom HTTP client (optional)
	HTTPClient *http.Client
	// Additional headers for A2A requests
	Headers map[string]string

	// Task waiting configuration
	TaskWaitingStrategy TaskWaitingStrategy
	TaskPollingInterval time.Duration
	TaskPollingTimeout  time.Duration
	MaxTaskPollingTries int

	// Streaming configuration
	ForceStreaming      bool // Force streaming even if agent doesn't advertise it
	StreamingTimeout    time.Duration
	StreamingBufferSize int

	// Retry configuration
	MaxRetries      int
	RetryBackoff    time.Duration
	RetryableErrors []string // Error messages that should trigger retry

	// Legacy fields for backward compatibility
	TaskPollingEnabled bool // Whether to poll for task completion (now controlled by TaskWaitingStrategy)
	PreferStreaming    bool // Prefer streaming over polling when agent supports it (now controlled by TaskWaitingStrategy)
}

// DefaultRemoteA2aAgentConfig returns default configuration
func DefaultRemoteA2aAgentConfig() *RemoteA2aAgentConfig {
	return &RemoteA2aAgentConfig{
		Timeout: 600 * time.Second, // 10 minutes
		Headers: make(map[string]string),

		TaskWaitingStrategy: TaskWaitingAuto,
		TaskPollingInterval: 2 * time.Second,
		TaskPollingTimeout:  300 * time.Second, // 5 minutes
		MaxTaskPollingTries: 150,               // 5 minutes / 2 seconds

		ForceStreaming:      false,
		StreamingTimeout:    600 * time.Second, // 10 minutes
		StreamingBufferSize: 100,

		MaxRetries:      3,
		RetryBackoff:    1 * time.Second,
		RetryableErrors: []string{"timeout", "connection refused", "temporary failure"},

		// Legacy compatibility
		TaskPollingEnabled: true,
		PreferStreaming:    true,
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

// shouldUseStreaming determines if streaming should be used based on agent capabilities and config
func (r *RemoteA2aAgent) shouldUseStreaming() bool {
	// Force streaming if configured
	if r.config.ForceStreaming {
		return true
	}

	// Check if agent card is resolved and supports streaming
	agentCard := r.GetAgentCard()
	if agentCard == nil {
		return false
	}

	// Check agent capabilities for streaming support
	return agentCard.Capabilities.Streaming
}

// determineTaskWaitingStrategy determines the actual strategy to use
func (r *RemoteA2aAgent) determineTaskWaitingStrategy() TaskWaitingStrategy {
	switch r.config.TaskWaitingStrategy {
	case TaskWaitingAuto:
		if r.shouldUseStreaming() {
			return TaskWaitingStream
		}
		return TaskWaitingPoll
	default:
		return r.config.TaskWaitingStrategy
	}
}

// RunAsync executes the agent with enhanced task waiting capabilities
func (r *RemoteA2aAgent) RunAsync(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	// Ensure agent is resolved
	if err := r.ensureResolved(invocationCtx); err != nil {
		return nil, err
	}

	strategy := r.determineTaskWaitingStrategy()

	switch strategy {
	case TaskWaitingStream:
		return r.runWithStreaming(invocationCtx)
	case TaskWaitingPoll:
		return r.runWithPolling(invocationCtx)
	case TaskWaitingNone:
		return r.runWithMessageSend(invocationCtx)
	default:
		return nil, fmt.Errorf("unsupported task waiting strategy: %v", strategy)
	}
}

// runWithStreaming executes the agent using streaming
func (r *RemoteA2aAgent) runWithStreaming(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, r.config.StreamingBufferSize)

	go func() {
		defer close(eventChan)

		// Check for cancellation
		select {
		case <-invocationCtx.Done():
			return
		default:
		}

		// Convert session events to A2A message
		message, err := r.constructA2AMessageFromSession(invocationCtx)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Error constructing A2A message: %v", err))
			return
		}

		// Prepare streaming request
		messageSendParams := &a2a.MessageSendParams{
			Message: *message,
			Configuration: &a2a.MessageSendConfiguration{
				AcceptedOutputModes: []string{"text"},
			},
		}

		// Use streaming timeout context
		streamCtx, cancel := context.WithTimeout(invocationCtx, r.config.StreamingTimeout)
		defer cancel()

		// Handle streaming events
		eventHandler := func(response *a2a.SendStreamingMessageResponse) error {
			select {
			case <-streamCtx.Done():
				return streamCtx.Err()
			default:
			}

			event, err := r.convertStreamingResponseToEvent(response, invocationCtx)
			if err != nil {
				return fmt.Errorf("failed to convert streaming response: %w", err)
			}

			select {
			case eventChan <- event:
			case <-streamCtx.Done():
				return streamCtx.Err()
			}

			// Check if this is the final event
			if response.Final != nil && *response.Final {
				return nil // Stop streaming
			}

			return nil
		}

		// Send streaming message
		if err := r.a2aClient.SendMessageStream(streamCtx, messageSendParams, eventHandler); err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Streaming failed: %v", err))
		}
	}()

	return eventChan, nil
}

// runWithPolling executes the agent using task polling
func (r *RemoteA2aAgent) runWithPolling(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		// Send initial message and get task
		task, err := r.sendInitialMessage(invocationCtx)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Failed to send initial message: %v", err))
			return
		}

		// Start task polling
		finalTask, err := r.pollForTaskCompletion(invocationCtx, task.ID)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Task polling failed: %v", err))
			return
		}

		// Convert final task to event
		event, err := r.convertA2ATaskToEvent(finalTask, invocationCtx)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Failed to convert final task: %v", err))
			return
		}

		select {
		case eventChan <- event:
		case <-invocationCtx.Done():
		}
	}()

	return eventChan, nil
}

// sendInitialMessage sends the initial message and returns the task
func (r *RemoteA2aAgent) sendInitialMessage(invocationCtx *core.InvocationContext) (*a2a.Task, error) {
	message, err := r.constructA2AMessageFromSession(invocationCtx)
	if err != nil {
		return nil, fmt.Errorf("error constructing message: %w", err)
	}

	messageSendParams := &a2a.MessageSendParams{
		Message: *message,
		Configuration: &a2a.MessageSendConfiguration{
			AcceptedOutputModes: []string{"text"},
		},
	}

	return r.a2aClient.SendMessage(invocationCtx, messageSendParams)
}

// runWithMessageSend executes using standard message sending without polling
func (r *RemoteA2aAgent) runWithMessageSend(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 1)

	go func() {
		defer close(eventChan)

		// Convert session events to A2A message
		message, err := r.constructA2AMessageFromSession(invocationCtx)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Error constructing A2A message: %v", err))
			return
		}

		// Send message to remote agent
		messageSendParams := &a2a.MessageSendParams{
			Message: *message,
			Configuration: &a2a.MessageSendConfiguration{
				AcceptedOutputModes: []string{"text"},
			},
		}

		task, err := r.a2aClient.SendMessage(invocationCtx, messageSendParams)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Error sending message to remote agent: %v", err))
			return
		}

		// Convert task to event
		event, err := r.convertA2ATaskToEvent(task, invocationCtx)
		if err != nil {
			r.sendErrorEvent(eventChan, invocationCtx, fmt.Sprintf("Error converting A2A response: %v", err))
			return
		}

		select {
		case eventChan <- event:
		case <-invocationCtx.Done():
		}
	}()

	return eventChan, nil
}

// pollForTaskCompletion polls for task completion with exponential backoff
func (r *RemoteA2aAgent) pollForTaskCompletion(ctx context.Context, taskID string) (*a2a.Task, error) {
	pollCtx, cancel := context.WithTimeout(ctx, r.config.TaskPollingTimeout)
	defer cancel()

	ticker := time.NewTicker(r.config.TaskPollingInterval)
	defer ticker.Stop()

	tries := 0
	for {
		select {
		case <-pollCtx.Done():
			return nil, fmt.Errorf("task polling timeout after %v", r.config.TaskPollingTimeout)
		case <-ticker.C:
			tries++
			if tries > r.config.MaxTaskPollingTries {
				return nil, fmt.Errorf("exceeded maximum polling tries (%d)", r.config.MaxTaskPollingTries)
			}

			task, err := r.getTaskStatus(pollCtx, taskID)
			if err != nil {
				// Check if this is a retryable error
				if r.isRetryableError(err) && tries < r.config.MaxRetries {
					time.Sleep(r.config.RetryBackoff)
					continue
				}
				return nil, fmt.Errorf("failed to get task status: %w", err)
			}

			// Check if task is in terminal state
			if r.isTerminalTaskState(task.Status.State) {
				return task, nil
			}

			// Log task progress for debugging
			if task.Status.Message != nil && len(task.Status.Message.Parts) > 0 {
				if task.Status.Message.Parts[0].Text != nil {
					fmt.Printf("Task %s status: %s - %s\n", taskID, task.Status.State, *task.Status.Message.Parts[0].Text)
				}
			}
		}
	}
}

// getTaskStatus retrieves the current status of a task
func (r *RemoteA2aAgent) getTaskStatus(ctx context.Context, taskID string) (*a2a.Task, error) {
	params := &a2a.TaskQueryParams{
		ID: taskID,
	}
	return r.a2aClient.GetTask(ctx, params)
}

// isTerminalTaskState checks if a task state is terminal (final)
func (r *RemoteA2aAgent) isTerminalTaskState(state a2a.TaskState) bool {
	switch state {
	case a2a.TaskStateCompleted, a2a.TaskStateFailed, a2a.TaskStateCanceled:
		return true
	default:
		return false
	}
}

// isRetryableError checks if an error should trigger a retry
func (r *RemoteA2aAgent) isRetryableError(err error) bool {
	errStr := err.Error()
	for _, retryableErr := range r.config.RetryableErrors {
		if fmt.Sprintf("%v", errStr) == retryableErr {
			return true
		}
	}
	return false
}

// convertStreamingResponseToEvent converts a streaming response to an ADK event
func (r *RemoteA2aAgent) convertStreamingResponseToEvent(response *a2a.SendStreamingMessageResponse, invocationCtx *core.InvocationContext) (*core.Event, error) {
	event := core.NewEvent(invocationCtx.InvocationID, r.Name())

	// Handle different result types
	switch result := response.Result.(type) {
	case *a2a.Task:
		return r.convertA2ATaskToEvent(result, invocationCtx)
	case *a2a.TaskStatusUpdateEvent:
		return r.convertTaskStatusUpdateToEvent(result, invocationCtx)
	case *a2a.TaskArtifactUpdateEvent:
		return r.convertTaskArtifactUpdateToEvent(result, invocationCtx)
	default:
		// Handle as generic response
		event.Content = &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr(fmt.Sprintf("Streaming update: %v", result)),
				},
			},
		}
	}

	event.Actions = core.EventActions{}
	if invocationCtx.Branch != nil {
		event.Branch = invocationCtx.Branch
	}

	return event, nil
}

// convertTaskStatusUpdateToEvent converts a task status update to an ADK event
func (r *RemoteA2aAgent) convertTaskStatusUpdateToEvent(update *a2a.TaskStatusUpdateEvent, invocationCtx *core.InvocationContext) (*core.Event, error) {
	event := core.NewEvent(invocationCtx.InvocationID, r.Name())

	var content *core.Content
	if update.Status.Message != nil && len(update.Status.Message.Parts) > 0 {
		var parts []core.Part
		for _, part := range update.Status.Message.Parts {
			if part.Text != nil {
				parts = append(parts, core.Part{
					Type: "text",
					Text: part.Text,
				})
			}
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
					Text: ptr.Ptr(fmt.Sprintf("Task status: %s", update.Status.State)),
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

// convertTaskArtifactUpdateToEvent converts a task artifact update to an ADK event
func (r *RemoteA2aAgent) convertTaskArtifactUpdateToEvent(update *a2a.TaskArtifactUpdateEvent, invocationCtx *core.InvocationContext) (*core.Event, error) {
	event := core.NewEvent(invocationCtx.InvocationID, r.Name())

	var parts []core.Part
	for _, part := range update.Artifact.Parts {
		if part.Text != nil {
			parts = append(parts, core.Part{
				Type: "text",
				Text: part.Text,
			})
		}
		// TODO: Handle other part types (files, data, etc.)
	}

	if len(parts) == 0 {
		parts = []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("Artifact updated"),
			},
		}
	}

	event.Content = &core.Content{
		Role:  "agent",
		Parts: parts,
	}
	event.Actions = core.EventActions{}
	if invocationCtx.Branch != nil {
		event.Branch = invocationCtx.Branch
	}

	return event, nil
}

// sendErrorEvent sends an error event to the event channel
func (r *RemoteA2aAgent) sendErrorEvent(eventChan chan<- *core.Event, invocationCtx *core.InvocationContext, errorMsg string) {
	event := core.NewEvent(invocationCtx.InvocationID, r.Name())
	event.Content = &core.Content{
		Role: "agent",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr(errorMsg),
			},
		},
	}
	event.Actions = core.EventActions{}
	if invocationCtx.Branch != nil {
		event.Branch = invocationCtx.Branch
	}

	select {
	case eventChan <- event:
	case <-invocationCtx.Done():
	}
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

	// Create message with required messageId field
	message := &a2a.Message{
		MessageID: generateMessageID(), // Required by A2A spec
		Role:      "user",
		Parts:     parts,
	}

	return message, nil
}

// generateMessageID generates a unique message ID for A2A protocol
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
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

// EnsureResolved ensures the agent is resolved (public method for external use)
func (r *RemoteA2aAgent) EnsureResolved(ctx context.Context) error {
	return r.ensureResolved(ctx)
}

// GetConfig returns the configuration
func (r *RemoteA2aAgent) GetConfig() *RemoteA2aAgentConfig {
	return r.config
}

// SetTaskWaitingStrategy updates the task waiting strategy
func (r *RemoteA2aAgent) SetTaskWaitingStrategy(strategy TaskWaitingStrategy) {
	r.config.TaskWaitingStrategy = strategy
}

// SetTaskPollingTimeout updates the task polling timeout
func (r *RemoteA2aAgent) SetTaskPollingTimeout(timeout time.Duration) {
	r.config.TaskPollingTimeout = timeout
}

// SetTaskPollingInterval updates the task polling interval
func (r *RemoteA2aAgent) SetTaskPollingInterval(interval time.Duration) {
	r.config.TaskPollingInterval = interval
}
