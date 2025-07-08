package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientConfig holds configuration for the A2A client
type ClientConfig struct {
	// Timeout for HTTP requests
	Timeout time.Duration
	// Custom HTTP client (optional)
	HTTPClient *http.Client
	// Base URL for the A2A server
	BaseURL string
	// Additional headers to include in requests
	Headers map[string]string
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout: 600 * time.Second, // 10 minutes default
		Headers: make(map[string]string),
	}
}

// Client is an A2A client for communicating with remote agents
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
	agentCard  *AgentCard
	baseURL    string
}

// NewClient creates a new A2A client
func NewClient(agentCard *AgentCard, config *ClientConfig) (*Client, error) {
	if agentCard == nil {
		return nil, fmt.Errorf("agent card cannot be nil")
	}

	if config == nil {
		config = DefaultClientConfig()
	}

	// Use custom HTTP client if provided, otherwise create one
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.Timeout,
		}
	}

	// Use URL from agent card if not overridden
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = agentCard.URL
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		agentCard:  agentCard,
		baseURL:    baseURL,
	}, nil
}

// SendMessage sends a message to the remote agent and returns the response
func (c *Client) SendMessage(ctx context.Context, params *TaskSendParams) (*Task, error) {
	request := &SendTaskRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tasks/send",
		Params:  *params,
	}

	var response SendTaskResponse
	if err := c.sendJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("A2A error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// SendMessageStream sends a message and subscribes to streaming updates
func (c *Client) SendMessageStream(ctx context.Context, params *TaskSendParams, eventHandler func(*SendTaskStreamingResponse) error) error {
	request := &SendTaskStreamingRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tasks/sendSubscribe",
		Params:  *params,
	}

	return c.sendStreamingRequest(ctx, request, eventHandler)
}

// GetTask retrieves task details by ID
func (c *Client) GetTask(ctx context.Context, params *TaskQueryParams) (*Task, error) {
	request := &GetTaskRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tasks/get",
		Params:  *params,
	}

	var response GetTaskResponse
	if err := c.sendJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("A2A error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// CancelTask cancels a task by ID
func (c *Client) CancelTask(ctx context.Context, params *TaskIdParams) (*Task, error) {
	request := &CancelTaskRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tasks/cancel",
		Params:  *params,
	}

	var response CancelTaskResponse
	if err := c.sendJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("failed to cancel task: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("A2A error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// SetTaskPushNotification sets push notification config for a task
func (c *Client) SetTaskPushNotification(ctx context.Context, config *TaskPushNotificationConfig) (*TaskPushNotificationConfig, error) {
	request := &SetTaskPushNotificationRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tasks/pushNotification/set",
		Params:  *config,
	}

	var response SetTaskPushNotificationResponse
	if err := c.sendJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("failed to set push notification: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("A2A error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// GetTaskPushNotification gets push notification config for a task
func (c *Client) GetTaskPushNotification(ctx context.Context, params *TaskIdParams) (*TaskPushNotificationConfig, error) {
	request := &GetTaskPushNotificationRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "tasks/pushNotification/get",
		Params:  *params,
	}

	var response GetTaskPushNotificationResponse
	if err := c.sendJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, fmt.Errorf("failed to get push notification: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("A2A error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// sendJSONRPCRequest sends a JSON-RPC request and unmarshals the response
func (c *Client) sendJSONRPCRequest(ctx context.Context, request any, response any) error {
	// Marshal request to JSON
	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(body))
	}

	// Read and unmarshal response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// sendStreamingRequest sends a streaming request and processes SSE events
func (c *Client) sendStreamingRequest(ctx context.Context, request any, eventHandler func(*SendTaskStreamingResponse) error) error {
	// Marshal request to JSON
	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers for SSE
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(body))
	}

	// Check content type
	contentType := httpResp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return fmt.Errorf("expected text/event-stream, got %s", contentType)
	}

	// Process SSE stream
	return c.processSSEStream(ctx, httpResp.Body, eventHandler)
}

// processSSEStream processes Server-Sent Events from the response body
func (c *Client) processSSEStream(ctx context.Context, body io.Reader, eventHandler func(*SendTaskStreamingResponse) error) error {
	buf := make([]byte, 4096)
	var dataBuffer strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := body.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read SSE stream: %w", err)
		}

		chunk := string(buf[:n])
		lines := strings.Split(chunk, "\n")

		for i, line := range lines {
			line = strings.TrimSpace(line)

			// Handle incomplete lines
			if i == len(lines)-1 && !strings.HasSuffix(chunk, "\n") {
				dataBuffer.WriteString(line)
				continue
			}

			// Add any buffered data
			if dataBuffer.Len() > 0 {
				line = dataBuffer.String() + line
				dataBuffer.Reset()
			}

			// Process SSE line
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "" {
					continue
				}

				// Parse JSON-RPC response
				var response SendTaskStreamingResponse
				if err := json.Unmarshal([]byte(data), &response); err != nil {
					slog.Warn("Failed to parse SSE data", "data", data, "error", err)
					continue
				}

				// Handle the event
				if err := eventHandler(&response); err != nil {
					return fmt.Errorf("event handler error: %w", err)
				}
			}
		}
	}

	return nil
}

// Close closes the client and cleans up resources
func (c *Client) Close() error {
	// HTTP client doesn't need explicit closing in standard lib
	// Custom transports can be closed if needed by the caller
	return nil
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// AgentCardResolver helps resolve agent cards from URLs
type AgentCardResolver struct {
	httpClient *http.Client
	baseURL    string
}

// NewAgentCardResolver creates a new agent card resolver
func NewAgentCardResolver(baseURL string, httpClient *http.Client) *AgentCardResolver {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &AgentCardResolver{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// GetAgentCard fetches an agent card from a relative path
func (r *AgentCardResolver) GetAgentCard(ctx context.Context, relativePath string) (*AgentCard, error) {
	// Construct full URL
	fullURL, err := url.JoinPath(r.baseURL, relativePath)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var agentCard AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&agentCard); err != nil {
		return nil, fmt.Errorf("failed to decode agent card: %w", err)
	}

	return &agentCard, nil
}

// GetWellKnownAgentCard fetches the agent card from /.well-known/agent.json
func (r *AgentCardResolver) GetWellKnownAgentCard(ctx context.Context) (*AgentCard, error) {
	return r.GetAgentCard(ctx, "/.well-known/agent.json")
}
