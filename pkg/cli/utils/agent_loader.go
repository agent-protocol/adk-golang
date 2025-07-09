package utils

import (
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
		"agent.go", // Go source (preferred)
		"agent.so", // Plugin
	}

	for _, file := range validFiles {
		if _, err := os.Stat(filepath.Join(agentDir, file)); err == nil {
			return true
		}
	}

	return false
}
