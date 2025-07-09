// Package api provides web UI functionality for the ADK web server
package api

import (
	"embed"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// WebUIHandler provides web UI functionality
type WebUIHandler struct {
	server *Server
}

// NewWebUIHandler creates a new web UI handler
func NewWebUIHandler(server *Server) *WebUIHandler {
	return &WebUIHandler{server: server}
}

// HandleIndex serves the main web UI page
func (w *WebUIHandler) HandleIndex(writer http.ResponseWriter, req *http.Request) {
	// Serve the embedded index.html file
	indexContent, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(writer, "Failed to load index page", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "text/html")
	writer.Write(indexContent)
}

// SetupWebRoutes adds web UI routes to the server
func (s *Server) SetupWebRoutes() {
	webUI := NewWebUIHandler(s)

	// Serve the main web UI page
	s.router.HandleFunc("/", webUI.HandleIndex)

}
