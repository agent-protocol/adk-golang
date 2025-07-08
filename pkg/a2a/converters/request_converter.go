package converters

import (
	"fmt"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/core"
)

// ADKRunArgs represents the arguments needed to run an ADK agent.
type ADKRunArgs struct {
	UserID     string
	SessionID  string
	NewMessage *core.Content
	RunConfig  *core.RunConfig
}

// RequestContext represents the context of an A2A request (simplified version).
type RequestContext struct {
	TaskID      string
	ContextID   string
	Message     *a2a.Message
	CurrentTask *a2a.Task
	SessionID   string
	UserID      string
	Metadata    map[string]interface{}
}

// ConvertA2ARequestToADKRunArgs converts an A2A request context to ADK run arguments.
func ConvertA2ARequestToADKRunArgs(requestCtx *RequestContext) (*ADKRunArgs, error) {
	if requestCtx.Message == nil {
		return nil, fmt.Errorf("request message cannot be nil")
	}

	// Convert A2A message parts to ADK content
	adkParts := make([]core.Part, 0, len(requestCtx.Message.Parts))
	for _, a2aPart := range requestCtx.Message.Parts {
		adkPart, err := convertA2APartToADKPart(a2aPart)
		if err != nil {
			return nil, fmt.Errorf("failed to convert A2A part: %w", err)
		}
		adkParts = append(adkParts, adkPart)
	}

	// Create ADK content
	adkContent := &core.Content{
		Role:  "user", // A2A messages from clients are typically user messages
		Parts: adkParts,
	}

	// Determine user ID
	userID := requestCtx.UserID
	if userID == "" {
		// Fallback to context ID if no user ID is provided
		userID = fmt.Sprintf("A2A_USER_%s", requestCtx.ContextID)
	}

	// Use context ID as session ID if not explicitly provided
	sessionID := requestCtx.SessionID
	if sessionID == "" {
		sessionID = requestCtx.ContextID
	}

	return &ADKRunArgs{
		UserID:     userID,
		SessionID:  sessionID,
		NewMessage: adkContent,
		RunConfig:  &core.RunConfig{}, // Default run config
	}, nil
}

// convertA2APartToADKPart converts a single A2A part to an ADK part.
func convertA2APartToADKPart(a2aPart a2a.Part) (core.Part, error) {
	adkPart := core.Part{
		Metadata: a2aPart.Metadata,
	}

	switch a2aPart.Type {
	case "text":
		if a2aPart.Text == nil {
			return adkPart, fmt.Errorf("text part missing text field")
		}
		adkPart.Type = "text"
		adkPart.Text = a2aPart.Text

	case "file":
		if a2aPart.File == nil {
			return adkPart, fmt.Errorf("file part missing file field")
		}
		// For now, we'll handle file parts as metadata
		// In a full implementation, this would handle file content properly
		adkPart.Type = "file"
		adkPart.Metadata = map[string]any{
			"file": a2aPart.File,
		}

	case "data":
		if a2aPart.Data == nil {
			return adkPart, fmt.Errorf("data part missing data field")
		}

		// Check if this is a function call based on metadata or data structure
		if isA2AFunctionCall(a2aPart) {
			functionCall, err := convertA2ADataToFunctionCall(a2aPart.Data)
			if err != nil {
				return adkPart, fmt.Errorf("failed to convert A2A data to function call: %w", err)
			}
			adkPart.Type = "function_call"
			adkPart.FunctionCall = functionCall
		} else {
			// Handle as generic data
			adkPart.Type = "data"
			adkPart.Metadata = map[string]any{
				"data": a2aPart.Data,
			}
		}

	default:
		return adkPart, fmt.Errorf("unsupported A2A part type: %s", a2aPart.Type)
	}

	return adkPart, nil
}

// isA2AFunctionCall determines if an A2A data part represents a function call.
func isA2AFunctionCall(a2aPart a2a.Part) bool {
	// Check metadata for function call type indicator
	if a2aPart.Metadata != nil {
		if typeVal, exists := a2aPart.Metadata["adk:type"]; exists {
			if typeStr, ok := typeVal.(string); ok && typeStr == "function_call" {
				return true
			}
		}
	}

	// Check if data has function call structure (name and args)
	if a2aPart.Data != nil {
		_, hasName := a2aPart.Data["name"]
		_, hasArgs := a2aPart.Data["args"]
		return hasName && hasArgs
	}

	return false
}

// convertA2ADataToFunctionCall converts A2A data to an ADK function call.
func convertA2ADataToFunctionCall(data map[string]any) (*core.FunctionCall, error) {
	name, ok := data["name"].(string)
	if !ok {
		return nil, fmt.Errorf("function call missing or invalid name field")
	}

	// Extract arguments (may be nil for functions with no args)
	var args map[string]any
	if argsData, exists := data["args"]; exists {
		if argsMap, ok := argsData.(map[string]any); ok {
			args = argsMap
		} else {
			return nil, fmt.Errorf("function call args must be a map")
		}
	}

	// Extract ID if present
	id := ""
	if idData, exists := data["id"]; exists {
		if idStr, ok := idData.(string); ok {
			id = idStr
		}
	}

	return &core.FunctionCall{
		ID:   id,
		Name: name,
		Args: args,
	}, nil
}
