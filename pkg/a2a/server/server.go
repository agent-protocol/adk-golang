package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// A2AServer wraps a single local agent as an A2A endpoint
type A2AServer struct {
	agent     core.BaseAgent
	agentCard *a2a.AgentCard
	tasks     map[string]*TaskExecution
	mu        sync.RWMutex
}

// A2AServerConfig contains configuration for the A2A server
type A2AServerConfig struct {
	Agent     core.BaseAgent
	AgentCard *a2a.AgentCard
}

// NewA2AServer creates a new A2A server
func NewA2AServer(config A2AServerConfig) *A2AServer {
	if config.Agent == nil {
		panic("agent is required")
	}
	if config.AgentCard == nil {
		panic("agent card is required")
	}

	return &A2AServer{
		agent:     config.Agent,
		agentCard: config.AgentCard,
		tasks:     make(map[string]*TaskExecution),
	}
}

// NewSimpleA2AServer creates a new A2A server with an agent and agent card
func NewSimpleA2AServer(agent core.BaseAgent, agentCard *a2a.AgentCard) *A2AServer {
	return NewA2AServer(A2AServerConfig{
		Agent:     agent,
		AgentCard: agentCard,
	})
}

// TaskExecution tracks the execution state of an A2A task
type TaskExecution struct {
	ID      string
	Message *a2a.Message
	Context context.Context
	Cancel  context.CancelFunc
	Status  *a2a.TaskStatus
	Events  chan *core.Event
	Done    chan struct{}
}

// GetAgent returns the server's agent
func (s *A2AServer) GetAgent() core.BaseAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agent
}

// GetAgentCard returns the server's agent card
func (s *A2AServer) GetAgentCard() *a2a.AgentCard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agentCard := s.agentCard
	agentCard.Capabilities.Streaming = false // since we do not support streaming for now
	return agentCard
}

// ServeHTTP implements http.Handler for the A2A server
func (s *A2AServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests for JSON-RPC
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, nil, -32700, "Parse error", nil)
		return
	}

	// Parse JSON-RPC request
	var request a2a.JSONRPCRequest
	if err := json.Unmarshal(body, &request); err != nil {
		s.sendError(w, nil, -32700, "Parse error", nil)
		return
	}

	// Handle the request
	result, err := s.handleJSONRPCRequest(r.Context(), &request)
	if err != nil {
		s.sendError(w, request.ID, -32603, "Internal error", err.Error())
		return
	}

	// Send response
	response := a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleJSONRPCRequest handles a JSON-RPC request
func (s *A2AServer) handleJSONRPCRequest(ctx context.Context, request *a2a.JSONRPCRequest) (interface{}, error) {
	switch request.Method {
	case "agents/card":
		return s.handleGetAgentCard(ctx, request.Params)
	// A2A-compliant method names
	case "message/send":
		return s.handleSendMessage(ctx, request.Params)
	case "message/stream":
		return s.handleStreamMessage(ctx, request.Params)
	case "tasks/get":
		return s.handleGetTask(ctx, request.Params)
	case "tasks/cancel":
		return s.handleCancelTask(ctx, request.Params)
	case "tasks/pushNotificationConfig/set":
		// These methods are not implemented in the A2A server
		return nil, fmt.Errorf("method not implemented: %s", request.Method)

	case "tasks/pushNotificationConfig/get":
		// These methods are not implemented in the A2A server
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	case "tasks/pushNotificationConfig/list":
		// These methods are not implemented in the A2A server
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	case "tasks/pushNotificationConfig/delete":
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	case "tasks/resubscribe":
		// This method is not implemented in the A2A server
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	case "agent/authenticatedExtendedCard":
		// This method is not implemented in the A2A server
		return nil, fmt.Errorf("method not implemented: %s", request.Method)
	default:
		return nil, fmt.Errorf("method not found: %s", request.Method)
	}
}

// handleSendTask handles the tasks/send method
func (s *A2AServer) handleSendTask(ctx context.Context, params any) (interface{}, error) {
	// Parse parameters
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var taskParams a2a.TaskSendParams
	if err := json.Unmarshal(paramBytes, &taskParams); err != nil {
		return nil, err
	}

	// Get the single agent
	agent := s.GetAgent()
	if agent == nil {
		return nil, fmt.Errorf("no agent configured")
	}

	// Create task execution
	taskCtx, cancel := context.WithCancel(ctx)
	taskID := taskParams.ID
	if taskID == "" {
		taskID = generateTaskID()
	}

	task := &TaskExecution{
		ID:      taskID,
		Message: &taskParams.Message,
		Context: taskCtx,
		Cancel:  cancel,
		Status: &a2a.TaskStatus{
			State:   a2a.TaskStateWorking,
			Message: nil,
		},
		Events: make(chan *core.Event, 100),
		Done:   make(chan struct{}),
	}

	// Store task
	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	// Start agent execution in background
	go s.executeAgent(task, agent)

	return &a2a.Task{
		ID:        taskID,
		ContextID: taskParams.SessionID,
		Status:    *task.Status,
		Metadata:  taskParams.Metadata,
	}, nil
}

// handleSendMessage handles the A2A-compliant message/send method
func (s *A2AServer) handleSendMessage(ctx context.Context, params any) (interface{}, error) {
	// Parse parameters as MessageSendParams
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var messageSendParams a2a.MessageSendParams
	if err := json.Unmarshal(paramBytes, &messageSendParams); err != nil {
		return nil, err
	}

	// Validate required messageId
	if messageSendParams.Message.MessageID == "" {
		return nil, fmt.Errorf("messageId is required in message")
	}

	// Convert MessageSendParams to TaskSendParams for internal processing
	taskParams := s.convertMessageSendParamsToTaskSendParams(&messageSendParams)

	// Use existing task sending logic
	return s.handleSendTask(ctx, taskParams)
}

// handleStreamMessage handles the A2A-compliant message/stream method
func (s *A2AServer) handleStreamMessage(ctx context.Context, params any) (interface{}, error) {
	// Parse parameters as MessageSendParams (same as message/send)
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var messageSendParams a2a.MessageSendParams
	if err := json.Unmarshal(paramBytes, &messageSendParams); err != nil {
		return nil, err
	}

	// Validate required messageId
	if messageSendParams.Message.MessageID == "" {
		return nil, fmt.Errorf("messageId is required in message")
	}

	// Convert MessageSendParams to TaskSendParams for internal processing
	taskParams := s.convertMessageSendParamsToTaskSendParams(&messageSendParams)

	// For streaming, we would typically handle this differently
	// For now, delegate to the task sending logic
	// TODO: Implement actual streaming response with SSE
	return s.handleSendTask(ctx, taskParams)
}

// convertMessageSendParamsToTaskSendParams converts A2A MessageSendParams to legacy TaskSendParams
func (s *A2AServer) convertMessageSendParamsToTaskSendParams(msgParams *a2a.MessageSendParams) *a2a.TaskSendParams {
	// Generate task ID from message ID or create new one
	taskID := ""
	if msgParams.Message.TaskID != nil {
		taskID = *msgParams.Message.TaskID
	}
	if taskID == "" {
		taskID = generateTaskID()
	}

	taskParams := &a2a.TaskSendParams{
		ID:       taskID,
		Message:  msgParams.Message,
		Metadata: msgParams.Metadata,
	}

	// Convert configuration options if present
	if msgParams.Configuration != nil {
		taskParams.HistoryLength = msgParams.Configuration.HistoryLength
		taskParams.PushNotification = msgParams.Configuration.PushNotificationConfig
	}

	return taskParams
}

// handleGetTask handles the tasks/get method
func (s *A2AServer) handleGetTask(ctx context.Context, params any) (interface{}, error) {
	// Parse parameters
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var queryParams a2a.TaskQueryParams
	if err := json.Unmarshal(paramBytes, &queryParams); err != nil {
		return nil, err
	}

	s.mu.RLock()
	task, exists := s.tasks[queryParams.ID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task not found: %s", queryParams.ID)
	}

	return &a2a.Task{
		ID:       task.ID,
		Status:   *task.Status,
		Metadata: queryParams.Metadata,
	}, nil
}

// handleCancelTask handles the tasks/cancel method
func (s *A2AServer) handleCancelTask(ctx context.Context, params any) (interface{}, error) {
	// Parse parameters
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var idParams a2a.TaskIdParams
	if err := json.Unmarshal(paramBytes, &idParams); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[idParams.ID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", idParams.ID)
	}

	task.Cancel()
	task.Status.State = a2a.TaskStateCanceled
	close(task.Done)
	delete(s.tasks, idParams.ID)

	return &a2a.Task{
		ID: idParams.ID,
		Status: a2a.TaskStatus{
			State: a2a.TaskStateCanceled,
		},
		Metadata: idParams.Metadata,
	}, nil
}

// handleGetAgentCard handles the agents/card method
func (s *A2AServer) handleGetAgentCard(ctx context.Context, params any) (interface{}, error) {
	// The agents/card method returns the agent's own card
	// According to A2A spec, no parameters are needed
	card := s.GetAgentCard()
	if card == nil {
		return nil, fmt.Errorf("no agent card configured")
	}
	return card, nil
}

// executeAgent executes the agent with the given message
func (s *A2AServer) executeAgent(task *TaskExecution, agent core.BaseAgent) {
	defer close(task.Done)

	// Convert A2A message to ADK content
	content := a2a.ConvertA2AMessageToContent(task.Message)

	// Create invocation context
	invocationCtx := &core.InvocationContext{
		Context:      task.Context,
		InvocationID: task.ID,
		Agent:        agent,
		UserContent:  content,
	}

	// Execute agent
	eventChan, err := agent.RunAsync(invocationCtx)
	if err != nil {
		task.Status.State = a2a.TaskStateFailed
		task.Status.Message = &a2a.Message{
			Role: "agent",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: ptr.Ptr(fmt.Sprintf("Agent execution failed: %v", err)),
				},
			},
		}
		return
	}

	// Forward events
	for event := range eventChan {
		select {
		case task.Events <- event:
		case <-task.Context.Done():
			return
		}
	}

	// Mark as completed
	task.Status.State = a2a.TaskStateCompleted
}

// sendError sends a JSON-RPC error response
func (s *A2AServer) sendError(w http.ResponseWriter, id any, code int, message string, data any) {
	response := a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &a2a.JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors are still 200 OK
	json.NewEncoder(w).Encode(response)
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
