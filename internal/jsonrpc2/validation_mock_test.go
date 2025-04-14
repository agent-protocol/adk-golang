package jsonrpc2

import (
	"net/http"
	"sync"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// ValidationMockTaskHandler extends MockTaskHandler with additional testing capabilities
type ValidationMockTaskHandler struct {
	*MockTaskHandler
	errorMap map[string]error
	mu       sync.RWMutex
}

// NewValidationMockTaskHandler creates a new instance of EnhancedMockTaskHandler
func NewValidationMockTaskHandler() *ValidationMockTaskHandler {
	return &ValidationMockTaskHandler{
		MockTaskHandler: NewMockTaskHandler(),
		errorMap:        make(map[string]error),
	}
}

// SetErrorToReturn configures the mock to return a specific error for a method
func (h *ValidationMockTaskHandler) SetErrorToReturn(method string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errorMap[method] = err
}

// GetErrorToReturn retrieves the error set for a specific method
func (h *ValidationMockTaskHandler) GetErrorToReturn(method string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.errorMap[method]
}

// SendTask overrides MockTaskHandler.SendTask to handle custom errors
func (h *ValidationMockTaskHandler) SendTask(params *a2a.TaskSendParams) (*a2a.Task, error) {
	if err := h.GetErrorToReturn("tasks/send"); err != nil {
		return nil, err
	}
	return h.MockTaskHandler.SendTask(params)
}

// GetTask overrides MockTaskHandler.GetTask to handle custom errors
func (h *ValidationMockTaskHandler) GetTask(params *a2a.TaskQueryParams) (*a2a.Task, error) {
	if err := h.GetErrorToReturn("tasks/get"); err != nil {
		return nil, err
	}
	return h.MockTaskHandler.GetTask(params)
}

// CancelTask overrides MockTaskHandler.CancelTask to handle custom errors
func (h *ValidationMockTaskHandler) CancelTask(params *a2a.TaskIdParams) (*a2a.Task, error) {
	if err := h.GetErrorToReturn("tasks/cancel"); err != nil {
		return nil, err
	}
	return h.MockTaskHandler.CancelTask(params)
}

// SetTaskPushNotification overrides MockTaskHandler.SetTaskPushNotification to handle custom errors
func (h *ValidationMockTaskHandler) SetTaskPushNotification(params *a2a.TaskPushNotificationConfig) (*a2a.TaskPushNotificationConfig, error) {
	if err := h.GetErrorToReturn("tasks/pushNotification/set"); err != nil {
		return nil, err
	}
	return h.MockTaskHandler.SetTaskPushNotification(params)
}

// GetTaskPushNotification overrides MockTaskHandler.GetTaskPushNotification to handle custom errors
func (h *ValidationMockTaskHandler) GetTaskPushNotification(params *a2a.TaskIdParams) (*a2a.TaskPushNotificationConfig, error) {
	if err := h.GetErrorToReturn("tasks/pushNotification/get"); err != nil {
		return nil, err
	}
	return h.MockTaskHandler.GetTaskPushNotification(params)
}

// SubscribeToTask overrides MockTaskHandler.SubscribeToTask to handle custom errors
func (h *ValidationMockTaskHandler) SubscribeToTask(params *a2a.TaskSendParams, w http.ResponseWriter, id any) error {
	if err := h.GetErrorToReturn("tasks/sendSubscribe"); err != nil {
		return err
	}
	return h.MockTaskHandler.SubscribeToTask(params, w, id)
}

// ResubscribeToTask overrides MockTaskHandler.ResubscribeToTask to handle custom errors
func (h *ValidationMockTaskHandler) ResubscribeToTask(params *a2a.TaskQueryParams, w http.ResponseWriter, id any) error {
	if err := h.GetErrorToReturn("tasks/resubscribe"); err != nil {
		return err
	}
	return h.MockTaskHandler.ResubscribeToTask(params, w, id)
}
