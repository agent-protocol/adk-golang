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
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Parts       []Part         `json:"parts"` // Part is a union type, using any or specific struct with Type field
	Index       int            `json:"index,omitempty"`
	Append      *bool          `json:"append,omitempty"`
	LastChunk   *bool          `json:"lastChunk,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// AuthenticationInfo holds authentication details.
type AuthenticationInfo struct {
	Schemes     []string `json:"schemes"`
	Credentials *string  `json:"credentials,omitempty"`
	// Note: additionalProperties: {} allows arbitrary fields, not directly mapped in Go struct easily.
	// Consider using map[string]any or custom marshaling if needed.
}

// PushNotificationNotSupportedError indicates push notifications are not supported.
type PushNotificationNotSupportedError struct {
	Code    int    `json:"code"`    // Should be -32003
	Message string `json:"message"` // Should be "Push Notification is not supported"
	Data    any    `json:"data"`    // Should be null
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

// InternalError represents a generic internal JSON-RPC error.
type InternalError struct {
	Code    int    `json:"code"`    // Should be -32603
	Message string `json:"message"` // Should be "Internal error"
	Data    any    `json:"data,omitempty"`
}

// InvalidParamsError represents a JSON-RPC invalid parameters error.
type InvalidParamsError struct {
	Code    int    `json:"code"`    // Should be -32602
	Message string `json:"message"` // Should be "Invalid parameters"
	Data    any    `json:"data,omitempty"`
}

// InvalidRequestError represents a JSON-RPC invalid request error.
type InvalidRequestError struct {
	Code    int    `json:"code"`    // Should be -32600
	Message string `json:"message"` // Should be "Request payload validation error"
	Data    any    `json:"data,omitempty"`
}

// JSONParseError represents a JSON-RPC parse error.
type JSONParseError struct {
	Code    int    `json:"code"`    // Should be -32700
	Message string `json:"message"` // Should be "Invalid JSON payload"
	Data    any    `json:"data,omitempty"`
}

// JSONRPCError represents a standard JSON-RPC error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
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
	Role     string         `json:"role"`  // "user" or "agent"
	Parts    []Part         `json:"parts"` // Part is a union type
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MethodNotFoundError represents a JSON-RPC method not found error.
type MethodNotFoundError struct {
	Code    int    `json:"code"`    // Should be -32601
	Message string `json:"message"` // Should be "Method not found"
	Data    any    `json:"data"`    // Should be null
}

// PushNotificationConfig defines the configuration for push notifications.
type PushNotificationConfig struct {
	URL            string              `json:"url"`
	Token          *string             `json:"token,omitempty"`
	Authentication *AuthenticationInfo `json:"authentication,omitempty"`
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

// SendTaskRequest is a JSON-RPC request to send a message/start a task.
type SendTaskRequest struct {
	JSONRPC string         `json:"jsonrpc"` // "2.0"
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"` // "tasks/send"
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
	SessionID *string        `json:"sessionId,omitempty"`
	Status    TaskStatus     `json:"status"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TaskPushNotificationConfig associates a task ID with its push notification settings.
type TaskPushNotificationConfig struct {
	ID                     string                 `json:"id"`
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
}

// TaskNotCancelableError indicates a task cannot be canceled (e.g., already completed).
type TaskNotCancelableError struct {
	Code    int    `json:"code"`    // Should be -32002
	Message string `json:"message"` // Should be "Task cannot be canceled"
	Data    any    `json:"data"`    // Should be null
}

// TaskNotFoundError indicates the requested task ID was not found.
type TaskNotFoundError struct {
	Code    int    `json:"code"`    // Should be -32001
	Message string `json:"message"` // Should be "Task not found"
	Data    any    `json:"data"`    // Should be null
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
	ID       string         `json:"id"`
	Status   TaskStatus     `json:"status"`
	Final    bool           `json:"final,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskArtifactUpdateEvent represents an event indicating a new or updated artifact.
type TaskArtifactUpdateEvent struct {
	ID       string         `json:"id"`
	Artifact Artifact       `json:"artifact"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TextPart represents a plain text part of a message or artifact.
type TextPart struct {
	Type     string         `json:"type"` // "text"
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// UnsupportedOperationError indicates the requested operation is not supported by the agent.
type UnsupportedOperationError struct {
	Code    int    `json:"code"`    // Should be -32004
	Message string `json:"message"` // Should be "This operation is not supported"
	Data    any    `json:"data"`    // Should be null
}

// A2ARequest represents any valid A2A request (union type).
// In Go, this is typically handled by unmarshaling into a temporary structure
// to read the 'method', then unmarshaling into the specific request type,
// or by using json.RawMessage within a wrapper struct.
// type A2ARequest any // Placeholder
