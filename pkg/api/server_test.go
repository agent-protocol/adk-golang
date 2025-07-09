package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/cli/utils"
	"github.com/agent-protocol/adk-golang/pkg/runners"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

func TestPatternRouting(t *testing.T) {
	// Create a test server
	config := &ServerConfig{
		Host:      "localhost",
		Port:      8080,
		AgentsDir: "test-agents",
	}

	server := &Server{
		config:          config,
		sessionService:  sessions.NewInMemorySessionService(),
		artifactService: nil,
		memoryService:   nil,
		agentLoader:     utils.NewAgentLoader(config.AgentsDir),
		runnerCache:     make(map[string]*runners.RunnerImpl),
	}

	server.setupRoutes()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "Health Check",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			description:    "Simple route without wildcards",
		},
		{
			name:           "List Apps",
			method:         "GET",
			path:           "/list-apps",
			expectedStatus: http.StatusInternalServerError, // Expected since no agents directory setup
			description:    "Simple route without wildcards",
		},
		{
			name:           "Wrong Method Health",
			method:         "POST",
			path:           "/health",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "Method not allowed should be handled by pattern matching",
		},
		{
			name:           "Pattern Matching - List Sessions",
			method:         "GET",
			path:           "/apps/test-app/users/user123/sessions",
			expectedStatus: http.StatusOK, // Returns empty list since session service is working
			description:    "Pattern matching with wildcards should extract parameters correctly",
		},
		{
			name:           "Pattern Matching - Get Session",
			method:         "GET",
			path:           "/apps/my-app/users/user456/sessions/session789",
			expectedStatus: http.StatusNotFound, // Expected since session doesn't exist
			description:    "Pattern matching with multiple wildcards",
		},
		{
			name:           "Wrong Method for Session",
			method:         "PUT",
			path:           "/apps/test-app/users/user123/sessions",
			expectedStatus: http.StatusMethodNotAllowed, // Go 1.22+ returns 405 for wrong method on existing pattern
			description:    "Method not matching any pattern should return 405",
		},
		{
			name:           "Not Implemented Route",
			method:         "GET",
			path:           "/apps/test-app/users/user123/sessions/session456/artifacts",
			expectedStatus: http.StatusNotImplemented,
			description:    "Route marked as not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d for %s %s (%s)",
					tt.expectedStatus, w.Code, tt.method, tt.path, tt.description)
			}
		})
	}
}

func TestPathValueExtraction(t *testing.T) {
	// Test that our wrapper functions correctly extract path values
	config := &ServerConfig{
		Host:      "localhost",
		Port:      8080,
		AgentsDir: "test-agents",
	}

	server := &Server{
		config:         config,
		sessionService: sessions.NewInMemorySessionService(),
		agentLoader:    utils.NewAgentLoader(config.AgentsDir),
		runnerCache:    make(map[string]*runners.RunnerImpl),
	}

	server.setupRoutes()

	req := httptest.NewRequest("GET", "/apps/my-test-app/users/user123/sessions", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// The request should have reached our handler (even if it fails due to missing setup)
	// This tests that the pattern matching and parameter extraction is working
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d (showing pattern matched), got %d",
			http.StatusOK, w.Code)
	}
}
