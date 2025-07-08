package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

// A2AServer wraps local agents as A2A endpoints
type A2AServer struct {
	agents     map[string]core.BaseAgent
	agentCards map[string]*a2a.AgentCard
	tasks      map[string]*TaskExecution
	mu         sync.RWMutex
}

// A2AServerConfig contains configuration for the A2A server
type A2AServerConfig struct {
	Agents     map[string]core.BaseAgent
	AgentCards map[string]*a2a.AgentCard
}

// NewA2AServer creates a new A2A server
func NewA2AServer(config A2AServerConfig) *A2AServer {
	server := &A2AServer{
		agents:     config.Agents,
		agentCards: config.AgentCards,
		tasks:      make(map[string]*TaskExecution),
	}

	if server.agents == nil {
		server.agents = make(map[string]core.BaseAgent)
	}
	if server.agentCards == nil {
		server.agentCards = make(map[string]*a2a.AgentCard)
	}

	return server
}

// TaskExecution tracks the execution state of an A2A task
type TaskExecution struct {
	ID        string
	AgentName string
	Message   *a2a.Message
	Context   context.Context
	Cancel    context.CancelFunc
	Status    *a2a.TaskStatus
	Events    chan *core.Event
	Done      chan struct{}
}

// RegisterAgent registers a new agent with the server
func (s *A2AServer) RegisterAgent(name string, agent core.BaseAgent, card *a2a.AgentCard) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[name] = agent
	s.agentCards[name] = card
}

// UnregisterAgent removes an agent from the server
func (s *A2AServer) UnregisterAgent(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.agents, name)
	delete(s.agentCards, name)
}

// GetAgent returns an agent by name
func (s *A2AServer) GetAgent(name string) (core.BaseAgent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[name]
	return agent, exists
}

// GetAgentCard returns an agent card by name
func (s *A2AServer) GetAgentCard(name string) (*a2a.AgentCard, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	card, exists := s.agentCards[name]
	return card, exists
}

// ListAgents returns all registered agent names
func (s *A2AServer) ListAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var names []string
	for name := range s.agents {
		names = append(names, name)
	}
	return names
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
	case "tasks/send":
		return s.handleSendTask(ctx, request.Params)
	case "tasks/get":
		return s.handleGetTask(ctx, request.Params)
	case "tasks/cancel":
		return s.handleCancelTask(ctx, request.Params)
	case "agents/card":
		return s.handleGetAgentCard(ctx, request.Params)
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

	// Determine agent name from metadata or ID
	agentName := "default"
	if taskParams.Metadata != nil {
		if name, ok := taskParams.Metadata["agent_name"].(string); ok {
			agentName = name
		}
	}

	// Get agent
	agent, exists := s.GetAgent(agentName)
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", agentName)
	}

	// Create task execution
	taskCtx, cancel := context.WithCancel(ctx)
	taskID := taskParams.ID
	if taskID == "" {
		taskID = generateTaskID()
	}

	task := &TaskExecution{
		ID:        taskID,
		AgentName: agentName,
		Message:   &taskParams.Message,
		Context:   taskCtx,
		Cancel:    cancel,
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
		SessionID: taskParams.SessionID,
		Status:    *task.Status,
		Metadata:  taskParams.Metadata,
	}, nil
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
	// Parse parameters
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	var request a2a.GetAgentCardRequest
	if err := json.Unmarshal(paramBytes, &request); err != nil {
		return nil, err
	}

	card, exists := s.GetAgentCard(request.AgentName)
	if !exists {
		return nil, fmt.Errorf("agent card not found: %s", request.AgentName)
	}

	return &a2a.GetAgentCardResponse{
		AgentCard: *card,
	}, nil
}

// executeAgent executes the agent with the given message
func (s *A2AServer) executeAgent(task *TaskExecution, agent core.BaseAgent) {
	defer close(task.Done)

	// Convert A2A message to ADK content
	content := s.convertA2AMessageToContent(task.Message)

	// Create invocation context
	invocationCtx := &core.InvocationContext{
		InvocationID: task.ID,
		Agent:        agent,
		UserContent:  content,
	}

	// Execute agent
	eventChan, err := agent.RunAsync(task.Context, invocationCtx)
	if err != nil {
		task.Status.State = a2a.TaskStateFailed
		task.Status.Message = &a2a.Message{
			Role: "agent",
			Parts: []a2a.Part{
				{
					Type: "text",
					Text: stringPtr(fmt.Sprintf("Agent execution failed: %v", err)),
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

// convertA2AMessageToContent converts an A2A message to ADK content
func (s *A2AServer) convertA2AMessageToContent(message *a2a.Message) *core.Content {
	var parts []core.Part

	for _, part := range message.Parts {
		if part.Text != nil {
			parts = append(parts, core.Part{
				Type: "text",
				Text: part.Text,
			})
		}
		// TODO: Handle other part types (function calls, files, etc.)
	}

	return &core.Content{
		Role:  message.Role,
		Parts: parts,
	}
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

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
