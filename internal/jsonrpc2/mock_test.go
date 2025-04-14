package jsonrpc2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// MockTaskHandler is a mock implementation of the TaskHandler interface for testing
type MockTaskHandler struct {
	mu             sync.RWMutex
	tasks          map[string]*a2a.Task
	notifications  map[string]*a2a.TaskPushNotificationConfig
	streaming      map[string]bool
	shouldFail     bool
	failMethod     string
	streamingDelay time.Duration
}

// NewMockTaskHandler creates a new instance of MockTaskHandler
func NewMockTaskHandler() *MockTaskHandler {
	return &MockTaskHandler{
		tasks:         make(map[string]*a2a.Task),
		notifications: make(map[string]*a2a.TaskPushNotificationConfig),
		streaming:     make(map[string]bool),
	}
}

// SetTaskToFail configures the mock to fail for a specific method
func (h *MockTaskHandler) SetTaskToFail(method string) {
	h.shouldFail = true
	h.failMethod = method
}

// SetStreamingDelay sets a delay for streaming responses
func (h *MockTaskHandler) SetStreamingDelay(delay time.Duration) {
	h.streamingDelay = delay
}

// SendTask implements TaskHandler.SendTask
func (h *MockTaskHandler) SendTask(params *a2a.TaskSendParams) (*a2a.Task, error) {
	if h.shouldFail && h.failMethod == "tasks/send" {
		return nil, &a2a.InvalidParamsError{
			Code:    -32602,
			Message: "Invalid parameters",
			Data:    "Testing error scenario",
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	task := &a2a.Task{
		ID:        params.ID,
		SessionID: params.SessionID,
		Status: a2a.TaskStatus{
			State:     a2a.TaskStateSubmitted,
			Timestamp: &now,
		},
		Artifacts: []a2a.Artifact{},
		Metadata:  params.Metadata,
	}
	h.tasks[params.ID] = task
	return task, nil
}

// GetTask implements TaskHandler.GetTask
func (h *MockTaskHandler) GetTask(params *a2a.TaskQueryParams) (*a2a.Task, error) {
	if h.shouldFail && h.failMethod == "tasks/get" {
		return nil, &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	task, exists := h.tasks[params.ID]
	if !exists {
		return nil, &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	return task, nil
}

// CancelTask implements TaskHandler.CancelTask
func (h *MockTaskHandler) CancelTask(params *a2a.TaskIdParams) (*a2a.Task, error) {
	if h.shouldFail && h.failMethod == "tasks/cancel" {
		return nil, &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	task, exists := h.tasks[params.ID]
	if !exists {
		return nil, &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	now := time.Now()
	task.Status.State = a2a.TaskStateCanceled
	task.Status.Timestamp = &now
	return task, nil
}

// SetTaskPushNotification implements TaskHandler.SetTaskPushNotification
func (h *MockTaskHandler) SetTaskPushNotification(params *a2a.TaskPushNotificationConfig) (*a2a.TaskPushNotificationConfig, error) {
	if h.shouldFail && h.failMethod == "tasks/pushNotification/set" {
		return nil, &a2a.InvalidParamsError{
			Code:    -32602,
			Message: "Invalid parameters",
			Data:    "Testing error scenario",
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.notifications[params.ID] = params
	return params, nil
}

// GetTaskPushNotification implements TaskHandler.GetTaskPushNotification
func (h *MockTaskHandler) GetTaskPushNotification(params *a2a.TaskIdParams) (*a2a.TaskPushNotificationConfig, error) {
	if h.shouldFail && h.failMethod == "tasks/pushNotification/get" {
		return nil, &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	config, exists := h.notifications[params.ID]
	if !exists {
		return nil, &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	return config, nil
}

// SubscribeToTask implements TaskHandler.SubscribeToTask
func (h *MockTaskHandler) SubscribeToTask(params *a2a.TaskSendParams, w http.ResponseWriter, id any) error {
	if h.shouldFail && h.failMethod == "tasks/sendSubscribe" {
		return &a2a.InvalidParamsError{
			Code:    -32602,
			Message: "Invalid parameters",
			Data:    "Testing error scenario",
		}
	}

	h.mu.Lock()
	h.streaming[params.ID] = true
	h.mu.Unlock()

	// Mark the writer for streaming (we need to use Flusher interface)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Set proper headers for streaming
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial task created message
	task, err := h.SendTask(params)
	if err != nil {
		return err
	}

	// Convert task to a status update
	initialUpdate := a2a.TaskStatusUpdateEvent{
		ID: task.ID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateSubmitted,
		},
		Final: false,
	}

	// Create streaming response
	resp := &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  initialUpdate,
	}

	// Serialize and write
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	flusher.Flush()

	// Simulate processing delay
	if h.streamingDelay > 0 {
		time.Sleep(h.streamingDelay)
	}

	// Send simulated "working" update
	workingUpdate := a2a.TaskStatusUpdateEvent{
		ID: task.ID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateWorking,
		},
		Final: false,
	}

	resp = &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  workingUpdate,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	flusher.Flush()

	// Simulate processing delay
	if h.streamingDelay > 0 {
		time.Sleep(h.streamingDelay)
	}

	// Send a simulated artifact
	artifact := a2a.Artifact{
		Name: stringPtr("Test Artifact"),
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr("This is a test artifact from streaming"),
			},
		},
	}

	artifactUpdate := a2a.TaskArtifactUpdateEvent{
		ID:       task.ID,
		Artifact: artifact,
	}

	resp = &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  artifactUpdate,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	flusher.Flush()

	// Simulate processing delay
	if h.streamingDelay > 0 {
		time.Sleep(h.streamingDelay)
	}

	// Send final "completed" update
	completedUpdate := a2a.TaskStatusUpdateEvent{
		ID: task.ID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCompleted,
		},
		Final: true,
	}

	resp = &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  completedUpdate,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	flusher.Flush()

	return nil
}

// ResubscribeToTask implements TaskHandler.ResubscribeToTask
func (h *MockTaskHandler) ResubscribeToTask(params *a2a.TaskQueryParams, w http.ResponseWriter, id any) error {
	if h.shouldFail && h.failMethod == "tasks/resubscribe" {
		return &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	h.mu.RLock()
	_, exists := h.tasks[params.ID]
	h.mu.RUnlock()

	if !exists {
		return &a2a.TaskNotFoundError{
			Code:    -32001,
			Message: "Task not found",
		}
	}

	// Mark the writer for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Set proper headers for streaming
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send a "working" update
	workingUpdate := a2a.TaskStatusUpdateEvent{
		ID: params.ID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateWorking,
		},
		Final: false,
	}

	resp := &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  workingUpdate,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	flusher.Flush()

	// Simulate processing delay
	if h.streamingDelay > 0 {
		time.Sleep(h.streamingDelay)
	}

	// Send a simulated artifact
	artifact := a2a.Artifact{
		Name: stringPtr("Resubscribed Artifact"),
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: stringPtr("This is a test artifact from resubscription"),
			},
		},
	}

	artifactUpdate := a2a.TaskArtifactUpdateEvent{
		ID:       params.ID,
		Artifact: artifact,
	}

	resp = &a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  artifactUpdate,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	flusher.Flush()

	return nil
}

// Helper to convert a string to a pointer for optional fields
func stringPtr(s string) *string {
	return &s
}

// Test utilities for making JSON-RPC requests
func makeRequest(method string, params interface{}, id any) ([]byte, error) {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}

	if id != nil {
		req["id"] = id
	}

	return json.Marshal(req)
}
