package jsonrpc2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// A2ARequest is a union type of all possible A2A JSON-RPC requests
type A2ARequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// TaskHandler defines the interface for handling A2A protocol operations
type TaskHandler interface {
	// Core task methods
	SendTask(params *a2a.TaskSendParams) (*a2a.Task, error)
	GetTask(params *a2a.TaskQueryParams) (*a2a.Task, error)
	CancelTask(params *a2a.TaskIdParams) (*a2a.Task, error)

	// Push notification methods
	SetTaskPushNotification(params *a2a.TaskPushNotificationConfig) (*a2a.TaskPushNotificationConfig, error)
	GetTaskPushNotification(params *a2a.TaskIdParams) (*a2a.TaskPushNotificationConfig, error)

	// Streaming support
	SubscribeToTask(params *a2a.TaskSendParams, w http.ResponseWriter, id any) error
	ResubscribeToTask(params *a2a.TaskQueryParams, w http.ResponseWriter, id any) error
}

// Server represents a JSON-RPC 2.0 server for A2A protocol
type Server struct {
	handler TaskHandler
}

// NewServer creates a new A2A JSON-RPC 2.0 server with the given task handler
func NewServer(handler TaskHandler) *Server {
	return &Server{
		handler: handler,
	}
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Determine if this is a batch request
	var isBatch bool
	for _, c := range body {
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		isBatch = c == '['
		break
	}

	w.Header().Set("Content-Type", "application/json")

	if isBatch {
		s.handleBatchRequest(w, body)
	} else {
		s.handleSingleRequest(w, body)
	}
}

// handleSingleRequest processes a single JSON-RPC request
func (s *Server) handleSingleRequest(w http.ResponseWriter, body []byte) {
	var req A2ARequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, nil, &a2a.JSONParseError{
			Code:    -32700,
			Message: "Invalid JSON payload",
		})
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		writeError(w, req.ID, &a2a.InvalidRequestError{
			Code:    -32600,
			Message: "Request payload validation error",
			Data:    "jsonrpc must be '2.0'",
		})
		return
	}

	// Process the request/notification
	resp := s.processRequest(w, req) // processRequest now handles both

	// Only send response if it's not a notification and not a streaming request handled separately
	if resp != nil {
		json.NewEncoder(w).Encode(resp)
	}
}

// handleBatchRequest processes a batch of JSON-RPC requests
func (s *Server) handleBatchRequest(w http.ResponseWriter, body []byte) {
	var requests []A2ARequest
	if err := json.Unmarshal(body, &requests); err != nil {
		writeError(w, nil, &a2a.JSONParseError{
			Code:    -32700,
			Message: "Invalid JSON payload",
		})
		return
	}

	// Empty batch is invalid
	if len(requests) == 0 {
		writeError(w, nil, &a2a.InvalidRequestError{
			Code:    -32600,
			Message: "Request payload validation error",
			Data:    "Batch request cannot be empty",
		})
		return
	}

	// Process each request and collect responses
	responses := make([]json.RawMessage, 0, len(requests))
	for _, req := range requests {
		// Validate JSON-RPC version early for batch items
		if req.JSONRPC != "2.0" {
			errResp := a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID, // Use the request's ID if available
				Error: &a2a.JSONRPCError{
					Code:    -32600,
					Message: "Request payload validation error",
					Data:    "jsonrpc must be '2.0'",
				},
			}
			rawResp, _ := json.Marshal(errResp)
			responses = append(responses, rawResp)
			continue
		}

		// Process the request/notification
		resp := s.processRequest(w, req) // processRequest handles both

		// Only add response to batch if it's not a notification and not a streaming request
		if resp != nil {
			rawResp, err := json.Marshal(resp)
			// Handle potential marshaling error for the individual response
			if err != nil {
				slog.Error("Error marshaling batch response item", "error", err, "request_id", req.ID)
				// Optionally add a generic error response for this item
				errResp := a2a.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &a2a.JSONRPCError{Code: -32603, Message: "Internal error"},
				}
				rawResp, _ = json.Marshal(errResp) // Marshal the error response
				responses = append(responses, rawResp)
			} else {
				responses = append(responses, rawResp)
			}
		}
	}

	// Return all collected responses (if any)
	if len(responses) > 0 {
		w.Write([]byte("["))
		for i, resp := range responses {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write(resp)
		}
		w.Write([]byte("]"))
	} else {
		// According to JSON-RPC 2.0 spec, if a batch consists entirely of notifications,
		// the server MUST NOT return an empty Array. It MUST NOT return any response.
		// So, we write nothing here.
	}
}

// _dispatchMethodCall handles the core logic of unmarshaling params and calling the handler method
// for non-streaming requests/notifications.
func (s *Server) _dispatchMethodCall(req A2ARequest) (any, error) {
	var params any
	var handlerCall func() (any, error)
	var err error

	switch req.Method {
	case "tasks/send":
		var p a2a.TaskSendParams
		params = &p
		handlerCall = func() (any, error) { return s.handler.SendTask(&p) }
	case "tasks/get":
		var p a2a.TaskQueryParams
		params = &p
		handlerCall = func() (any, error) {
			if p.ID == "" {
				return nil, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: "Task ID cannot be empty"}
			}
			return s.handler.GetTask(&p)
		}
	case "tasks/cancel":
		var p a2a.TaskIdParams
		params = &p
		handlerCall = func() (any, error) {
			if p.ID == "" {
				return nil, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: "Task ID cannot be empty"}
			}
			return s.handler.CancelTask(&p)
		}
	case "tasks/pushNotification/set":
		var p a2a.TaskPushNotificationConfig
		params = &p
		handlerCall = func() (any, error) { return s.handler.SetTaskPushNotification(&p) }
	case "tasks/pushNotification/get":
		var p a2a.TaskIdParams
		params = &p
		handlerCall = func() (any, error) {
			if p.ID == "" {
				return nil, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: "Task ID cannot be empty"}
			}
			return s.handler.GetTaskPushNotification(&p)
		}
	default:
		return nil, &a2a.MethodNotFoundError{Code: -32601, Message: "Method not found"}
	}

	// Unmarshal params
	if err = json.Unmarshal(req.Params, params); err != nil {
		return nil, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: err.Error()}
	}

	// Call the handler
	return handlerCall()
}

// processRequest handles a single JSON-RPC request or notification.
// It returns a response object for requests, nil for notifications or handled streaming requests.
func (s *Server) processRequest(w http.ResponseWriter, req A2ARequest) *a2a.JSONRPCResponse {
	var result any
	var err error

	// Handle streaming methods separately as they need the ResponseWriter
	switch req.Method {
	case "tasks/sendSubscribe":
		var params a2a.TaskSendParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			// Need ID for error response
			if req.ID == nil {
				slog.Error("Cannot process streaming notification", "method", req.Method, "error", err)
				return nil // Cannot send error response for notification
			}
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: err.Error()})
		}
		// Streaming requests require an ID
		if req.ID == nil {
			slog.Error("Streaming method requires an ID", "method", req.Method)
			// Technically invalid, but can't return error without ID
			return nil
		}
		err = s.handler.SubscribeToTask(&params, w, req.ID)
		if err != nil {
			slog.Error("Error subscribing to task", "error", err, "request_id", req.ID)
			// Attempt to send an error response back if possible, although the stream might be compromised
			return createErrorResponse(req.ID, err)
		}
		return nil // Handler manages the streaming response

	case "tasks/resubscribe":
		var params a2a.TaskQueryParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			if req.ID == nil {
				slog.Error("Cannot process streaming notification", "method", req.Method, "error", err)
				return nil
			}
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: err.Error()})
		}
		if req.ID == nil {
			slog.Error("Streaming method requires an ID", "method", req.Method)
			return nil
		}
		if params.ID == "" {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{Code: -32602, Message: "Invalid parameters", Data: "Task ID cannot be empty"})
		}
		err = s.handler.ResubscribeToTask(&params, w, req.ID)
		if err != nil {
			slog.Error("Error resubscribing to task", "error", err, "request_id", req.ID)
			return createErrorResponse(req.ID, err)
		}
		return nil // Handler manages the streaming response

	default:
		// Handle non-streaming methods using the helper
		result, err = s._dispatchMethodCall(req)
	}

	// --- Response Handling ---

	// If it's a notification (no ID), log errors but don't return a response
	if req.ID == nil {
		if err != nil {
			// Log the error encountered while processing the notification
			slog.Error("Error processing notification", "method", req.Method, "error", err)
		} else {
			// Optional: Log successful notification processing
			slog.Debug("Processed notification", "method", req.Method)
		}
		return nil // No response for notifications
	}

	// If it's a request (has ID), handle errors or create success response
	if err != nil {
		// Use createErrorResponse which handles specific A2A error types
		return createErrorResponse(req.ID, err)
	}

	// Create successful response
	return &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// Helper functions
func writeError(w http.ResponseWriter, id any, err interface{}) {
	var jsonRpcErr *a2a.JSONRPCError

	switch e := err.(type) {
	case *a2a.JSONRPCError:
		jsonRpcErr = e
	case *a2a.JSONParseError:
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	case *a2a.InvalidRequestError:
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	case *a2a.MethodNotFoundError:
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	case *a2a.InvalidParamsError:
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	case *a2a.InternalError:
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	case *a2a.TaskNotCancelableError: // Add missing case for TaskNotCancelableError
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	default:
		// If it's an unknown error type, create a generic internal error
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    -32603,
			Message: "Internal error",
			Data:    errors.New("unknown error type").Error(),
		}
	}

	resp := a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   jsonRpcErr,
	}
	json.NewEncoder(w).Encode(resp)
}

func createErrorResponse(id any, err error) *a2a.JSONRPCResponse {
	var jsonRpcErr *a2a.JSONRPCError

	// Convert specific A2A error types to JSONRPCError
	switch e := err.(type) {
	case *a2a.TaskNotFoundError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.InvalidParamsError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.MethodNotFoundError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.InvalidRequestError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.JSONParseError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.InternalError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.TaskNotCancelableError:
		jsonRpcErr = &a2a.JSONRPCError{Code: e.Code, Message: e.Message, Data: e.Data}
	case *a2a.JSONRPCError: // Already in the correct format
		jsonRpcErr = e
	default:
		// Fallback for unexpected error types
		slog.Error("createErrorResponse received unexpected error type", "error", err)
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    -32603, // Internal Error
			Message: "Internal error",
			Data:    fmt.Sprintf("Unhandled error: %v", err),
		}
	}

	return &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   jsonRpcErr,
	}
}
