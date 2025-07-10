package agents

import (
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/a2a"
)

func TestRemoteA2aAgentConfig(t *testing.T) {
	// Test default configuration
	config := DefaultRemoteA2aAgentConfig()

	if config.Timeout != 600*time.Second {
		t.Errorf("Expected timeout 600s, got %v", config.Timeout)
	}

	if config.TaskPollingEnabled != true {
		t.Error("Expected task polling to be enabled by default")
	}

	if config.TaskPollingInterval != 2*time.Second {
		t.Errorf("Expected polling interval 2s, got %v", config.TaskPollingInterval)
	}

	if config.PreferStreaming != true {
		t.Error("Expected streaming to be preferred by default")
	}
}

func TestRemoteA2aAgentFromCard(t *testing.T) {
	name := "test-agent"
	description := "Test agent"
	card := &a2a.AgentCard{
		Name:        name,
		Description: &description,
		URL:         "http://localhost:8080",
		Version:     "1.0",
		Capabilities: a2a.AgentCapabilities{
			Streaming: true,
		},
		Skills: []a2a.AgentSkill{},
	}

	agent, err := NewRemoteA2aAgentFromCard("test-agent", card, nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if agent == nil {
		t.Error("Expected non-nil agent")
	}

	// Initially the agent card might not be resolved yet
	// This is expected behavior as the card needs to be resolved in a context
	if agent.IsResolved() {
		// If resolved, the card should match
		if agent.GetAgentCard() != card {
			t.Error("Expected agent card to match when resolved")
		}
	}
}

func TestStreamingDecision(t *testing.T) {
	// Test that an agent with streaming capabilities can be created
	name := "test-agent"
	description := "Test agent"
	card := &a2a.AgentCard{
		Name:        name,
		Description: &description,
		URL:         "http://localhost:8080",
		Version:     "1.0",
		Capabilities: a2a.AgentCapabilities{
			Streaming: true,
		},
		Skills: []a2a.AgentSkill{},
	}

	config := &RemoteA2aAgentConfig{
		PreferStreaming: true,
	}

	agent, err := NewRemoteA2aAgentFromCard("test-agent", card, config)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Test that the agent was created successfully
	if agent == nil {
		t.Error("Expected non-nil agent")
	}

	// Note: The shouldUseStreaming method requires the agent to be resolved first
	// This test just verifies the agent can be created with streaming configuration
}
