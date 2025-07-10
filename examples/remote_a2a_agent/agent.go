// Package main demonstrates how to use RemoteA2aAgent to communicate with
// a remote Python ADK agent via the A2A protocol.
//
// This example shows:
// - Creating a RemoteA2aAgent that connects to a Python agent
// - Using the remote agent with enhanced task handling capabilities
// - Proper error handling and context management
package main

import (
	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/core"
)

var RootAgent core.BaseAgent

func init() {
	agentCardURL := "http://localhost:8001/a2a/check_prime_agent/.well-known/agent.json"
	// Using the new unified RemoteA2aAgent (with enhanced capabilities built-in)
	RootAgent, _ = agents.NewRemoteA2aAgentFromURL("remote_prime_checker", agentCardURL, nil)
}
