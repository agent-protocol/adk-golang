package jsonrpc2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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

	// Process the request
	resp := s.processRequest(w, req)
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
		// Validate JSON-RPC version
		if req.JSONRPC != "2.0" {
			errResp := a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
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

		resp := s.processRequest(w, req)
		if resp != nil {
			rawResp, _ := json.Marshal(resp)
			responses = append(responses, rawResp)
		}
	}

	// Return all responses
	if len(responses) > 0 {
		w.Write([]byte("["))
		for i, resp := range responses {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write(resp)
		}
		w.Write([]byte("]"))
	}
}

// processRequest handles a single JSON-RPC request
func (s *Server) processRequest(w http.ResponseWriter, req A2ARequest) *a2a.JSONRPCResponse {
	// For notifications (no ID), we don't return a response
	if req.ID == nil {
		s.processNotification(req)
		return nil
	}

	var err error
	var result any

	switch req.Method {
	case "tasks/send":
		var params a2a.TaskSendParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}
		result, err = s.handler.SendTask(&params)

	case "tasks/get":
		var params a2a.TaskQueryParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}
		result, err = s.handler.GetTask(&params)

	case "tasks/cancel":
		var params a2a.TaskIdParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}
		result, err = s.handler.CancelTask(&params)

	case "tasks/pushNotification/set":
		var params a2a.TaskPushNotificationConfig
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}
		result, err = s.handler.SetTaskPushNotification(&params)

	case "tasks/pushNotification/get":
		var params a2a.TaskIdParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}
		result, err = s.handler.GetTaskPushNotification(&params)

	case "tasks/sendSubscribe":
		// This is a streaming request
		var params a2a.TaskSendParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}

		// For streaming requests, we don't return a response immediately
		// The handler will send streaming responses
		err = s.handler.SubscribeToTask(&params, w, req.ID)
		if err != nil {
			slog.Error("Error subscribing to task", "error", err)
		}
		return nil

	case "tasks/resubscribe":
		// This is also a streaming request
		var params a2a.TaskQueryParams
		if err = json.Unmarshal(req.Params, &params); err != nil {
			return createErrorResponse(req.ID, &a2a.InvalidParamsError{
				Code:    -32602,
				Message: "Invalid parameters",
				Data:    err.Error(),
			})
		}

		err = s.handler.ResubscribeToTask(&params, w, req.ID)
		if err != nil {
			slog.Error("Error resubscribing to task", "error", err)
		}
		return nil

	default:
		return createErrorResponse(req.ID, &a2a.MethodNotFoundError{
			Code:    -32601,
			Message: "Method not found",
		})
	}

	// Handle errors
	if err != nil {
		// First check if it's already a JSONRPCError
		if jsonRPCErr, ok := err.(*a2a.JSONRPCError); ok {
			return createErrorResponse(req.ID, jsonRPCErr)
		}

		// Check for specific error types and convert them to JSONRPCError
		if taskNotFound, ok := err.(*a2a.TaskNotFoundError); ok {
			return &a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &a2a.JSONRPCError{
					Code:    taskNotFound.Code,
					Message: taskNotFound.Message,
					Data:    taskNotFound.Data,
				},
			}
		}

		if invalidParams, ok := err.(*a2a.InvalidParamsError); ok {
			return &a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &a2a.JSONRPCError{
					Code:    invalidParams.Code,
					Message: invalidParams.Message,
					Data:    invalidParams.Data,
				},
			}
		}

		if methodNotFound, ok := err.(*a2a.MethodNotFoundError); ok {
			return &a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &a2a.JSONRPCError{
					Code:    methodNotFound.Code,
					Message: methodNotFound.Message,
					Data:    methodNotFound.Data,
				},
			}
		}

		if invalidRequest, ok := err.(*a2a.InvalidRequestError); ok {
			return &a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &a2a.JSONRPCError{
					Code:    invalidRequest.Code,
					Message: invalidRequest.Message,
					Data:    invalidRequest.Data,
				},
			}
		}

		if jsonParseErr, ok := err.(*a2a.JSONParseError); ok {
			return &a2a.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &a2a.JSONRPCError{
					Code:    jsonParseErr.Code,
					Message: jsonParseErr.Message,
					Data:    jsonParseErr.Data,
				},
			}
		}

		// Default to internal error
		return &a2a.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &a2a.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			},
		}
	}

	// Create successful response
	return &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// processNotification handles JSON-RPC notifications (no response)
func (s *Server) processNotification(req A2ARequest) {
	// Process notifications by method, but don't return a response
	log.Printf("Received notification: %s", req.Method)

	switch req.Method {
	case "tasks/send":
		var params a2a.TaskSendParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			log.Printf("Error parsing notification params: %v", err)
			return
		}
		// We need to make sure task gets created in the handler
		task, err := s.handler.SendTask(&params)
		if err != nil {
			log.Printf("Error processing notification: %v", err)
		} else {
			log.Printf("Task created from notification: %s", task.ID)
		}
	case "tasks/cancel":
		var params a2a.TaskIdParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			log.Printf("Error parsing notification params: %v", err)
			return
		}
		_, err := s.handler.CancelTask(&params)
		if err != nil {
			log.Printf("Error processing notification: %v", err)
		}
	// Add other methods as needed
	default:
		log.Printf("Notification for unknown method: %s", req.Method)
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

func createErrorResponse(id any, err interface{}) *a2a.JSONRPCResponse {
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
	case *a2a.TaskNotFoundError:
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
	default:
		// If it's an unknown error type, create a generic internal error
		jsonRpcErr = &a2a.JSONRPCError{
			Code:    -32603,
			Message: "Internal error",
			Data:    fmt.Sprintf("%v", err),
		}
	}

	return &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   jsonRpcErr,
	}
}
