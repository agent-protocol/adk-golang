// Package llm provides LLM connection implementations for various providers.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

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

// NewOllamaConnectionFromEnv creates a new Ollama connection using environment variables.
// Supports OLLAMA_API_BASE for base URL and OLLAMA_MODEL for model name.
func NewOllamaConnectionFromEnv() *OllamaConnection {
	config := DefaultOllamaConfig()

	// Check for environment variables
	if baseURL := getEnvOrDefault("OLLAMA_API_BASE", ""); baseURL != "" {
		config.BaseURL = baseURL
	}
	if model := getEnvOrDefault("OLLAMA_MODEL", ""); model != "" {
		config.Model = model
	}

	return NewOllamaConnection(config)
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
	var ollamaResp OllamaChatResponse
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
			var chunk OllamaChatResponse
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

// buildEnhancedToolInstructions creates clear instructions for tool usage.
func (c *OllamaConnection) buildEnhancedToolInstructions(tools []*core.FunctionDeclaration) string {
	var sb strings.Builder

	sb.WriteString("## Tool Usage Instructions\n\n")
	sb.WriteString("You have access to the following tools. Use them when they can help answer the user's question:\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))
		if tool.Parameters != nil {
			if props, hasProps := tool.Parameters["properties"].(map[string]interface{}); hasProps {
				sb.WriteString("Parameters:\n")
				for paramName, paramDef := range props {
					if paramMap, ok := paramDef.(map[string]interface{}); ok {
						if desc, hasDesc := paramMap["description"].(string); hasDesc {
							sb.WriteString(fmt.Sprintf("- %s: %s\n", paramName, desc))
						}
					}
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Response Guidelines\n\n")
	sb.WriteString("1. **Analyze First**: Determine if any tool can help answer the user's request\n")
	sb.WriteString("2. **Use Tools When Relevant**: If a tool is available and can provide the needed information, use it\n")
	sb.WriteString("3. **Tool Call Format**: When calling a tool, respond with a JSON object in this exact format:\n")
	sb.WriteString("   ```json\n")
	sb.WriteString("   {\"name\": \"tool_name\", \"parameters\": {\"param1\": \"value1\", \"param2\": \"value2\"}}\n")
	sb.WriteString("   ```\n")
	sb.WriteString("4. **Direct Answers**: If no tool is needed, provide a direct, helpful answer\n")
	sb.WriteString("5. **No Repeated Calls**: Don't call the same tool multiple times with identical parameters\n\n")
	sb.WriteString("Remember: Your goal is to be helpful. Use tools when they add value, provide direct answers when they don't.\n")

	return sb.String()
}

// convertToOllamaRequest converts an ADK LLMRequest to Ollama format.
func (c *OllamaConnection) convertToOllamaRequest(request *core.LLMRequest, stream bool) (*OllamaChatRequest, error) {
	ollamaReq := &OllamaChatRequest{
		Model:    c.model,
		Messages: make([]OllamaMessage, 0),
		Stream:   stream,
		Options:  make(map[string]interface{}),
	}

	// Apply configuration overrides
	if request.Config != nil {
		if request.Config.Model != "" {
			ollamaReq.Model = request.Config.Model
		}
		if request.Config.Temperature != nil {
			ollamaReq.Options["temperature"] = *request.Config.Temperature
		}
		if request.Config.MaxTokens != nil {
			ollamaReq.Options["num_predict"] = *request.Config.MaxTokens
		}
		if request.Config.TopP != nil {
			ollamaReq.Options["top_p"] = *request.Config.TopP
		}
		if request.Config.TopK != nil {
			ollamaReq.Options["top_k"] = *request.Config.TopK
		}
	}

	// Apply default config values if not overridden
	if c.config.Temperature != nil && ollamaReq.Options["temperature"] == nil {
		ollamaReq.Options["temperature"] = *c.config.Temperature
	}

	// Enhanced tool calling instruction for Ollama models
	if len(request.Tools) > 0 {
		// Add enhanced system message for better tool understanding
		toolInstructions := c.buildEnhancedToolInstructions(request.Tools)

		// Check if there's already a system message
		hasSystemMessage := false
		for i, content := range request.Contents {
			if content.Role == "system" {
				// Enhance existing system message
				for j, part := range content.Parts {
					if part.Type == "text" && part.Text != nil {
						enhanced := *part.Text + "\n\n" + toolInstructions
						request.Contents[i].Parts[j].Text = &enhanced
					}
				}
				hasSystemMessage = true
				break
			}
		}

		// Add system message if none exists
		if !hasSystemMessage {
			systemContent := core.Content{
				Role: "system",
				Parts: []core.Part{
					{
						Type: "text",
						Text: &toolInstructions,
					},
				},
			}
			// Insert at the beginning
			request.Contents = append([]core.Content{systemContent}, request.Contents...)
		}
	}

	// Convert contents to messages
	for _, content := range request.Contents {
		msg := OllamaMessage{
			Role: c.mapRole(content.Role),
		}

		// Handle different part types
		var textParts []string
		var toolCalls []OllamaToolCall

		for _, part := range content.Parts {
			switch part.Type {
			case "text":
				if part.Text != nil {
					textParts = append(textParts, *part.Text)
				}
			case "function_call":
				if part.FunctionCall != nil {
					toolCall := OllamaToolCall{
						Function: OllamaFunctionCall{
							Name:      part.FunctionCall.Name,
							Arguments: part.FunctionCall.Args,
						},
					}
					toolCalls = append(toolCalls, toolCall)
				}
			case "function_response":
				if part.FunctionResponse != nil {
					// Convert function response to text for Ollama
					responseText := fmt.Sprintf("Function %s returned: %v",
						part.FunctionResponse.Name, part.FunctionResponse.Response)
					textParts = append(textParts, responseText)
				}
			}
		}

		// Set message content
		if len(textParts) > 0 {
			msg.Content = strings.Join(textParts, "\n")
		}

		// Add tool calls if present
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}

		ollamaReq.Messages = append(ollamaReq.Messages, msg)
	}

	// Convert tools to Ollama format
	if len(request.Tools) > 0 {
		ollamaReq.Tools = make([]OllamaTool, 0, len(request.Tools))
		for _, tool := range request.Tools {
			ollamaTool := OllamaTool{
				Type: "function",
				Function: OllamaFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
			ollamaReq.Tools = append(ollamaReq.Tools, ollamaTool)
		}
	}

	return ollamaReq, nil
}

// convertFromOllamaResponse converts an Ollama response to ADK format.
func (c *OllamaConnection) convertFromOllamaResponse(resp *OllamaChatResponse) *core.LLMResponse {
	response := &core.LLMResponse{
		Partial: ptr.Ptr(!resp.Done),
		Metadata: map[string]any{
			"model":         resp.Model,
			"created_at":    resp.CreatedAt,
			"total_tokens":  resp.TotalDuration,
			"prompt_tokens": resp.PromptEvalCount,
		},
	}

	// Convert message content
	if resp.Message.Content != "" || len(resp.Message.ToolCalls) > 0 {
		content := &core.Content{
			Role:  c.mapRoleFromOllama(resp.Message.Role),
			Parts: make([]core.Part, 0),
		}

		// Add text content
		if resp.Message.Content != "" {
			content.Parts = append(content.Parts, core.Part{
				Type: "text",
				Text: &resp.Message.Content,
			})
		}

		// Add tool calls
		for _, toolCall := range resp.Message.ToolCalls {
			content.Parts = append(content.Parts, core.Part{
				Type: "function_call",
				FunctionCall: &core.FunctionCall{
					ID:   toolCall.ID,
					Name: toolCall.Function.Name,
					Args: toolCall.Function.Arguments,
				},
			})
		}

		response.Content = content
	}

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

// Ollama API types

// OllamaChatRequest represents a request to the Ollama chat API.
type OllamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Tools    []OllamaTool           `json:"tools,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// OllamaMessage represents a message in the Ollama format.
type OllamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

// OllamaTool represents a tool in the Ollama format.
type OllamaTool struct {
	Type     string         `json:"type"`
	Function OllamaFunction `json:"function"`
}

// OllamaFunction represents a function definition in Ollama format.
type OllamaFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// OllamaToolCall represents a tool call in Ollama format.
type OllamaToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function OllamaFunctionCall `json:"function"`
}

// OllamaFunctionCall represents a function call in Ollama format.
type OllamaFunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// OllamaChatResponse represents a response from the Ollama chat API.
type OllamaChatResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            OllamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

// getEnvOrDefault returns the environment variable value or the default if not set.
func getEnvOrDefault(envVar, defaultValue string) string {
	// Import os package for environment variable access
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}
