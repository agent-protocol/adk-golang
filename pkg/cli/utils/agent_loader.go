package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// AgentLoader handles loading agents from the filesystem
type AgentLoader struct {
	agentsDir string
	cache     map[string]core.BaseAgent
}

// NewAgentLoader creates a new agent loader
func NewAgentLoader(agentsDir string) *AgentLoader {
	return &AgentLoader{
		agentsDir: strings.TrimSuffix(agentsDir, "/"),
		cache:     make(map[string]core.BaseAgent),
	}
}

// LoadAgent loads an agent from the specified directory
// Supports the following structures:
// 1. agents_dir/{agent_name}/agent.go - compiled as plugin
// 2. agents_dir/{agent_name}/main.go - executable agent
// 3. agents_dir/{agent_name}.yml - declarative agent configuration
func (al *AgentLoader) LoadAgent(agentName string) (core.BaseAgent, error) {
	// Check cache first
	if agent, exists := al.cache[agentName]; exists {
		return agent, nil
	}

	agent, err := al.performLoad(agentName)
	if err != nil {
		return nil, err
	}

	// Cache the loaded agent
	al.cache[agentName] = agent
	return agent, nil
}

// performLoad handles the actual loading logic
func (al *AgentLoader) performLoad(agentName string) (core.BaseAgent, error) {
	agentDir := filepath.Join(al.agentsDir, agentName)

	// Check if agent directory exists
	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("agent directory not found: %s", agentDir)
	}

	// Try loading from Go source file first (build as plugin)
	if agent, err := al.loadFromGoSource(agentName, agentDir); err == nil {
		return agent, nil
	}

	// Try loading from existing plugin (.so file)
	if agent, err := al.loadFromPlugin(agentName, agentDir); err == nil {
		return agent, nil
	}

	// Try loading from YAML configuration
	if agent, err := al.loadFromYAML(agentName, agentDir); err == nil {
		return agent, nil
	}

	// Try loading from executable
	if agent, err := al.loadFromExecutable(agentName, agentDir); err == nil {
		return agent, nil
	}

	return nil, fmt.Errorf("no valid agent found in directory: %s", agentDir)
}

// loadFromGoSource loads an agent from Go source file by building and loading as plugin
func (al *AgentLoader) loadFromGoSource(agentName, agentDir string) (core.BaseAgent, error) {
	agentGoPath := filepath.Join(agentDir, "agent.go")
	mainGoPath := filepath.Join(agentDir, "main.go")

	var sourceFile string
	if _, err := os.Stat(agentGoPath); err == nil {
		sourceFile = agentGoPath
	} else if _, err := os.Stat(mainGoPath); err == nil {
		sourceFile = mainGoPath
	} else {
		return nil, fmt.Errorf("no Go source file found (agent.go or main.go)")
	}

	// Build the agent as a plugin
	pluginPath := filepath.Join(agentDir, "agent.so")
	if err := al.buildPlugin(sourceFile, pluginPath); err != nil {
		return nil, fmt.Errorf("failed to build plugin: %w", err)
	}

	// Load the plugin
	return al.loadFromPlugin(agentName, agentDir)
}

// buildPlugin builds a Go source file as a plugin
func (al *AgentLoader) buildPlugin(sourceFile, outputPath string) error {
	// Use go build -buildmode=plugin to create the plugin
	args := []string{"build", "-buildmode=plugin", "-o", outputPath, sourceFile}

	// Execute go build command
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Dir(sourceFile)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// loadFromPlugin loads an agent from a compiled Go plugin
func (al *AgentLoader) loadFromPlugin(agentName, agentDir string) (core.BaseAgent, error) {
	pluginPath := filepath.Join(agentDir, "agent.so")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin not found: %s", pluginPath)
	}

	p, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symAgent, err := p.Lookup("RootAgent")
	if err != nil {
		return nil, fmt.Errorf("RootAgent not found in plugin: %w", err)
	}

	// Try to cast to BaseAgent directly
	if agent, ok := symAgent.(core.BaseAgent); ok {
		return agent, nil
	}

	// Try to cast to pointer to BaseAgent
	if agentPtr, ok := symAgent.(*core.BaseAgent); ok {
		if *agentPtr != nil {
			return *agentPtr, nil
		}
		return nil, fmt.Errorf("RootAgent is nil")
	}

	return nil, fmt.Errorf("RootAgent is not a BaseAgent, got type: %T", symAgent)
}

// loadFromYAML loads an agent from YAML configuration
func (al *AgentLoader) loadFromYAML(agentName, agentDir string) (core.BaseAgent, error) {
	yamlPath := filepath.Join(agentDir, "agent.yml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		yamlPath = filepath.Join(agentDir, "agent.yaml")
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("no YAML configuration found")
		}
	}

	// TODO: Implement YAML loading once we have agent configuration structures
	return nil, fmt.Errorf("YAML agent loading not yet implemented")
}

// loadFromExecutable loads an agent that runs as a separate process
func (al *AgentLoader) loadFromExecutable(agentName, agentDir string) (core.BaseAgent, error) {
	execPaths := []string{
		filepath.Join(agentDir, "main"),
		filepath.Join(agentDir, agentName),
		filepath.Join(agentDir, "agent"),
	}

	var execPath string
	for _, path := range execPaths {
		if _, err := os.Stat(path); err == nil {
			execPath = path
			break
		}
	}

	if execPath == "" {
		return nil, fmt.Errorf("no executable agent found")
	}

	// For this demo, create a simple proxy agent that can represent the executable
	// In a full implementation, this would use IPC to communicate with the executable
	return NewExecutableAgentProxy(agentName, execPath), nil
}

// ExecutableAgentProxy represents an agent that runs as a separate executable
type ExecutableAgentProxy struct {
	name        string
	execPath    string
	description string
}

// NewExecutableAgentProxy creates a new proxy for an executable agent
func NewExecutableAgentProxy(name, execPath string) *ExecutableAgentProxy {
	return &ExecutableAgentProxy{
		name:        name,
		execPath:    execPath,
		description: fmt.Sprintf("Executable agent: %s", name),
	}
}

func (e *ExecutableAgentProxy) Name() string                  { return e.name }
func (e *ExecutableAgentProxy) Description() string           { return e.description }
func (e *ExecutableAgentProxy) Instruction() string           { return "Executable agent proxy" }
func (e *ExecutableAgentProxy) SubAgents() []core.BaseAgent   { return nil }
func (e *ExecutableAgentProxy) ParentAgent() core.BaseAgent   { return nil }
func (e *ExecutableAgentProxy) SetParentAgent(core.BaseAgent) {}

func (e *ExecutableAgentProxy) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		// Create a simple response event
		event := core.NewEvent(invocationCtx.InvocationID, e.name)

		// Extract text from user's message
		var userText string
		if invocationCtx.UserContent != nil {
			for _, part := range invocationCtx.UserContent.Parts {
				if part.Text != nil {
					userText = *part.Text
					break
				}
			}
		}

		// Create a simple response (for demo purposes)
		responseText := fmt.Sprintf("Executable agent '%s' received: %s", e.name, userText)
		event.Content = &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{Type: "text", Text: &responseText},
			},
		}

		select {
		case eventChan <- event:
		case <-ctx.Done():
			return
		}
	}()

	return eventChan, nil
}

func (e *ExecutableAgentProxy) Run(ctx context.Context, invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	eventStream, err := e.RunAsync(ctx, invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range eventStream {
		events = append(events, event)
	}
	return events, nil
}

func (e *ExecutableAgentProxy) FindAgent(name string) core.BaseAgent {
	if e.name == name {
		return e
	}
	return nil
}

func (e *ExecutableAgentProxy) FindSubAgent(name string) core.BaseAgent          { return nil }
func (e *ExecutableAgentProxy) GetBeforeAgentCallback() core.BeforeAgentCallback { return nil }
func (e *ExecutableAgentProxy) SetBeforeAgentCallback(core.BeforeAgentCallback)  {}
func (e *ExecutableAgentProxy) GetAfterAgentCallback() core.AfterAgentCallback   { return nil }
func (e *ExecutableAgentProxy) SetAfterAgentCallback(core.AfterAgentCallback)    {}
func (e *ExecutableAgentProxy) Cleanup(ctx context.Context) error                { return nil }

// ListAgents returns a list of available agents in the agents directory
func (al *AgentLoader) ListAgents() ([]string, error) {
	entries, err := os.ReadDir(al.agentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read agents directory: %w", err)
	}

	var agents []string
	for _, entry := range entries {
		if entry.IsDir() {
			agentName := entry.Name()
			// Check if the directory contains valid agent files
			if al.hasValidAgentFiles(filepath.Join(al.agentsDir, agentName)) {
				agents = append(agents, agentName)
			}
		}
	}

	return agents, nil
}

// hasValidAgentFiles checks if a directory contains valid agent files
func (al *AgentLoader) hasValidAgentFiles(agentDir string) bool {
	validFiles := []string{
		"agent.go",   // Go source (preferred)
		"main.go",    // Go source alternative
		"agent.so",   // Plugin
		"agent.yml",  // YAML config
		"agent.yaml", // YAML config
		"main",       // Executable
		"agent",      // Executable
	}

	for _, file := range validFiles {
		if _, err := os.Stat(filepath.Join(agentDir, file)); err == nil {
			return true
		}
	}

	return false
}

// LoadDotEnv loads environment variables from .env file in agent directory
func (al *AgentLoader) LoadDotEnv(agentName string) error {
	envPath := filepath.Join(al.agentsDir, agentName, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// .env file is optional
		return nil
	}

	// TODO: Implement .env file loading
	// For now, we'll just log that we found an .env file
	fmt.Printf("Found .env file for agent %s: %s\n", agentName, envPath)
	return nil
}
