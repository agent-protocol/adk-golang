// Package agents provides SequentialAgent implementation that runs sub-agents sequentially.
//
// SequentialAgent is a workflow agent that executes its sub-agents in the order they are
// specified in the list. This is particularly useful for creating conversation loops
// between agents where each agent's output becomes input for the next agent.
//
// Key features:
//   - Executes sub-agents in strict sequential order
//   - Passes output from one agent as input to the next
//   - Supports configurable number of rounds/iterations
//   - Manages conversation state across agent interactions
//   - Handles proper A2A event types and metadata
//
// Example usage:
//
//	studentAgent := NewLLMAgent("Student", "Ask questions about topics", config)
//	teacherAgent := NewLLMAgent("Teacher", "Provide detailed answers", config)
//	sequentialAgent := NewSequentialAgent("StudySession", "Student-teacher conversation",
//	                                     []core.BaseAgent{studentAgent, teacherAgent}, 10)
package agents

import (
	"context"
	"fmt"
	"log"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

var _ core.BaseAgent = (*SequentialAgent)(nil)

// SequentialAgentConfig contains configuration options for SequentialAgent.
type SequentialAgentConfig struct {
	// MaxRounds specifies the maximum number of conversation rounds
	MaxRounds int `json:"max_rounds,omitempty"`

	// StopOnError determines whether to stop execution if any sub-agent fails
	StopOnError bool `json:"stop_on_error,omitempty"`

	// PassCompleteHistory determines whether to pass the complete conversation
	// history to each agent or just the last response
	PassCompleteHistory bool `json:"pass_complete_history,omitempty"`

	// AddTurnMarkers adds turn/round information to the conversation context
	AddTurnMarkers bool `json:"add_turn_markers,omitempty"`
}

// DefaultSequentialAgentConfig returns default configuration for SequentialAgent.
func DefaultSequentialAgentConfig() *SequentialAgentConfig {
	return &SequentialAgentConfig{
		MaxRounds:           10,
		StopOnError:         true,
		PassCompleteHistory: true,
		AddTurnMarkers:      true,
	}
}

// SequentialAgent is a workflow agent that executes sub-agents in sequence.
// It's designed to create conversation loops between multiple agents.
type SequentialAgent struct {
	*CustomAgent
	config *SequentialAgentConfig
	agents []core.BaseAgent
}

// NewSequentialAgent creates a new SequentialAgent with the given sub-agents.
func NewSequentialAgent(name, description string, agents []core.BaseAgent, maxRounds int) *SequentialAgent {
	config := DefaultSequentialAgentConfig()
	if maxRounds > 0 {
		config.MaxRounds = maxRounds
	}

	agent := &SequentialAgent{
		CustomAgent: NewCustomAgent(name, description),
		config:      config,
		agents:      agents,
	}

	// Set up sub-agents in the hierarchy
	for _, subAgent := range agents {
		agent.AddSubAgent(subAgent)
	}

	// Set the execution function
	agent.CustomAgent.SetExecute(agent.executeSequentialFlow)

	return agent
}

// NewSequentialAgentWithConfig creates a new SequentialAgent with custom configuration.
func NewSequentialAgentWithConfig(name, description string, agents []core.BaseAgent, config *SequentialAgentConfig) *SequentialAgent {
	if config == nil {
		config = DefaultSequentialAgentConfig()
	}

	agent := &SequentialAgent{
		CustomAgent: NewCustomAgent(name, description),
		config:      config,
		agents:      agents,
	}

	// Set up sub-agents in the hierarchy
	for _, subAgent := range agents {
		agent.AddSubAgent(subAgent)
	}

	// Set the execution function
	agent.CustomAgent.SetExecute(agent.executeSequentialFlow)

	return agent
}

// Config returns the agent's configuration.
func (a *SequentialAgent) Config() *SequentialAgentConfig {
	return a.config
}

// SetConfig updates the agent's configuration.
func (a *SequentialAgent) SetConfig(config *SequentialAgentConfig) {
	a.config = config
}

// Agents returns the list of sub-agents.
func (a *SequentialAgent) Agents() []core.BaseAgent {
	return a.agents
}

// AddAgent adds a sub-agent to the sequence.
func (a *SequentialAgent) AddAgent(agent core.BaseAgent) {
	a.agents = append(a.agents, agent)
	a.AddSubAgent(agent)
}

// RemoveAgent removes a sub-agent from the sequence by name.
func (a *SequentialAgent) RemoveAgent(agentName string) bool {
	for i, agent := range a.agents {
		if agent.Name() == agentName {
			// Remove from agents slice
			a.agents = append(a.agents[:i], a.agents[i+1:]...)
			// Note: We don't remove from sub-agents here to maintain hierarchy consistency
			return true
		}
	}
	return false
}

// executeSequentialFlow manages the sequential execution of sub-agents.
func (a *SequentialAgent) executeSequentialFlow(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	log.Printf("Starting sequential agent flow with %d agents for %d rounds", len(a.agents), a.config.MaxRounds)

	if len(a.agents) == 0 {
		return fmt.Errorf("no sub-agents configured for sequential execution")
	}

	// Initialize conversation with user input if present
	if invocationCtx.UserContent != nil {
		if err := a.initializeConversation(invocationCtx); err != nil {
			return fmt.Errorf("failed to initialize conversation: %w", err)
		}
	}

	// Execute sequential rounds
	for round := 0; round < a.config.MaxRounds; round++ {
		log.Printf("Starting round %d of %d", round+1, a.config.MaxRounds)

		// Check for cancellation
		select {
		case <-invocationCtx.Context.Done():
			return invocationCtx.Context.Err()
		default:
		}

		// Execute each agent in sequence
		roundCompleted, err := a.executeRound(invocationCtx, eventChan, round)
		if err != nil {
			if a.config.StopOnError {
				return fmt.Errorf("round %d failed: %w", round+1, err)
			}
			log.Printf("Round %d failed but continuing: %v", round+1, err)
			continue
		}

		// Check if conversation should end early
		if roundCompleted {
			log.Printf("Conversation completed early at round %d", round+1)
			break
		}
	}

	// Send final completion event
	if err := a.sendCompletionEvent(invocationCtx, eventChan); err != nil {
		return fmt.Errorf("failed to send completion event: %w", err)
	}

	log.Println("Sequential agent flow completed successfully")
	return nil
}

// initializeConversation sets up the initial conversation state.
func (a *SequentialAgent) initializeConversation(invocationCtx *core.InvocationContext) error {
	// Add user content to session if not already present
	if len(invocationCtx.Session.Events) == 0 ||
		invocationCtx.Session.Events[len(invocationCtx.Session.Events)-1].Content == nil ||
		invocationCtx.Session.Events[len(invocationCtx.Session.Events)-1].Content.Role != "user" {

		userEvent := core.NewEvent(invocationCtx.InvocationID, "user")
		userEvent.Content = invocationCtx.UserContent

		// Add A2A-specific metadata for proper conversation tracking
		if userEvent.CustomMetadata == nil {
			userEvent.CustomMetadata = make(map[string]any)
		}
		userEvent.CustomMetadata["a2a:role"] = "user"
		userEvent.CustomMetadata["a2a:turn"] = 0
		userEvent.CustomMetadata["a2a:agent_type"] = "sequential"

		invocationCtx.Session.AddEvent(userEvent)
	}

	return nil
}

// executeRound executes one complete round of the sequential agent flow.
func (a *SequentialAgent) executeRound(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event, round int) (bool, error) {
	roundEvents := make([]*core.Event, 0, len(a.agents))

	for agentIndex, agent := range a.agents {
		log.Printf("Executing agent %s (%d/%d) in round %d", agent.Name(), agentIndex+1, len(a.agents), round+1)

		// Check for cancellation
		select {
		case <-invocationCtx.Context.Done():
			return false, invocationCtx.Context.Err()
		default:
		}

		// Create context for this agent execution
		agentCtx := a.createAgentContext(invocationCtx, agent, round, agentIndex)

		// Execute the agent
		agentEvents, err := a.executeAgent(agentCtx, agent, eventChan, round, agentIndex)
		if err != nil {
			return false, fmt.Errorf("agent %s failed: %w", agent.Name(), err)
		}

		// Store events for this round
		roundEvents = append(roundEvents, agentEvents...)

		// Add events to session for next agent
		for _, event := range agentEvents {
			invocationCtx.Session.AddEvent(event)
		}
	}

	// Check if conversation should continue based on the last agent's response
	if len(roundEvents) > 0 {
		lastEvent := roundEvents[len(roundEvents)-1]
		// Check for completion signals
		if lastEvent.TurnComplete != nil && *lastEvent.TurnComplete {
			log.Printf("Agent signaled completion at round %d", round+1)
			return true, nil
		}

		// Check for escalation
		if lastEvent.Actions.Escalate != nil && *lastEvent.Actions.Escalate {
			log.Printf("Agent requested escalation at round %d", round+1)
			return true, nil
		}
	}

	return false, nil
}

// createAgentContext creates an invocation context for a specific agent.
func (a *SequentialAgent) createAgentContext(invocationCtx *core.InvocationContext, agent core.BaseAgent, round, agentIndex int) *core.InvocationContext {
	// Create a new context with agent-specific branch
	agentCtx := &core.InvocationContext{
		InvocationID: invocationCtx.InvocationID,
		Session:      invocationCtx.Session,
		Context:      invocationCtx.Context,
		UserContent:  nil, // Will be set based on configuration
	}

	// Set branch for A2A compatibility
	branch := fmt.Sprintf("%s.%s.R%d", a.Name(), agent.Name(), round+1)
	agentCtx.Branch = &branch

	// Determine what content to pass to the agent
	if a.config.PassCompleteHistory {
		// Agent will get full conversation history from session
		agentCtx.UserContent = nil
	} else {
		// Pass only the most recent relevant content
		agentCtx.UserContent = a.getLastRelevantContent(invocationCtx, agentIndex)
	}

	return agentCtx
}

// getLastRelevantContent extracts the most recent relevant content for an agent.
func (a *SequentialAgent) getLastRelevantContent(invocationCtx *core.InvocationContext, agentIndex int) *core.Content {
	events := invocationCtx.Session.Events
	if len(events) == 0 {
		return invocationCtx.UserContent
	}

	// For the first agent in a round, use the last user or previous agent's message
	// For subsequent agents, use the immediately previous agent's response
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Content != nil && len(event.Content.Parts) > 0 {
			// Check if this is a relevant message (not a function call/response)
			hasTextContent := false
			for _, part := range event.Content.Parts {
				if part.Type == "text" && part.Text != nil {
					hasTextContent = true
					break
				}
			}

			if hasTextContent {
				return event.Content
			}
		}
	}

	// Fallback to original user content
	return invocationCtx.UserContent
}

// executeAgent executes a single agent and returns its events.
func (a *SequentialAgent) executeAgent(agentCtx *core.InvocationContext, agent core.BaseAgent, eventChan chan<- *core.Event, round, agentIndex int) ([]*core.Event, error) {
	// Add turn marker if configured
	if a.config.AddTurnMarkers && agentCtx.UserContent == nil {
		// Create a turn marker content to provide context
		turnContent := &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr(fmt.Sprintf("Continue the conversation. This is round %d, agent %s turn.", round+1, agent.Name())),
				},
			},
		}
		agentCtx.UserContent = turnContent
	}

	// Execute the agent
	agentEvents, err := agent.Run(agentCtx)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	// Process and enhance events with sequential agent metadata
	processedEvents := make([]*core.Event, 0, len(agentEvents))
	for _, event := range agentEvents {
		// Add sequential agent metadata for A2A compatibility
		if event.CustomMetadata == nil {
			event.CustomMetadata = make(map[string]any)
		}
		event.CustomMetadata["a2a:sequential_agent"] = a.Name()
		event.CustomMetadata["a2a:round"] = round + 1
		event.CustomMetadata["a2a:agent_index"] = agentIndex
		event.CustomMetadata["a2a:agent_name"] = agent.Name()
		event.CustomMetadata["a2a:role"] = "agent"

		// Set branch if not already set
		if event.Branch == nil {
			branch := fmt.Sprintf("%s.%s.R%d", a.Name(), agent.Name(), round+1)
			event.Branch = &branch
		}

		// Forward event to the main event channel
		select {
		case eventChan <- event:
		case <-agentCtx.Context.Done():
			return processedEvents, agentCtx.Context.Err()
		}

		processedEvents = append(processedEvents, event)
	}

	return processedEvents, nil
}

// sendCompletionEvent sends a final completion event for the sequential workflow.
func (a *SequentialAgent) sendCompletionEvent(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	completionEvent := core.NewEvent(invocationCtx.InvocationID, a.Name())
	completionEvent.Content = &core.Content{
		Role: "agent",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr(fmt.Sprintf("Sequential agent conversation completed. Executed %d agents for up to %d rounds.", len(a.agents), a.config.MaxRounds)),
			},
		},
	}

	// Mark as final response
	completionEvent.TurnComplete = ptr.Ptr(true)

	// Add completion metadata
	if completionEvent.CustomMetadata == nil {
		completionEvent.CustomMetadata = make(map[string]any)
	}
	completionEvent.CustomMetadata["a2a:role"] = "agent"
	completionEvent.CustomMetadata["a2a:type"] = "completion"
	completionEvent.CustomMetadata["a2a:sequential_agent"] = a.Name()
	completionEvent.CustomMetadata["a2a:agents_count"] = len(a.agents)
	completionEvent.CustomMetadata["a2a:max_rounds"] = a.config.MaxRounds

	select {
	case eventChan <- completionEvent:
		return nil
	case <-invocationCtx.Context.Done():
		return invocationCtx.Context.Err()
	}
}

// Run executes the sequential agent synchronously.
func (a *SequentialAgent) Run(invocationCtx *core.InvocationContext) ([]*core.Event, error) {
	return a.CustomAgent.Run(invocationCtx)
}

// RunAsync executes the sequential agent asynchronously.
func (a *SequentialAgent) RunAsync(invocationCtx *core.InvocationContext) (core.EventStream, error) {
	return a.CustomAgent.RunAsync(invocationCtx)
}

// Cleanup performs cleanup operations for the sequential agent and its sub-agents.
func (a *SequentialAgent) Cleanup(ctx context.Context) error {
	// Cleanup all sub-agents
	for _, agent := range a.agents {
		if err := agent.Cleanup(ctx); err != nil {
			log.Printf("Failed to cleanup agent %s: %v", agent.Name(), err)
		}
	}

	// Call parent cleanup
	return a.CustomAgent.Cleanup(ctx)
}
