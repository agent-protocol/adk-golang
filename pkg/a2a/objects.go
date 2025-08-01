package a2a

import (
	"encoding/json"
	"fmt"
	"time"
)

// AgentAuthentication defines authentication details for an agent.
type AgentAuthentication struct {
	Schemes     []string `json:"schemes"`
	Credentials *string  `json:"credentials,omitempty"`
}

// AgentCapabilities defines the capabilities of an agent.
type AgentCapabilities struct {
	Streaming              bool `json:"streaming,omitempty"`
	PushNotifications      bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

// AgentCard provides metadata about an agent.
type AgentCard struct {
	Name               string               `json:"name"`
	Description        *string              `json:"description,omitempty"`
	URL                string               `json:"url"`
	Provider           *AgentProvider       `json:"provider,omitempty"`
	Version            string               `json:"version"`
	DocumentationURL   *string              `json:"documentationUrl,omitempty"`
	Capabilities       AgentCapabilities    `json:"capabilities"`
	Authentication     *AgentAuthentication `json:"authentication,omitempty"`
	DefaultInputModes  []string             `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string             `json:"defaultOutputModes,omitempty"`
	Skills             []AgentSkill         `json:"skills"`
}

// AgentProvider provides information about the agent's provider.
type AgentProvider struct {
	Organization string  `json:"organization"`
	URL          *string `json:"url,omitempty"`
}

// AgentSkill describes a specific skill or capability of the agent.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// Artifact represents a piece of data generated or used by a task.
type Artifact struct {
	ArtifactId  string         `json:"artifactId"`
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Parts       []Part         `json:"parts"` // Part is a union type, using any or specific struct with Type field
	Metadata    map[string]any `json:"metadata,omitempty"`
	Extensions  []string       `json:"extensions,omitempty"`
}

// PushNotificationAuthenticationInfo holds authentication details.
type PushNotificationAuthenticationInfo struct {
	Schemes     []string `json:"schemes"`
	Credentials *string  `json:"credentials,omitempty"`
}

// CancelTaskRequest is a JSON-RPC request to cancel a task.
type CancelTaskRequest struct {
	JSONRPC string       `json:"jsonrpc"` // "2.0"
	ID      any          `json:"id,omitempty"`
	Method  string       `json:"method"` // "tasks/cancel"
	Params  TaskIdParams `json:"params"`
}

// CancelTaskResponse is a JSON-RPC response for a cancel task request.
type CancelTaskResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  *Task         `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// GetAgentCardRequest represents a request to get an agent card.
type GetAgentCardRequest struct {
	JSONRPC string `json:"jsonrpc"` // "2.0"
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`           // "agents/card"
	Params  any    `json:"params,omitempty"` // Usually empty for agent's own card
}

// GetAgentCardResponse represents the response containing an agent card.
type GetAgentCardResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  *AgentCard    `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// DataPart represents a structured data part of a message or artifact.
type DataPart struct {
	Type     string         `json:"type"` // "data"
	Data     map[string]any `json:"data"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// FileContent represents the content of a file, either inline or via URI.
type FileContent struct {
	Name     *string `json:"name,omitempty"`
	MimeType *string `json:"mimeType,omitempty"`
	Bytes    *string `json:"bytes,omitempty"` // Base64 encoded content
	URI      *string `json:"uri,omitempty"`
	// Validation: Either Bytes or URI should be non-nil, but not both.
}

// Validate ensures that FileContent has either Bytes or URI but not both
func (fc *FileContent) Validate() error {
	if (fc.Bytes == nil && fc.URI == nil) || (fc.Bytes != nil && fc.URI != nil) {
		return fmt.Errorf("FileContent must have either Bytes or URI field, but not both")
	}
	return nil
}

// FilePart represents a file part of a message or artifact.
type FilePart struct {
	Type     string         `json:"type"` // "file"
	File     FileContent    `json:"file"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// GetTaskPushNotificationRequest is a JSON-RPC request to get push notification config for a task.
type GetTaskPushNotificationRequest struct {
	JSONRPC string       `json:"jsonrpc"` // "2.0"
	ID      any          `json:"id,omitempty"`
	Method  string       `json:"method"` // "tasks/pushNotification/get"
	Params  TaskIdParams `json:"params"`
}

// GetTaskPushNotificationResponse is a JSON-RPC response for getting push notification config.
type GetTaskPushNotificationResponse struct {
	JSONRPC string                      `json:"jsonrpc,omitempty"` // "2.0"
	ID      any                         `json:"id"`
	Result  *TaskPushNotificationConfig `json:"result,omitempty"`
	Error   *JSONRPCError               `json:"error,omitempty"`
}

// GetTaskRequest is a JSON-RPC request to retrieve task details.
type GetTaskRequest struct {
	JSONRPC string          `json:"jsonrpc"` // "2.0"
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"` // "tasks/get"
	Params  TaskQueryParams `json:"params"`
}

// GetTaskResponse is a JSON-RPC response containing task details.
type GetTaskResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  *Task         `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCMessage is a base structure for JSON-RPC messages.
type JSONRPCMessage struct {
	JSONRPC string `json:"jsonrpc,omitempty"` // "2.0"
	ID      any    `json:"id,omitempty"`
}

// JSONRPCRequest is a base structure for JSON-RPC requests.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"` // "2.0"
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse is a base structure for JSON-RPC responses.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// Message represents a single message in a task conversation.
type Message struct {
	Kind             string         `json:"kind"`      // "message"
	MessageID        string         `json:"messageId"` // REQUIRED by A2A spec
	TaskID           *string        `json:"taskId,omitempty"`
	ContextID        *string        `json:"contextId,omitempty"`
	Role             string         `json:"role"`  // "user" or "agent"
	Parts            []Part         `json:"parts"` // Part is a union type
	Metadata         map[string]any `json:"metadata,omitempty"`
	Extensions       []string       `json:"extensions,omitempty"`
	ReferenceTaskIds []string       `json:"referenceTaskIds,omitempty"` // List of task IDs this message references
}

// MessageSendParams represents parameters for message/send and message/stream methods (A2A spec)
type MessageSendParams struct {
	Message       Message                   `json:"message"` // REQUIRED
	Configuration *MessageSendConfiguration `json:"configuration,omitempty"`
	Metadata      map[string]any            `json:"metadata,omitempty"`
}

// MessageSendConfiguration represents optional configuration for message sending (A2A spec)
type MessageSendConfiguration struct {
	AcceptedOutputModes    []string                `json:"acceptedOutputModes"` // REQUIRED when present
	Blocking               *bool                   `json:"blocking,omitempty"`
	HistoryLength          *int                    `json:"historyLength,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig,omitempty"`
}

// PushNotificationConfig defines the configuration for push notifications.
type PushNotificationConfig struct {
	ID             *string                             `json:"id"`
	URL            string                              `json:"url"`
	Token          *string                             `json:"token,omitempty"`
	Authentication *PushNotificationAuthenticationInfo `json:"authentication,omitempty"`
}

// Part represents a component of a message or artifact.
// It's a union type (TextPart, FilePart, DataPart).
// Using a struct with a Type field and optional content fields is one way to model this.
// Custom UnmarshalJSON is often preferred for true union type handling.
type Part struct {
	Type     string         `json:"type"` // "text", "file", or "data"
	Text     *string        `json:"text,omitempty"`
	File     *FileContent   `json:"file,omitempty"`
	Data     map[string]any `json:"data,omitempty"` // For DataPart type
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for Part to ensure data consistency
func (p *Part) UnmarshalJSON(data []byte) error {
	// First unmarshal to get the type field
	type PartAlias Part
	var temp struct {
		Type string `json:"type"`
		*PartAlias
	}
	temp.PartAlias = (*PartAlias)(p)

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Now validate based on the type field
	switch temp.Type {
	case "text":
		if temp.Text == nil {
			return fmt.Errorf("text part missing 'text' field")
		}
	case "file":
		if temp.File == nil {
			return fmt.Errorf("file part missing 'file' field")
		}
	case "data":
		if temp.Data == nil {
			return fmt.Errorf("data part missing 'data' field")
		}
	default:
		return fmt.Errorf("unknown part type: %s", temp.Type)
	}

	return nil
}

// ============================================================================
// A2A Protocol Request Types (Correct Method Names)
// ============================================================================

// SendMessageRequest represents the "message/send" JSON-RPC request (A2A compliant)
type SendMessageRequest struct {
	JSONRPC string            `json:"jsonrpc"` // "2.0"
	ID      any               `json:"id,omitempty"`
	Method  string            `json:"method"` // "message/send"
	Params  MessageSendParams `json:"params"`
}

// SendMessageResponse represents response from "message/send"
type SendMessageResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"` // SendMessageResponseResult
	Error   *JSONRPCError `json:"error,omitempty"`
}

type SendMessageResponseResult interface {
	Message | Task // Result can be either a Message or a Task
}

// SendStreamingMessageRequest represents the "message/stream" JSON-RPC request (A2A compliant)
type SendStreamingMessageRequest struct {
	JSONRPC string            `json:"jsonrpc"` // "2.0"
	ID      any               `json:"id,omitempty"`
	Method  string            `json:"method"` // "message/stream"
	Params  MessageSendParams `json:"params"` // Same as SendMessageRequest
}

// SendStreamingMessageResponse represents response/events from "message/stream"
type SendStreamingMessageResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"` // SendStreamingMessageResponseResult
	Error   *JSONRPCError `json:"error,omitempty"`
	Final   *bool         `json:"final,omitempty"` // Indicates final event in stream
}

type SendStreamingMessageResponseResult interface {
	Message | Task | TaskStatusUpdateEvent | TaskArtifactUpdateEvent
}

// ============================================================================
// Legacy Task-based Types (for backward compatibility if needed)
// ============================================================================

// SendTaskRequest is a JSON-RPC request to send a message/start a task.
// DEPRECATED: Use SendMessageRequest with "message/send" method instead
type SendTaskRequest struct {
	JSONRPC string         `json:"jsonrpc"` // "2.0"
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"` // "tasks/send" - INCORRECT per A2A spec
	Params  TaskSendParams `json:"params"`
}

// SendTaskResponse is a JSON-RPC response for a send task request.
type SendTaskResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  *Task         `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// SendTaskStreamingRequest is a JSON-RPC request to send a message and subscribe to updates.
type SendTaskStreamingRequest struct {
	JSONRPC string         `json:"jsonrpc"` // "2.0"
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"` // "tasks/sendSubscribe"
	Params  TaskSendParams `json:"params"`
}

// SendTaskStreamingResponse is a JSON-RPC response/event during a streaming task.
// The Result field can be TaskStatusUpdateEvent or TaskArtifactUpdateEvent.
type SendTaskStreamingResponse struct {
	JSONRPC string        `json:"jsonrpc,omitempty"` // "2.0"
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"` // *TaskStatusUpdateEvent or *TaskArtifactUpdateEvent
	Error   *JSONRPCError `json:"error,omitempty"`
}

// GetStatusUpdate returns the TaskStatusUpdateEvent if the Result contains one, or nil otherwise
func (r *SendTaskStreamingResponse) GetStatusUpdate() *TaskStatusUpdateEvent {
	if r.Result == nil {
		return nil
	}
	if update, ok := r.Result.(*TaskStatusUpdateEvent); ok {
		return update
	}
	return nil
}

// GetArtifactUpdate returns the TaskArtifactUpdateEvent if the Result contains one, or nil otherwise
func (r *SendTaskStreamingResponse) GetArtifactUpdate() *TaskArtifactUpdateEvent {
	if r.Result == nil {
		return nil
	}
	if update, ok := r.Result.(*TaskArtifactUpdateEvent); ok {
		return update
	}
	return nil
}

// SetTaskPushNotificationRequest is a JSON-RPC request to set push notification config for a task.
type SetTaskPushNotificationRequest struct {
	JSONRPC string                     `json:"jsonrpc"` // "2.0"
	ID      any                        `json:"id,omitempty"`
	Method  string                     `json:"method"` // "tasks/pushNotification/set"
	Params  TaskPushNotificationConfig `json:"params"` // Note: Schema shows params is TaskPushNotificationConfig, not TaskIdParams
}

// SetTaskPushNotificationResponse is a JSON-RPC response for setting push notification config.
type SetTaskPushNotificationResponse struct {
	JSONRPC string                      `json:"jsonrpc,omitempty"` // "2.0"
	ID      any                         `json:"id"`
	Result  *TaskPushNotificationConfig `json:"result,omitempty"`
	Error   *JSONRPCError               `json:"error,omitempty"`
}

// Task represents the state and data associated with an agent task.
type Task struct {
	ID        string         `json:"id"`
	ContextID *string        `json:"contextId,omitempty"`
	Status    TaskStatus     `json:"status"`
	History   []Message      `json:"history,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Kind      string         `json:"kind"` // "task"
}

// TaskPushNotificationConfig associates a task ID with its push notification settings.
type TaskPushNotificationConfig struct {
	TaskID                 string                 `json:"taskId"`
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
}

// TaskIdParams provides parameters containing just a task ID.
type TaskIdParams struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskQueryParams provides parameters for querying a task, including history length.
type TaskQueryParams struct {
	ID            string         `json:"id"`
	HistoryLength *int           `json:"historyLength,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// TaskSendParams provides parameters for sending a message to a task.
type TaskSendParams struct {
	ID               string                  `json:"id"`
	SessionID        *string                 `json:"sessionId,omitempty"`
	Message          Message                 `json:"message"`
	PushNotification *PushNotificationConfig `json:"pushNotification,omitempty"`
	HistoryLength    *int                    `json:"historyLength,omitempty"`
	Metadata         map[string]any          `json:"metadata,omitempty"`
}

// TaskState represents the possible states of a task.
type TaskState string

const (
	TaskStateSubmitted     TaskState = "submitted"
	TaskStateWorking       TaskState = "working"
	TaskStateInputRequired TaskState = "input-required"
	TaskStateCompleted     TaskState = "completed"
	TaskStateCanceled      TaskState = "canceled"
	TaskStateFailed        TaskState = "failed"
	TaskStateRejected      TaskState = "rejected"
	TaskStateAuthRequired  TaskState = "auth-required"
	TaskStateUnknown       TaskState = "unknown"
)

// TaskStatus represents the current status of a task.
type TaskStatus struct {
	State     TaskState  `json:"state"`
	Message   *Message   `json:"message,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"` // Use pointer to allow omission if not present
}

// TaskResubscriptionRequest is a JSON-RPC request to resubscribe to task updates.
type TaskResubscriptionRequest struct {
	JSONRPC string          `json:"jsonrpc"` // "2.0"
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"` // "tasks/resubscribe"
	Params  TaskQueryParams `json:"params"`
}

// TaskStatusUpdateEvent represents an event indicating a change in task status.
type TaskStatusUpdateEvent struct {
	Kind      string         `json:"kind"` // "status-update"
	TaskId    string         `json:"taskId"`
	ContextId string         `json:"contextId,omitempty"`
	Status    TaskStatus     `json:"status"`
	Final     bool           `json:"final,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TaskArtifactUpdateEvent represents an event indicating a new or updated artifact.
type TaskArtifactUpdateEvent struct {
	Kind      string         `json:"kind"` // "artifact-update"
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId,omitempty"`
	Artifact  Artifact       `json:"artifact"`
	Append    bool           `json:"append,omitempty"`    // Indicates if this is an append to existing artifact
	LastChunk bool           `json:"lastChunk,omitempty"` // Indicates if this is the last chunk of the artifact
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TextPart represents a plain text part of a message or artifact.
type TextPart struct {
	Type     string         `json:"type"` // "text"
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// A2ARequest represents any valid A2A request (union type).
// In Go, this is typically handled by unmarshaling into a temporary structure
// to read the 'method', then unmarshaling into the specific request type,
// or by using json.RawMessage within a wrapper struct.
// type A2ARequest any // Placeholder
