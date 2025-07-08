// Package agents provides concrete implementations of agent types.
package agents

import (
	"context"
	"fmt"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// BaseAgentImpl provides a basic implementation of the BaseAgent interface.
// This can be embedded in concrete agent types.
type BaseAgentImpl struct {
	name        string
	description string
	instruction string
	subAgents   []core.BaseAgent
	parentAgent core.BaseAgent
}

// NewBaseAgent creates a new base agent implementation.
func NewBaseAgent(name, description string) *BaseAgentImpl {
	return &BaseAgentImpl{
		name:        name,
		description: description,
		subAgents:   make([]core.BaseAgent, 0),
	}
}

// Name returns the agent's unique identifier.
func (a *BaseAgentImpl) Name() string {
	return a.name
}

// Description returns a description of the agent's purpose.
func (a *BaseAgentImpl) Description() string {
	return a.description
}

// Instruction returns optional system instructions for the agent.
func (a *BaseAgentImpl) Instruction() string {
	return a.instruction
}

// SetInstruction sets the system instructions for the agent.
func (a *BaseAgentImpl) SetInstruction(instruction string) {
	a.instruction = instruction
}

// SubAgents returns the list of child agents in the hierarchy.
func (a *BaseAgentImpl) SubAgents() []core.BaseAgent {
	return a.subAgents
}

// AddSubAgent adds a child agent to this agent.
func (a *BaseAgentImpl) AddSubAgent(subAgent core.BaseAgent) {
	subAgent.SetParentAgent(a)
	a.subAgents = append(a.subAgents, subAgent)
}

// ParentAgent returns the parent agent, if any.
func (a *BaseAgentImpl) ParentAgent() core.BaseAgent {
	return a.parentAgent
}

// SetParentAgent sets the parent agent.
func (a *BaseAgentImpl) SetParentAgent(parent core.BaseAgent) {
	a.parentAgent = parent
}

// FindAgent searches for an agent by name in the hierarchy.
func (a *BaseAgentImpl) FindAgent(name string) core.BaseAgent {
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
func (a *BaseAgentImpl) FindSubAgent(name string) core.BaseAgent {
	for _, subAgent := range a.subAgents {
		if subAgent.Name() == name {
			return subAgent
		}
	}
	return nil
}

// RunAsync executes the agent with the given context and returns an event stream.
// This is a base implementation that should be overridden by concrete agents.
func (a *BaseAgentImpl) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	// Create a channel to stream events
	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		// Create a simple response event
		event := core.NewEvent(invocationCtx.InvocationID, a.name)
		event.Content = &core.Content{
			Role: "agent",
			Parts: []core.Part{
				{
					Type: "text",
					Text: stringPtr("Hello from " + a.name),
				},
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

// Cleanup performs any necessary cleanup operations.
func (a *BaseAgentImpl) Cleanup(ctx context.Context) error {
	// Cleanup sub-agents
	for _, subAgent := range a.subAgents {
		if err := subAgent.Cleanup(ctx); err != nil {
			return fmt.Errorf("failed to cleanup sub-agent %s: %w", subAgent.Name(), err)
		}
	}
	return nil
}

// SequentialAgent executes sub-agents in sequence.
type SequentialAgent struct {
	*BaseAgentImpl
}

// NewSequentialAgent creates a new sequential agent.
func NewSequentialAgent(name, description string) *SequentialAgent {
	return &SequentialAgent{
		BaseAgentImpl: NewBaseAgent(name, description),
	}
}

// RunAsync executes sub-agents in sequence.
func (a *SequentialAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		// Execute each sub-agent in sequence
		for _, subAgent := range a.subAgents {
			subStream, err := subAgent.RunAsync(ctx, invocationCtx)
			if err != nil {
				// Send error event
				errorEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
				errorEvent.ErrorMessage = stringPtr(fmt.Sprintf("Error executing sub-agent %s: %v", subAgent.Name(), err))

				select {
				case eventChan <- errorEvent:
				case <-ctx.Done():
					return
				}
				return
			}

			// Forward all events from the sub-agent
			for event := range subStream {
				select {
				case eventChan <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return eventChan, nil
}

// LLMAgent represents an agent that uses a language model for reasoning.
type LLMAgent struct {
	*BaseAgentImpl
	model         string
	tools         []core.BaseTool
	inputSchema   interface{}
	outputSchema  interface{}
	llmConnection core.LLMConnection
}

// NewLLMAgent creates a new LLM-based agent.
func NewLLMAgent(name, description, model string) *LLMAgent {
	return &LLMAgent{
		BaseAgentImpl: NewBaseAgent(name, description),
		model:         model,
		tools:         make([]core.BaseTool, 0),
	}
}

// Model returns the LLM model name.
func (a *LLMAgent) Model() string {
	return a.model
}

// SetModel sets the LLM model name.
func (a *LLMAgent) SetModel(model string) {
	a.model = model
}

// Tools returns the available tools for this agent.
func (a *LLMAgent) Tools() []core.BaseTool {
	return a.tools
}

// AddTool adds a tool to this agent.
func (a *LLMAgent) AddTool(tool core.BaseTool) {
	a.tools = append(a.tools, tool)
}

// SetLLMConnection sets the LLM connection for this agent.
func (a *LLMAgent) SetLLMConnection(conn core.LLMConnection) {
	a.llmConnection = conn
}

// RunAsync executes the LLM agent with reasoning capabilities.
func (a *LLMAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
	if a.llmConnection == nil {
		return nil, fmt.Errorf("LLM connection not configured for agent %s", a.name)
	}

	eventChan := make(chan *core.Event, 10)

	go func() {
		defer close(eventChan)

		// Build LLM request from session history
		request := a.buildLLMRequest(invocationCtx)

		// Make LLM call
		response, err := a.llmConnection.GenerateContent(ctx, request)
		if err != nil {
			errorEvent := core.NewEvent(invocationCtx.InvocationID, a.name)
			errorEvent.ErrorMessage = stringPtr(fmt.Sprintf("LLM request failed: %v", err))

			select {
			case eventChan <- errorEvent:
			case <-ctx.Done():
				return
			}
			return
		}

		// Convert LLM response to event
		event := core.NewEvent(invocationCtx.InvocationID, a.name)
		event.Content = response.Content

		select {
		case eventChan <- event:
		case <-ctx.Done():
			return
		}
	}()

	return eventChan, nil
}

// buildLLMRequest constructs an LLM request from the session context.
func (a *LLMAgent) buildLLMRequest(invocationCtx *core.InvocationContext) *core.LLMRequest {
	// Convert session events to LLM contents
	contents := make([]core.Content, 0)

	// Add system instruction if present
	if a.instruction != "" {
		contents = append(contents, core.Content{
			Role: "system",
			Parts: []core.Part{
				{
					Type: "text",
					Text: &a.instruction,
				},
			},
		})
	}

	// Add session history
	for _, event := range invocationCtx.Session.Events {
		if event.Content != nil {
			contents = append(contents, *event.Content)
		}
	}

	// Add current user message if present
	if invocationCtx.UserContent != nil {
		contents = append(contents, *invocationCtx.UserContent)
	}

	// Build tool declarations
	var tools []*core.FunctionDeclaration
	for _, tool := range a.tools {
		if decl := tool.GetDeclaration(); decl != nil {
			tools = append(tools, decl)
		}
	}

	return &core.LLMRequest{
		Contents: contents,
		Config: &core.LLMConfig{
			Model: a.model,
			Tools: tools,
		},
		Tools: tools,
	}
}

// stringPtr returns a pointer to a string literal.
func stringPtr(s string) *string {
	return &s
}
