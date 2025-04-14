package a2a

import "fmt"

// JSONRPCError represents a standard JSON-RPC error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
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

// MethodNotFoundError represents a JSON-RPC method not found error.
type MethodNotFoundError struct {
	Code    int    `json:"code"`    // Should be -32601
	Message string `json:"message"` // Should be "Method not found"
	Data    any    `json:"data"`    // Should be null
}

// TaskNotFoundError indicates the requested task ID was not found.
type TaskNotFoundError struct {
	Code    int    `json:"code"`    // Should be -32001
	Message string `json:"message"` // Should be "Task not found"
	Data    any    `json:"data"`    // Should be null
}

// TaskNotCancelableError indicates a task cannot be canceled (e.g., already completed).
type TaskNotCancelableError struct {
	Code    int    `json:"code"`    // Should be -32002
	Message string `json:"message"` // Should be "Task cannot be canceled"
	Data    any    `json:"data"`    // Should be null
}

// PushNotificationNotSupportedError indicates push notifications are not supported.
type PushNotificationNotSupportedError struct {
	Code    int    `json:"code"`    // Should be -32003
	Message string `json:"message"` // Should be "Push Notification is not supported"
	Data    any    `json:"data"`    // Should be null
}

// UnsupportedOperationError indicates the requested operation is not supported by the agent.
type UnsupportedOperationError struct {
	Code    int    `json:"code"`    // Should be -32004
	Message string `json:"message"` // Should be "This operation is not supported"
	Data    any    `json:"data"`    // Should be null
}

// Error methods for the error types to implement Go's error interface

// Error implements the error interface for JSONRPCError
func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for TaskNotFoundError
func (e *TaskNotFoundError) Error() string {
	return fmt.Sprintf("Task not found error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for InvalidParamsError
func (e *InvalidParamsError) Error() string {
	return fmt.Sprintf("Invalid parameters error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for MethodNotFoundError
func (e *MethodNotFoundError) Error() string {
	return fmt.Sprintf("Method not found error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for InvalidRequestError
func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("Invalid request error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for JSONParseError
func (e *JSONParseError) Error() string {
	return fmt.Sprintf("JSON parse error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for InternalError
func (e *InternalError) Error() string {
	return fmt.Sprintf("Internal error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for TaskNotCancelableError
func (e *TaskNotCancelableError) Error() string {
	return fmt.Sprintf("Task not cancelable error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for PushNotificationNotSupportedError
func (e *PushNotificationNotSupportedError) Error() string {
	return fmt.Sprintf("Push notification not supported error %d: %s", e.Code, e.Message)
}

// Error implements the error interface for UnsupportedOperationError
func (e *UnsupportedOperationError) Error() string {
	return fmt.Sprintf("Unsupported operation error %d: %s", e.Code, e.Message)
}
