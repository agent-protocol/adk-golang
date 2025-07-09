// Package agents provides concrete implementations of agent types.
package agents

import (
	"context"
	"fmt"
	"log"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// CustomAgent provides a basic implementation of the BaseAgent interface.
// This can be embedded in concrete agent types.
type CustomAgent struct {
	name                string
	description         string
	instruction         string
	subAgents           []core.BaseAgent
	parentAgent         core.BaseAgent
	beforeAgentCallback core.BeforeAgentCallback
	afterAgentCallback  core.AfterAgentCallback
	execute             func(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error
}

// NewBaseAgent creates a new base agent implementation.
func NewBaseAgent(name, description string) *CustomAgent {
	return &CustomAgent{
		name:        name,
		description: description,
		subAgents:   make([]core.BaseAgent, 0),
	}
}

// Name returns the agent's unique identifier.
func (a *CustomAgent) Name() string {
	return a.name
}

// Description returns a description of the agent's purpose.
func (a *CustomAgent) Description() string {
	return a.description
}

// Instruction returns optional system instructions for the agent.
func (a *CustomAgent) Instruction() string {
	return a.instruction
}

// SetInstruction sets the system instructions for the agent.
func (a *CustomAgent) SetInstruction(instruction string) {
	a.instruction = instruction
}

// SubAgents returns the list of child agents in the hierarchy.
func (a *CustomAgent) SubAgents() []core.BaseAgent {
	return a.subAgents
}

// AddSubAgent adds a child agent to this agent.
func (a *CustomAgent) AddSubAgent(subAgent core.BaseAgent) {
	subAgent.SetParentAgent(a)
	a.subAgents = append(a.subAgents, subAgent)
}

// ParentAgent returns the parent agent, if any.
func (a *CustomAgent) ParentAgent() core.BaseAgent {
	return a.parentAgent
}

// SetParentAgent sets the parent agent.
func (a *CustomAgent) SetParentAgent(parent core.BaseAgent) {
	a.parentAgent = parent
}

// GetBeforeAgentCallback returns the before-agent callback.
func (a *CustomAgent) GetBeforeAgentCallback() core.BeforeAgentCallback {
	return a.beforeAgentCallback
}

// SetBeforeAgentCallback sets the before-agent callback.
func (a *CustomAgent) SetBeforeAgentCallback(callback core.BeforeAgentCallback) {
	a.beforeAgentCallback = callback
}

// GetAfterAgentCallback returns the after-agent callback.
func (a *CustomAgent) GetAfterAgentCallback() core.AfterAgentCallback {
	return a.afterAgentCallback
}

// SetAfterAgentCallback sets the after-agent callback.
func (a *CustomAgent) SetAfterAgentCallback(callback core.AfterAgentCallback) {
	a.afterAgentCallback = callback
}

// FindAgent searches for an agent by name in the hierarchy.
func (a *CustomAgent) FindAgent(name string) core.BaseAgent {
	if a.name == name {
		return a
	}

	// Search in sub-agents recursively
	for _, subAgent := range a.subAgents {
		if found := subAgent.FindAgent(name); found != nil {
			return found
		}
	}

	return nil
}

// FindSubAgent searches for a direct sub-agent by name.
func (a *CustomAgent) FindSubAgent(name string) core.BaseAgent {
	for _, subAgent := range a.subAgents {
		if subAgent.Name() == name {
			return subAgent
		}
	}
	return nil
}

// RunAsync executes the agent with the given context and returns an event stream.
// This is a base implementation that should be overridden by concrete agents.
func (a *CustomAgent) RunAsync(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	// Execute before-agent callback if present
	if a.beforeAgentCallback != nil {
		if err := a.beforeAgentCallback(invocationCtx); err != nil {
			return nil, fmt.Errorf("before-agent callback failed: %w", err)
		}
	}

	// Create a channel to stream events
	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		errorEvent := core.NewEvent(invocationCtx.InvocationID, a.name)

		if a.execute == nil {
			log.Printf("No execute function defined for agent: %s", a.name)
			// Send an error event if no execute function is defined
			errorEvent.ErrorMessage = ptr.Ptr("No execute function defined for this agent")
		} else if err := a.execute(invocationCtx, eventChan); err != nil {
			log.Printf("Conversation flow failed: %v", err)
			// Send error event
			errorEvent.ErrorMessage = ptr.Ptr(fmt.Sprintf("Conversation flow failed: %v", err))
		}

		select {
		case eventChan <- errorEvent:
		case <-invocationCtx.Done():
			return
		}
	}()

	return eventChan, nil
}

// Run is a synchronous wrapper around RunAsync that collects all events.
func (a *CustomAgent) Run(invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	stream, err := a.RunAsync(invocationCtx)
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	for event := range stream {
		events = append(events, event)
	}

	// Execute after-agent callback if present
	if a.afterAgentCallback != nil {
		if err := a.afterAgentCallback(invocationCtx, events); err != nil {
			return events, fmt.Errorf("after-agent callback failed: %w", err)
		}
	}

	return events, nil
}

// Cleanup performs any necessary cleanup operations.
func (a *CustomAgent) Cleanup(ctx context.Context) error {
	// Cleanup sub-agents
	for _, subAgent := range a.subAgents {
		if err := subAgent.Cleanup(ctx); err != nil {
			return fmt.Errorf("failed to cleanup sub-agent %s: %w", subAgent.Name(), err)
		}
	}
	return nil
}
