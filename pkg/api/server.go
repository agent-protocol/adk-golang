// Package api provides HTTP API server implementation equivalent to Python's fast_api.py
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/cors"

	"github.com/agent-protocol/adk-golang/pkg/cli/utils"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/runners"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

// ServerConfig contains configuration for the API server
type ServerConfig struct {
	Host               string
	Port               int
	AgentsDir          string
	SessionServiceURI  string
	ArtifactServiceURI string
	MemoryServiceURI   string
	EvalStorageURI     string
	AllowOrigins       []string
	TraceToCloud       bool
	A2AEnabled         bool
	LogLevel           string
}

// Server represents the HTTP API server
type Server struct {
	config          *ServerConfig
	router          *http.ServeMux
	sessionService  core.SessionService
	artifactService core.ArtifactService
	memoryService   core.MemoryService
	agentLoader     *utils.AgentLoader
	runnerCache     map[string]*runners.RunnerImpl
	upgrader        websocket.Upgrader
}

// AgentRunRequest represents a request to run an agent
type AgentRunRequest struct {
	AppName    string        `json:"app_name"`
	UserID     string        `json:"user_id"`
	SessionID  string        `json:"session_id"`
	NewMessage *core.Content `json:"new_message"`
	Streaming  bool          `json:"streaming,omitempty"`
}

// CreateSessionRequest represents a request to create a session
type CreateSessionRequest struct {
	State  map[string]any `json:"state,omitempty"`
	Events []*core.Event  `json:"events,omitempty"`
}

// AddSessionToEvalSetRequest represents a request to add session to eval set
type AddSessionToEvalSetRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	EvalID    string `json:"eval_id"`
}

// ListSessionsResponse represents the response for listing sessions
type ListSessionsResponse struct {
	Sessions []*core.Session `json:"sessions"`
}

// NewServer creates a new API server instance
func NewServer(config *ServerConfig) (*Server, error) {
	// Initialize services
	sessionService, err := createSessionService(config.SessionServiceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create session service: %w", err)
	}

	artifactService, err := createArtifactService(config.ArtifactServiceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact service: %w", err)
	}

	memoryService, err := createMemoryService(config.MemoryServiceURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory service: %w", err)
	}

	// Initialize agent loader
	agentLoader := utils.NewAgentLoader(config.AgentsDir)

	// Configure WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins for now - in production, implement proper CORS checking
			return true
		},
	}

	server := &Server{
		config:          config,
		sessionService:  sessionService,
		artifactService: artifactService,
		memoryService:   memoryService,
		agentLoader:     agentLoader,
		runnerCache:     make(map[string]*runners.RunnerImpl),
		upgrader:        upgrader,
	}

	server.setupRoutes()
	return server, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	s.router = http.NewServeMux()

	// API routes
	s.router.HandleFunc("/list-apps", s.handleListApps)
	s.router.HandleFunc("/run", s.handleRun)
	s.router.HandleFunc("/run_sse", s.handleRunSSE)
	s.router.HandleFunc("/run_live", s.handleRunLive) // WebSocket endpoint

	// Session management routes
	s.router.HandleFunc("/apps/", s.handleSessionRoutes)

	// Health check
	s.router.HandleFunc("/health", s.handleHealth)

	// A2A routes (if enabled)
	if s.config.A2AEnabled {
		s.setupA2ARoutes()
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   s.config.AllowOrigins,
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
	})

	handler := c.Handler(s.router)

	address := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("Starting ADK API Server on %s", address)
	log.Printf("Agents directory: %s", s.config.AgentsDir)

	if s.config.A2AEnabled {
		log.Printf("A2A endpoint enabled")
	}

	return http.ListenAndServe(address, handler)
}

// handleListApps returns available agents
func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agents, err := s.agentLoader.ListAgents()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list agents: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// handleRun handles synchronous agent execution
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Get session
	_, err := s.sessionService.GetSession(r.Context(), &core.GetSessionRequest{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Get runner
	runner, err := s.getRunner(req.AppName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get runner: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute agent
	runReq := &core.RunRequest{
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		NewMessage: req.NewMessage,
	}

	events, err := runner.Run(r.Context(), runReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// handleRunSSE handles Server-Sent Events streaming
func (s *Server) handleRunSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get session
	_, err := s.sessionService.GetSession(r.Context(), &core.GetSessionRequest{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\": \"Session not found\"}\n\n")
		return
	}

	// Get runner
	runner, err := s.getRunner(req.AppName)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\": \"Failed to get runner: %v\"}\n\n", err)
		return
	}

	// Create run request
	runReq := &core.RunRequest{
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		NewMessage: req.NewMessage,
	}

	// Execute agent and stream events
	eventStream, err := runner.RunAsync(r.Context(), runReq)
	if err != nil {
		fmt.Fprintf(w, "data: {\"error\": \"Agent execution failed: %v\"}\n\n", err)
		return
	}

	// Stream events as SSE
	for event := range eventStream {
		eventJSON, err := json.Marshal(event)
		if err != nil {
			fmt.Fprintf(w, "data: {\"error\": \"Failed to encode event\"}\n\n")
			continue
		}

		fmt.Fprintf(w, "data: %s\n\n", eventJSON)

		// Flush the response to send data immediately
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// handleRunLive handles WebSocket connections for live agent interactions
func (s *Server) handleRunLive(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	appName := query.Get("app_name")
	userID := query.Get("user_id")
	sessionID := query.Get("session_id")

	if appName == "" || userID == "" || sessionID == "" {
		http.Error(w, "Missing required parameters: app_name, user_id, session_id", http.StatusBadRequest)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}
	defer conn.Close()

	// Get session
	session, err := s.sessionService.GetSession(r.Context(), &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1002, "Session not found"))
		return
	}

	// Get runner
	runner, err := s.getRunner(appName)
	if err != nil {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1011, "Failed to get runner"))
		return
	}

	// Handle WebSocket communication
	s.handleWebSocketSession(conn, runner, session)
}

// handleWebSocketSession manages the WebSocket session
func (s *Server) handleWebSocketSession(conn *websocket.Conn, runner *runners.RunnerImpl, session *core.Session) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for incoming messages
	messageChan := make(chan *core.Content, 10)

	// Goroutine to read WebSocket messages
	go func() {
		defer close(messageChan)
		for {
			var message map[string]interface{}
			err := conn.ReadJSON(&message)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}

			// Convert message to Content
			if content, ok := convertToContent(message); ok {
				select {
				case messageChan <- content:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Handle incoming messages
	for {
		select {
		case content, ok := <-messageChan:
			if !ok {
				return // Connection closed
			}

			// Create run request
			runReq := &core.RunRequest{
				UserID:     session.UserID,
				SessionID:  session.ID,
				NewMessage: content,
			}

			// Execute agent and stream responses
			eventStream, err := runner.RunAsync(ctx, runReq)
			if err != nil {
				conn.WriteJSON(map[string]interface{}{
					"error": fmt.Sprintf("Agent execution failed: %v", err),
				})
				continue
			}

			// Stream events back via WebSocket
			go func() {
				for event := range eventStream {
					if err := conn.WriteJSON(event); err != nil {
						log.Printf("Failed to write WebSocket message: %v", err)
						return
					}
				}
			}()

		case <-ctx.Done():
			return
		}
	}
}

// handleSessionRoutes handles session-related routes
func (s *Server) handleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/apps/")
	parts := strings.Split(path, "/")

	if len(parts) < 4 {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}

	appName := parts[0]
	// parts[1] should be "users"
	userID := parts[2]
	// parts[3] should be "sessions"

	if len(parts) == 4 {
		// /apps/{app_name}/users/{user_id}/sessions
		switch r.Method {
		case http.MethodGet:
			s.handleListSessions(w, r, appName, userID)
		case http.MethodPost:
			s.handleCreateSession(w, r, appName, userID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	sessionID := parts[4]

	if len(parts) == 5 {
		// /apps/{app_name}/users/{user_id}/sessions/{session_id}
		switch r.Method {
		case http.MethodGet:
			s.handleGetSession(w, r, appName, userID, sessionID)
		case http.MethodPost:
			s.handleCreateSessionWithID(w, r, appName, userID, sessionID)
		case http.MethodDelete:
			s.handleDeleteSession(w, r, appName, userID, sessionID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Handle other session sub-routes (artifacts, etc.)
	// TODO: Implement artifact routes
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// getRunner retrieves or creates a runner for the given app
func (s *Server) getRunner(appName string) (*runners.RunnerImpl, error) {
	if runner, exists := s.runnerCache[appName]; exists {
		return runner, nil
	}

	// Load agent
	agent, err := s.agentLoader.LoadAgent(appName)
	if err != nil {
		return nil, fmt.Errorf("failed to load agent: %w", err)
	}

	// Create runner
	runner := runners.NewRunner(appName, agent, s.sessionService)
	if s.artifactService != nil {
		runner.SetArtifactService(s.artifactService)
	}
	if s.memoryService != nil {
		runner.SetMemoryService(s.memoryService)
	}

	s.runnerCache[appName] = runner
	return runner, nil
}

// Helper functions for service creation
func createSessionService(uri string) (core.SessionService, error) {
	if uri == "" {
		return sessions.NewInMemorySessionService(), nil
	}
	// TODO: Implement database session service
	return sessions.NewInMemorySessionService(), nil
}

func createArtifactService(uri string) (core.ArtifactService, error) {
	// TODO: Implement artifact service
	return nil, nil
}

func createMemoryService(uri string) (core.MemoryService, error) {
	// TODO: Implement memory service
	return nil, nil
}

// convertToContent converts a generic message to Content
func convertToContent(message map[string]interface{}) (*core.Content, bool) {
	// Simple implementation - extend as needed
	if text, ok := message["text"].(string); ok {
		return &core.Content{
			Role: "user",
			Parts: []core.Part{
				{Type: "text", Text: &text},
			},
		}, true
	}
	return nil, false
}

// setupA2ARoutes configures A2A protocol routes
func (s *Server) setupA2ARoutes() {
	// TODO: Implement A2A routes
	s.router.HandleFunc("/a2a", s.handleA2A)
}

// handleA2A handles A2A protocol requests
func (s *Server) handleA2A(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse A2A request
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Extract agent name and message from A2A request format
	agentName, ok := req["agent_name"].(string)
	if !ok {
		http.Error(w, "Missing agent_name in request", http.StatusBadRequest)
		return
	}

	// Convert A2A message to ADK format
	// This is a simplified implementation - extend as needed
	content := &core.Content{
		Role:  "user",
		Parts: []core.Part{},
	}

	if messageData, ok := req["message"].(map[string]interface{}); ok {
		if parts, ok := messageData["parts"].([]interface{}); ok {
			for _, partData := range parts {
				if partMap, ok := partData.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						content.Parts = append(content.Parts, core.Part{
							Type: "text",
							Text: &text,
						})
					}
				}
			}
		}
	}

	// Create a session for this A2A request
	sessionReq := &core.CreateSessionRequest{
		AppName: agentName,
		UserID:  "a2a-user", // Special user ID for A2A requests
	}

	session, err := s.sessionService.CreateSession(r.Context(), sessionReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute the agent
	runReq := &core.RunRequest{
		UserID:     session.UserID,
		SessionID:  session.ID,
		NewMessage: content,
	}

	runner, err := s.getRunner(agentName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get runner: %v", err), http.StatusInternalServerError)
		return
	}

	events, err := runner.Run(r.Context(), runReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Agent execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert response to A2A format
	response := map[string]interface{}{
		"task_id": session.ID,
		"events":  events,
		"status":  "completed",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Session management handlers

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request, appName, userID string) {
	resp, err := s.sessionService.ListSessions(r.Context(), &core.ListSessionsRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Sessions)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request, appName, userID string) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	session, err := s.sessionService.CreateSession(r.Context(), &core.CreateSessionRequest{
		AppName: appName,
		UserID:  userID,
		State:   req.State,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	// Add events if provided
	for _, event := range req.Events {
		if err := s.sessionService.AppendEvent(r.Context(), session, event); err != nil {
			log.Printf("Failed to append event: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request, appName, userID, sessionID string) {
	session, err := s.sessionService.GetSession(r.Context(), &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *Server) handleCreateSessionWithID(w http.ResponseWriter, r *http.Request, appName, userID, sessionID string) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = CreateSessionRequest{} // Use empty request if body is invalid
	}

	// Check if session already exists
	existing, _ := s.sessionService.GetSession(r.Context(), &core.GetSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if existing != nil {
		http.Error(w, fmt.Sprintf("Session already exists: %s", sessionID), http.StatusBadRequest)
		return
	}

	session, err := s.sessionService.CreateSession(r.Context(), &core.CreateSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: &sessionID,
		State:     req.State,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request, appName, userID, sessionID string) {
	err := s.sessionService.DeleteSession(r.Context(), &core.DeleteSessionRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete session: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
