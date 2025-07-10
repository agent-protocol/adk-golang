// Package a2a provides conversion utilities between A2A protocol data structures
// and ADK core data structures. These converters handle the transformation of
// messages, parts, and content between the two formats, ensuring compatibility
// and proper data mapping.
//
// The conversion functions support:
// - Text content (bidirectional)
// - File content (with metadata preservation)
// - Function calls (represented as text in A2A)
// - Data parts (with structured metadata)
//
// Usage:
//
//	// Convert A2A message to core content
//	content := a2a.ConvertA2AMessageToContent(a2aMessage)
//
//	// Convert core content to A2A message
//	message := a2a.ConvertCoreContentToA2AMessage(content, "unique-message-id")
//
//	// Extract simple text from parts
//	text := a2a.ExtractTextFromA2AParts(parts)
package a2a

import (
	"fmt"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// ConvertA2AMessageToContent converts an A2A message to ADK content
func ConvertA2AMessageToContent(message *Message) *core.Content {
	if message == nil {
		return nil
	}

	parts := ConvertA2APartsToCoreParts(message.Parts)

	return &core.Content{
		Role:  message.Role,
		Parts: parts,
	}
}

// ConvertCoreContentToA2AMessage converts ADK content to an A2A message
func ConvertCoreContentToA2AMessage(content *core.Content, messageID string) *Message {
	if content == nil {
		return nil
	}

	parts := ConvertCorePartsToA2AParts(content.Parts)

	return &Message{
		MessageID: messageID,
		Role:      content.Role,
		Parts:     parts,
	}
}

// ConvertA2APartsToCoreParts converts A2A parts to ADK core parts
func ConvertA2APartsToCoreParts(a2aParts []Part) []core.Part {
	var parts []core.Part

	for _, part := range a2aParts {
		corePart := ConvertA2APartToCorePart(part)
		if corePart != nil {
			parts = append(parts, *corePart)
		}
	}

	return parts
}

// ConvertCorePartsToA2AParts converts ADK core parts to A2A parts
func ConvertCorePartsToA2AParts(coreParts []core.Part) []Part {
	var parts []Part

	for _, part := range coreParts {
		a2aPart := ConvertCorePartToA2APart(part)
		if a2aPart != nil {
			parts = append(parts, *a2aPart)
		}
	}

	return parts
}

// ConvertA2APartToCorePart converts a single A2A part to an ADK core part
func ConvertA2APartToCorePart(a2aPart Part) *core.Part {
	switch a2aPart.Type {
	case "text":
		if a2aPart.Text != nil {
			return &core.Part{
				Type:     "text",
				Text:     a2aPart.Text,
				Metadata: a2aPart.Metadata,
			}
		}
	case "file":
		if a2aPart.File != nil {
			// Convert file content to core part
			// For now, we'll represent it as text with file metadata
			fileName := ""
			if a2aPart.File.Name != nil {
				fileName = *a2aPart.File.Name
			}
			return &core.Part{
				Type: "file",
				Text: ptr.Ptr(fmt.Sprintf("File: %s", fileName)),
				Metadata: map[string]any{
					"file_name":     a2aPart.File.Name,
					"file_uri":      a2aPart.File.URI,
					"file_bytes":    a2aPart.File.Bytes,
					"file_mime":     a2aPart.File.MimeType,
					"original_type": "file",
				},
			}
		}
	case "data":
		if a2aPart.Data != nil {
			// Convert data part to core part
			// Represent as structured data in metadata
			return &core.Part{
				Type: "data",
				Text: ptr.Ptr("Data content"),
				Metadata: map[string]any{
					"data":          a2aPart.Data,
					"original_type": "data",
				},
			}
		}
	}

	// For unsupported or malformed parts, return nil to skip them
	return nil
}

// ConvertCorePartToA2APart converts a single ADK core part to an A2A part
func ConvertCorePartToA2APart(corePart core.Part) *Part {
	switch corePart.Type {
	case "text":
		if corePart.Text != nil {
			return &Part{
				Type:     "text",
				Text:     corePart.Text,
				Metadata: corePart.Metadata,
			}
		}
	case "function_call":
		if corePart.FunctionCall != nil {
			// Convert function call to text representation for A2A
			// A2A doesn't have native function call support, so we represent it as text
			functionCallText := fmt.Sprintf("Function call: %s(%v)",
				corePart.FunctionCall.Name, corePart.FunctionCall.Args)
			return &Part{
				Type: "text",
				Text: ptr.Ptr(functionCallText),
				Metadata: map[string]any{
					"function_call": corePart.FunctionCall,
					"original_type": "function_call",
				},
			}
		}
	case "function_response":
		if corePart.FunctionResponse != nil {
			// Convert function response to text representation for A2A
			functionResponseText := fmt.Sprintf("Function response: %s = %v",
				corePart.FunctionResponse.Name, corePart.FunctionResponse.Response)
			return &Part{
				Type: "text",
				Text: ptr.Ptr(functionResponseText),
				Metadata: map[string]any{
					"function_response": corePart.FunctionResponse,
					"original_type":     "function_response",
				},
			}
		}
	case "file":
		// Check if this was originally a file from A2A
		if corePart.Metadata != nil {
			if fileName, ok := corePart.Metadata["file_name"].(*string); ok {
				fileURI, _ := corePart.Metadata["file_uri"].(*string)
				fileBytes, _ := corePart.Metadata["file_bytes"].(*string)
				fileMime, _ := corePart.Metadata["file_mime"].(*string)

				return &Part{
					Type: "file",
					File: &FileContent{
						Name:     fileName,
						URI:      fileURI,
						Bytes:    fileBytes,
						MimeType: fileMime,
					},
					Metadata: corePart.Metadata,
				}
			}
		}
		// Otherwise, convert to text representation
		if corePart.Text != nil {
			return &Part{
				Type:     "text",
				Text:     corePart.Text,
				Metadata: corePart.Metadata,
			}
		}
	case "data":
		// Check if this was originally data from A2A
		if corePart.Metadata != nil {
			if data, ok := corePart.Metadata["data"].(map[string]any); ok {
				return &Part{
					Type:     "data",
					Data:     data,
					Metadata: corePart.Metadata,
				}
			}
		}
		// Otherwise, convert to text representation
		if corePart.Text != nil {
			return &Part{
				Type:     "text",
				Text:     corePart.Text,
				Metadata: corePart.Metadata,
			}
		}
	}

	// For unsupported or malformed parts, return nil to skip them
	return nil
}

// ConvertA2ATaskStatusToContent converts A2A task status message to ADK content
func ConvertA2ATaskStatusToContent(status *TaskStatus) *core.Content {
	if status == nil || status.Message == nil {
		return nil
	}

	return ConvertA2AMessageToContent(status.Message)
}

// ConvertA2AArtifactToContent converts A2A artifact to ADK content
func ConvertA2AArtifactToContent(artifact *Artifact) *core.Content {
	if artifact == nil {
		return nil
	}

	parts := ConvertA2APartsToCoreParts(artifact.Parts)

	// If no parts are available, create a default text part
	if len(parts) == 0 {
		parts = []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr("Artifact content"),
				Metadata: map[string]any{
					"artifact_name":        artifact.Name,
					"artifact_description": artifact.Description,
				},
			},
		}
	}

	return &core.Content{
		Role:  "agent", // Artifacts are typically from agents
		Parts: parts,
	}
}

// ExtractTextFromA2AParts extracts text content from A2A parts for simple use cases
func ExtractTextFromA2AParts(parts []Part) string {
	for _, part := range parts {
		if part.Type == "text" && part.Text != nil {
			return *part.Text
		}
	}
	return ""
}

// ExtractTextFromCoreParts extracts text content from core parts for simple use cases
func ExtractTextFromCoreParts(parts []core.Part) string {
	for _, part := range parts {
		if part.Type == "text" && part.Text != nil {
			return *part.Text
		}
	}
	return ""
}

// CreateSimpleTextA2AMessage creates a simple A2A message with text content
func CreateSimpleTextA2AMessage(messageID, role, text string) *Message {
	return &Message{
		MessageID: messageID,
		Role:      role,
		Parts: []Part{
			{
				Type: "text",
				Text: ptr.Ptr(text),
			},
		},
	}
}

// CreateSimpleTextCoreContent creates a simple core content with text
func CreateSimpleTextCoreContent(role, text string) *core.Content {
	return &core.Content{
		Role: role,
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr(text),
			},
		},
	}
}

func DetermineTaskStateFromEvent(event *core.Event) TaskState {
	if event.ErrorCode != nil {
		return TaskStateFailed
	}
	if event.Actions.RequestedAuthConfigs != nil {
		return TaskStateAuthRequired
	}
	if event.TurnComplete != nil && *event.TurnComplete {
		return TaskStateCompleted
	}
	return TaskStateWorking
}

// ConvertEventToTaskStatusUpdate converts an ADK event to a task status update message
func ConvertEventToTaskStatusUpdate(event *core.Event, invocationCtx *core.InvocationContext) *TaskStatusUpdateEvent {
	panic("ConvertEventToTaskStatusUpdate is not implemented yet")
}

func ConvertEventToTaskArtifactUpdate(event *core.Event, filename string, version int) *TaskArtifactUpdateEvent {
	panic("ConvertEventToTaskArtifactUpdate is not implemented yet")
}
