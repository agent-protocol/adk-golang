package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

var _ core.LLMConnection = (*OllamaConnection)(nil)

// OllamaConnection implements the LLMConnection interface for Ollama.
type OllamaConnection struct {
	baseURL    string
	httpClient *http.Client
	model      string
	config     *OllamaConfig
}

// OllamaConfig contains configuration options for Ollama connections.
type OllamaConfig struct {
	BaseURL     string        `json:"base_url"`
	Model       string        `json:"model"`
	Temperature *float32      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float32      `json:"top_p,omitempty"`
	TopK        *int          `json:"top_k,omitempty"`
	Timeout     time.Duration `json:"timeout"`
	Stream      bool          `json:"stream"`
}

// DefaultOllamaConfig returns a default configuration for Ollama.
func DefaultOllamaConfig() *OllamaConfig {
	return &OllamaConfig{
		BaseURL:     "http://localhost:11434",
		Model:       "llama3.2",
		Temperature: ptr.Float32((0.7)),
		Timeout:     30 * time.Second,
		Stream:      false,
	}
}

// NewOllamaConnection creates a new Ollama connection with the given configuration.
func NewOllamaConnection(config *OllamaConfig) *OllamaConnection {
	if config == nil {
		config = DefaultOllamaConfig()
	}

	// Ensure BaseURL doesn't end with slash
	baseURL := strings.TrimSuffix(config.BaseURL, "/")

	return &OllamaConnection{
		baseURL: baseURL,
		model:   config.Model,
		config:  config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// GenerateContent sends a request to Ollama and returns the response.
func (c *OllamaConnection) GenerateContent(ctx context.Context, request *core.LLMRequest) (*core.LLMResponse, error) {
	// Convert ADK request to Ollama format
	ollamaReq, err := c.convertToOllamaRequest(request, false)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	// Make HTTP request
	resp, err := c.makeHTTPRequest(ctx, "/api/chat", ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var ollamaResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to ADK response
	return c.convertFromOllamaResponse(&ollamaResp), nil
}

// GenerateContentStream sends a request and returns a streaming response.
func (c *OllamaConnection) GenerateContentStream(ctx context.Context, request *core.LLMRequest) (<-chan *core.LLMResponse, error) {
	// Convert ADK request to Ollama format with streaming enabled
	ollamaReq, err := c.convertToOllamaRequest(request, true)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	// Make HTTP request
	resp, err := c.makeHTTPRequest(ctx, "/api/chat", ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	responseChan := make(chan *core.LLMResponse, 10)

	go func() {
		defer close(responseChan)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		var accumulatedContent strings.Builder

		for {
			var chunk ChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					break
				}
				// Send error through channel
				errorResp := &core.LLMResponse{
					Content: &core.Content{
						Role: "assistant",
						Parts: []core.Part{
							{
								Type: "text",
								Text: ptr.Ptr(fmt.Sprintf("Error: %v", err)),
							},
						},
					},
					Partial: ptr.Ptr(false),
				}
				select {
				case responseChan <- errorResp:
				case <-ctx.Done():
				}
				return
			}

			// Accumulate content
			if chunk.Message.Content != "" {
				accumulatedContent.WriteString(chunk.Message.Content)
			}

			// Convert and send partial response
			partialResp := c.convertFromOllamaResponse(&chunk)

			// Set accumulated content for consistent streaming
			if accumulatedContent.Len() > 0 {
				partialResp.Content = &core.Content{
					Role: "assistant",
					Parts: []core.Part{
						{
							Type: "text",
							Text: ptr.Ptr(accumulatedContent.String()),
						},
					},
				}
			}

			// Mark as partial unless it's the final chunk
			partialResp.Partial = ptr.Ptr(!chunk.Done)

			select {
			case responseChan <- partialResp:
			case <-ctx.Done():
				return
			}

			// Break if this is the final chunk
			if chunk.Done {
				break
			}
		}
	}()

	return responseChan, nil
}

// Close closes the connection (no-op for HTTP-based connections).
func (c *OllamaConnection) Close(ctx context.Context) error {
	return nil
}

// convertToOllamaRequest converts an ADK LLMRequest to Ollama format.
func (c *OllamaConnection) convertToOllamaRequest(request *core.LLMRequest, stream bool) (*ChatRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Create Ollama ChatRequest
	chatReq := &ChatRequest{
		Model:   c.model,
		Stream:  ptr.Ptr(stream),
		Options: make(map[string]any),
	}

	// Convert ADK Contents to Ollama Messages
	messages := make([]Message, 0, len(request.Contents))
	for _, content := range request.Contents {
		message := Message{
			Role: c.mapRole(content.Role),
		}

		// Process content parts
		var textParts []string
		var toolCalls []ToolCall
		var images []ImageData

		for _, part := range content.Parts {
			switch part.Type {
			case "text":
				if part.Text != nil {
					textParts = append(textParts, *part.Text)
				}
			case "function_call":
				if part.FunctionCall != nil {
					toolCall := ToolCall{
						Function: ToolCallFunction{
							Name:      part.FunctionCall.Name,
							Arguments: ToolCallFunctionArguments(part.FunctionCall.Args),
						},
					}
					toolCalls = append(toolCalls, toolCall)
				}
			case "function_response":
				if part.FunctionResponse != nil {
					// For function responses, we include them as text content
					responseText := fmt.Sprintf("Function %s returned: %v", part.FunctionResponse.Name, part.FunctionResponse.Response)
					textParts = append(textParts, responseText)
				}
			}
		}

		// Combine text parts
		if len(textParts) > 0 {
			message.Content = strings.Join(textParts, "\n")
		}

		// Add tool calls if any
		if len(toolCalls) > 0 {
			message.ToolCalls = toolCalls
		}

		// Add images if any
		if len(images) > 0 {
			message.Images = images
		}

		messages = append(messages, message)
	}

	chatReq.Messages = messages

	// Convert tools to Ollama format
	if len(request.Tools) > 0 {
		ollamaTools := make(Tools, 0, len(request.Tools))
		for _, tool := range request.Tools {
			ollamaTool := Tool{
				Type: "function",
				Function: ToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
				},
			}

			// Convert parameters if present
			if tool.Parameters != nil {
				ollamaTool.Function.Parameters.Type = "object"
				if props, hasProps := tool.Parameters["properties"].(map[string]interface{}); hasProps {
					ollamaTool.Function.Parameters.Properties = make(map[string]struct {
						Type        PropertyType `json:"type"`
						Items       any          `json:"items,omitempty"`
						Description string       `json:"description"`
						Enum        []any        `json:"enum,omitempty"`
					})

					for propName, propValue := range props {
						if propDetails, ok := propValue.(map[string]interface{}); ok {
							prop := struct {
								Type        PropertyType `json:"type"`
								Items       any          `json:"items,omitempty"`
								Description string       `json:"description"`
								Enum        []any        `json:"enum,omitempty"`
							}{}

							if typeVal, hasType := propDetails["type"].(string); hasType {
								prop.Type = PropertyType{typeVal}
							}
							if desc, hasDesc := propDetails["description"].(string); hasDesc {
								prop.Description = desc
							}
							if enum, hasEnum := propDetails["enum"].([]interface{}); hasEnum {
								prop.Enum = enum
							}
							if items, hasItems := propDetails["items"]; hasItems {
								prop.Items = items
							}

							ollamaTool.Function.Parameters.Properties[propName] = prop
						}
					}
				}
				if required, hasRequired := tool.Parameters["required"].([]interface{}); hasRequired {
					reqStrings := make([]string, 0, len(required))
					for _, req := range required {
						if reqStr, ok := req.(string); ok {
							reqStrings = append(reqStrings, reqStr)
						}
					}
					ollamaTool.Function.Parameters.Required = reqStrings
				}
			}

			ollamaTools = append(ollamaTools, ollamaTool)
		}
		chatReq.Tools = ollamaTools
	}

	// Apply connection-level configuration first
	if c.config.Temperature != nil {
		chatReq.Options["temperature"] = *c.config.Temperature
	}
	if c.config.MaxTokens != nil {
		chatReq.Options["num_predict"] = *c.config.MaxTokens
	}
	if c.config.TopP != nil {
		chatReq.Options["top_p"] = *c.config.TopP
	}
	if c.config.TopK != nil {
		chatReq.Options["top_k"] = *c.config.TopK
	}

	// Apply request-level configuration (overrides connection config)
	if request.Config != nil {
		if request.Config.Temperature != nil {
			chatReq.Options["temperature"] = *request.Config.Temperature
		}
		if request.Config.MaxTokens != nil {
			chatReq.Options["num_predict"] = *request.Config.MaxTokens
		}
		if request.Config.TopP != nil {
			chatReq.Options["top_p"] = *request.Config.TopP
		}
		if request.Config.TopK != nil {
			chatReq.Options["top_k"] = *request.Config.TopK
		}
	}

	return chatReq, nil
}

// convertFromOllamaResponse converts an Ollama response to ADK format.
func (c *OllamaConnection) convertFromOllamaResponse(resp *ChatResponse) *core.LLMResponse {
	if resp == nil {
		return &core.LLMResponse{}
	}

	response := &core.LLMResponse{
		Metadata: make(map[string]any),
	}

	// Convert message content
	if resp.Message.Content != "" || len(resp.Message.ToolCalls) > 0 {
		content := &core.Content{
			Role:  c.mapRoleFromOllama(resp.Message.Role),
			Parts: make([]core.Part, 0),
		}

		// Add text content if present
		if resp.Message.Content != "" {
			content.Parts = append(content.Parts, core.Part{
				Type: "text",
				Text: ptr.Ptr(resp.Message.Content),
			})
		}

		// Add thinking content if present
		if resp.Message.Thinking != "" {
			content.Parts = append(content.Parts, core.Part{
				Type: "text",
				Text: ptr.Ptr(fmt.Sprintf("[Thinking: %s]", resp.Message.Thinking)),
			})
		}

		// Convert tool calls
		for _, toolCall := range resp.Message.ToolCalls {
			content.Parts = append(content.Parts, core.Part{
				Type: "function_call",
				FunctionCall: &core.FunctionCall{
					ID:   fmt.Sprintf("call_%d", toolCall.Function.Index),
					Name: toolCall.Function.Name,
					Args: map[string]any(toolCall.Function.Arguments),
				},
			})
		}

		response.Content = content
	}

	// Add metadata
	if resp.Model != "" {
		response.Metadata["model"] = resp.Model
	}
	if !resp.CreatedAt.IsZero() {
		response.Metadata["created_at"] = resp.CreatedAt
	}
	if resp.DoneReason != "" {
		response.Metadata["done_reason"] = resp.DoneReason
	}

	// Add metrics
	if resp.TotalDuration > 0 {
		response.Metadata["total_duration"] = resp.TotalDuration
	}
	if resp.LoadDuration > 0 {
		response.Metadata["load_duration"] = resp.LoadDuration
	}
	if resp.PromptEvalCount > 0 {
		response.Metadata["prompt_eval_count"] = resp.PromptEvalCount
	}
	if resp.PromptEvalDuration > 0 {
		response.Metadata["prompt_eval_duration"] = resp.PromptEvalDuration
	}
	if resp.EvalCount > 0 {
		response.Metadata["eval_count"] = resp.EvalCount
	}
	if resp.EvalDuration > 0 {
		response.Metadata["eval_duration"] = resp.EvalDuration
	}

	// Set partial flag (inverse of Done)
	response.Partial = ptr.Ptr(!resp.Done)

	return response
}

// makeHTTPRequest makes an HTTP request to the Ollama API.
func (c *OllamaConnection) makeHTTPRequest(ctx context.Context, endpoint string, payload interface{}) (*http.Response, error) {
	// Serialize payload
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// mapRole maps ADK roles to Ollama roles.
func (c *OllamaConnection) mapRole(role string) string {
	switch role {
	case "user":
		return "user"
	case "agent", "model", "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

// mapRoleFromOllama maps Ollama roles to ADK roles.
func (c *OllamaConnection) mapRoleFromOllama(role string) string {
	switch role {
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "assistant"
	}
}
