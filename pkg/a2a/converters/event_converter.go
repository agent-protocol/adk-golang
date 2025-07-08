package converters

import (
	"fmt"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/core"
)

// ConvertEventToA2AEvents converts an ADK event to a list of A2A events.
func ConvertEventToA2AEvents(
	adkEvent *core.Event,
	session *core.Session,
	taskID string,
	contextID string,
) ([]interface{}, error) {
	if adkEvent == nil {
		return nil, fmt.Errorf("ADK event cannot be nil")
	}

	var a2aEvents []interface{}

	// Handle artifact deltas
	if adkEvent.Actions.ArtifactDelta != nil {
		for filename, version := range adkEvent.Actions.ArtifactDelta {
			artifactEvent, err := convertArtifactToA2AEvent(adkEvent, session, filename, version, taskID, contextID)
			if err != nil {
				// Log error but continue processing other artifacts
				fmt.Printf("Failed to convert artifact %s to A2A event: %v\n", filename, err)
				continue
			}
			a2aEvents = append(a2aEvents, artifactEvent)
		}
	}

	// Handle error scenarios
	if adkEvent.ErrorCode != nil && *adkEvent.ErrorCode != "" {
		errorEvent := createErrorStatusEvent(adkEvent, taskID, contextID)
		a2aEvents = append(a2aEvents, errorEvent)
	}

	// Handle regular message content
	if adkEvent.Content != nil && len(adkEvent.Content.Parts) > 0 {
		statusEvent, err := convertEventToStatusUpdateEvent(adkEvent, session, taskID, contextID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert event to status update: %w", err)
		}
		a2aEvents = append(a2aEvents, statusEvent)
	}

	return a2aEvents, nil
}

// ConvertEventToA2AMessage converts an ADK event to an A2A message.
func ConvertEventToA2AMessage(adkEvent *core.Event) (*a2a.Message, error) {
	if adkEvent == nil {
		return nil, fmt.Errorf("ADK event cannot be nil")
	}

	if adkEvent.Content == nil || len(adkEvent.Content.Parts) == 0 {
		return nil, nil // No content to convert
	}

	// Convert ADK parts to A2A parts
	a2aParts := make([]a2a.Part, 0, len(adkEvent.Content.Parts))
	for _, adkPart := range adkEvent.Content.Parts {
		a2aPart, err := convertADKPartToA2APart(adkPart, adkEvent)
		if err != nil {
			// Log error but continue with other parts
			fmt.Printf("Failed to convert ADK part: %v\n", err)
			continue
		}
		a2aParts = append(a2aParts, a2aPart)
	}

	if len(a2aParts) == 0 {
		return nil, nil // No parts could be converted
	}

	// Determine role based on event author
	role := "agent"
	if adkEvent.Author == "user" {
		role = "user"
	}

	return &a2a.Message{
		Role:  role,
		Parts: a2aParts,
	}, nil
}

// convertADKPartToA2APart converts an ADK part to an A2A part.
func convertADKPartToA2APart(adkPart core.Part, adkEvent *core.Event) (a2a.Part, error) {
	a2aPart := a2a.Part{
		Metadata: adkPart.Metadata,
	}

	switch adkPart.Type {
	case "text":
		if adkPart.Text == nil {
			return a2aPart, fmt.Errorf("text part missing text field")
		}
		a2aPart.Type = "text"
		a2aPart.Text = adkPart.Text

	case "function_call":
		if adkPart.FunctionCall == nil {
			return a2aPart, fmt.Errorf("function call part missing function_call field")
		}
		a2aPart.Type = "data"
		a2aPart.Data = map[string]any{
			"name": adkPart.FunctionCall.Name,
			"args": adkPart.FunctionCall.Args,
		}
		if adkPart.FunctionCall.ID != "" {
			a2aPart.Data["id"] = adkPart.FunctionCall.ID
		}

		// Add metadata to indicate this is a function call
		if a2aPart.Metadata == nil {
			a2aPart.Metadata = make(map[string]any)
		}
		a2aPart.Metadata["adk:type"] = "function_call"

		// Check if this is a long-running tool
		if isLongRunningTool(adkPart.FunctionCall.ID, adkEvent) {
			a2aPart.Metadata["adk:is_long_running"] = true
		}

	case "function_response":
		if adkPart.FunctionResponse == nil {
			return a2aPart, fmt.Errorf("function response part missing function_response field")
		}
		a2aPart.Type = "data"
		a2aPart.Data = map[string]any{
			"name":     adkPart.FunctionResponse.Name,
			"response": adkPart.FunctionResponse.Response,
		}
		if adkPart.FunctionResponse.ID != "" {
			a2aPart.Data["id"] = adkPart.FunctionResponse.ID
		}

		// Add metadata to indicate this is a function response
		if a2aPart.Metadata == nil {
			a2aPart.Metadata = make(map[string]any)
		}
		a2aPart.Metadata["adk:type"] = "function_response"

	case "file":
		// Handle file parts (implementation depends on specific requirements)
		a2aPart.Type = "file"
		if adkPart.Metadata != nil {
			if fileData, exists := adkPart.Metadata["file"]; exists {
				if fileContent, ok := fileData.(*a2a.FileContent); ok {
					a2aPart.File = fileContent
				}
			}
		}

	default:
		// Handle unknown types as data parts
		a2aPart.Type = "data"
		a2aPart.Data = map[string]any{
			"type":    adkPart.Type,
			"content": adkPart.Metadata,
		}
	}

	return a2aPart, nil
}

// convertEventToStatusUpdateEvent converts an ADK event to a TaskStatusUpdateEvent.
func convertEventToStatusUpdateEvent(
	adkEvent *core.Event,
	session *core.Session,
	taskID string,
	contextID string,
) (*a2a.TaskStatusUpdateEvent, error) {
	// Convert event to A2A message
	message, err := ConvertEventToA2AMessage(adkEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to convert event to A2A message: %w", err)
	}

	// Determine task state
	state := a2a.TaskStateWorking
	if adkEvent.TurnComplete != nil && *adkEvent.TurnComplete {
		state = a2a.TaskStateCompleted
	}

	// Check for special conditions that affect state
	if message != nil {
		for _, part := range message.Parts {
			if part.Type == "data" && part.Metadata != nil {
				if typeVal, exists := part.Metadata["adk:type"]; exists {
					if typeStr, ok := typeVal.(string); ok && typeStr == "function_call" {
						// Check if this is a long-running tool requiring input
						if isLongRunning, exists := part.Metadata["adk:is_long_running"]; exists {
							if isLongRunningBool, ok := isLongRunning.(bool); ok && isLongRunningBool {
								state = a2a.TaskStateInputRequired
							}
						}
					}
				}
			}
		}
	}

	// Create task status
	status := a2a.TaskStatus{
		State:   state,
		Message: message,
	}
	if adkEvent.Timestamp.IsZero() {
		status.Timestamp = timePtr(time.Now())
	} else {
		status.Timestamp = &adkEvent.Timestamp
	}

	return &a2a.TaskStatusUpdateEvent{
		ID:       taskID,
		Status:   status,
		Final:    state == a2a.TaskStateCompleted || state == a2a.TaskStateFailed || state == a2a.TaskStateCanceled,
		Metadata: createEventMetadata(adkEvent, session),
	}, nil
}

// convertArtifactToA2AEvent converts an artifact update to a TaskArtifactUpdateEvent.
func convertArtifactToA2AEvent(
	adkEvent *core.Event,
	session *core.Session,
	filename string,
	version int,
	taskID string,
	contextID string,
) (*a2a.TaskArtifactUpdateEvent, error) {
	// Create artifact ID
	artifactID := fmt.Sprintf("%s-%s-%s-%s-%d", session.AppName, session.UserID, session.ID, filename, version)

	// Create basic artifact (in a real implementation, you'd load the actual artifact content)
	artifact := a2a.Artifact{
		Name: &filename,
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr(fmt.Sprintf("Artifact %s version %d", filename, version)),
			},
		},
		Metadata: map[string]any{
			"artifactId": artifactID,
			"filename":   filename,
			"version":    version,
		},
	}

	return &a2a.TaskArtifactUpdateEvent{
		ID:       taskID,
		Artifact: artifact,
		Metadata: createEventMetadata(adkEvent, session),
	}, nil
}

// createErrorStatusEvent creates a TaskStatusUpdateEvent for error scenarios.
func createErrorStatusEvent(adkEvent *core.Event, taskID string, contextID string) *a2a.TaskStatusUpdateEvent {
	errorMessage := "An error occurred during processing"
	if adkEvent.ErrorMessage != nil {
		errorMessage = *adkEvent.ErrorMessage
	}

	message := &a2a.Message{
		Role: "agent",
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: &errorMessage,
			},
		},
	}

	status := a2a.TaskStatus{
		State:     a2a.TaskStateFailed,
		Message:   message,
		Timestamp: timePtr(time.Now()),
	}

	return &a2a.TaskStatusUpdateEvent{
		ID:     taskID,
		Status: status,
		Final:  true,
	}
}

// createEventMetadata creates metadata for A2A events from ADK events.
func createEventMetadata(adkEvent *core.Event, session *core.Session) map[string]any {
	metadata := map[string]any{
		"adk:app_name":      session.AppName,
		"adk:user_id":       session.UserID,
		"adk:session_id":    session.ID,
		"adk:invocation_id": adkEvent.InvocationID,
		"adk:author":        adkEvent.Author,
	}

	// Add optional fields if present
	if adkEvent.Branch != nil {
		metadata["adk:branch"] = *adkEvent.Branch
	}
	if adkEvent.ErrorCode != nil {
		metadata["adk:error_code"] = *adkEvent.ErrorCode
	}
	if adkEvent.CustomMetadata != nil {
		for key, value := range adkEvent.CustomMetadata {
			metadata[fmt.Sprintf("adk:custom:%s", key)] = value
		}
	}

	return metadata
}

// isLongRunningTool checks if a function call ID is in the long-running tools list.
func isLongRunningTool(functionCallID string, adkEvent *core.Event) bool {
	for _, longRunningID := range adkEvent.LongRunningToolIDs {
		if longRunningID == functionCallID {
			return true
		}
	}
	return false
}
