package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/agent-protocol/adk-golang/pkg/api"
)

// apiServerCommand creates the 'api-server' command
func apiServerCommand() *cli.Command {
	flags := append(commonServiceFlags(), webServerFlags()...)
	flags = append(flags, &cli.StringFlag{
		Name:  "agents-dir",
		Usage: "Directory containing agent folders",
		Value: ".",
	})

	return &cli.Command{
		Name:      "api-server",
		Usage:     "Starts a FastAPI-style HTTP server for agents",
		ArgsUsage: "[AGENTS_DIR]",
		Flags:     flags,
		Action:    apiServerCommandAction,
	}
}

func apiServerCommandAction(c *cli.Context) error {
	agentsDir := c.Args().First()
	if agentsDir == "" {
		agentsDir = c.String("agents-dir")
	}

	// Get absolute path
	absAgentsDir, err := filepath.Abs(agentsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if agents directory exists
	if _, err := os.Stat(absAgentsDir); os.IsNotExist(err) {
		return fmt.Errorf("agents directory not found: %s", absAgentsDir)
	}

	host := c.String("host")
	port := c.Int("port")
	logLevel := c.String("log-level")
	allowOrigins := c.StringSlice("allow-origins")
	traceToCloud := c.Bool("trace-to-cloud")
	a2a := c.Bool("a2a")

	// Service URIs
	sessionServiceURI := c.String("session-service-uri")
	artifactServiceURI := c.String("artifact-service-uri")
	memoryServiceURI := c.String("memory-service-uri")
	evalStorageURI := c.String("eval-storage-uri")

	fmt.Printf("Starting ADK API Server...\n")
	fmt.Printf("Agents directory: %s\n", absAgentsDir)
	fmt.Printf("Server address: http://%s:%d\n", host, port)
	fmt.Printf("Log level: %s\n", logLevel)
	if len(allowOrigins) > 0 {
		fmt.Printf("CORS origins: %v\n", allowOrigins)
	}
	if a2a {
		fmt.Printf("A2A endpoint: enabled\n")
	}

	// Create server configuration
	config := &api.ServerConfig{
		Host:               host,
		Port:               port,
		AgentsDir:          absAgentsDir,
		SessionServiceURI:  sessionServiceURI,
		ArtifactServiceURI: artifactServiceURI,
		MemoryServiceURI:   memoryServiceURI,
		EvalStorageURI:     evalStorageURI,
		AllowOrigins:       allowOrigins,
		TraceToCloud:       traceToCloud,
		A2AEnabled:         a2a,
		LogLevel:           logLevel,
	}

	// Create and start server
	server, err := api.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	fmt.Printf("🚀 API endpoints available at: http://%s:%d\n", host, port)
	fmt.Printf("📖 API documentation:\n")
	fmt.Printf("  POST /run - Run agent synchronously\n")
	fmt.Printf("  POST /run_sse - Run agent with Server-Sent Events\n")
	fmt.Printf("  WS   /run_live - WebSocket for live agent interactions\n")
	fmt.Printf("  GET  /list-apps - List available agents\n")
	fmt.Printf("  GET  /health - Health check\n")
	if a2a {
		fmt.Printf("  POST /a2a - A2A protocol endpoint\n")
	}
	fmt.Printf("📁 Serving agents from: %s\n", absAgentsDir)

	// Start the server (API only, no web UI)
	log.Printf("Starting API server...")
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
