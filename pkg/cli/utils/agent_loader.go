package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/agent-protocol/adk-golang/internal/core"
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

	// Try loading from plugin (.so file)
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

	agent, ok := symAgent.(core.BaseAgent)
	if !ok {
		return nil, fmt.Errorf("RootAgent is not a BaseAgent")
	}

	return agent, nil
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

	for _, execPath := range execPaths {
		if _, err := os.Stat(execPath); err == nil {
			// TODO: Implement executable agent wrapper
			return nil, fmt.Errorf("executable agent loading not yet implemented")
		}
	}

	return nil, fmt.Errorf("no executable agent found")
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
