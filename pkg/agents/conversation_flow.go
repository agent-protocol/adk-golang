package agents

import (
	"context"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// ConversationFlowManager handles the conversation flow execution logic
type ConversationFlowManager struct {
	agent          *LLMAgent
	maxTurns       int
	maxToolCalls   int
	loopDetector   *LoopDetector
	eventPublisher *EventPublisher
}

// NewConversationFlowManager creates a new conversation flow manager
func NewConversationFlowManager(agent *LLMAgent, invocationCtx *core.InvocationContext) *ConversationFlowManager {
	maxTurns := 10 // Default max turns to prevent infinite loops
	if invocationCtx.RunConfig != nil && invocationCtx.RunConfig.MaxTurns != nil {
		maxTurns = *invocationCtx.RunConfig.MaxTurns
	}

	return &ConversationFlowManager{
		agent:          agent,
		maxTurns:       maxTurns,
		maxToolCalls:   agent.config.MaxToolCalls * 2, // Allow some flexibility
		loopDetector:   NewLoopDetector(),
		eventPublisher: NewEventPublisher(),
	}
}

// EventPublisher handles event publishing logic
type EventPublisher struct{}

// NewEventPublisher creates a new event publisher
func NewEventPublisher() *EventPublisher {
	return &EventPublisher{}
}

// PublishEvent publishes an event to the event channel
func (ep *EventPublisher) PublishEvent(ctx context.Context, eventChan chan<- *core.Event, event *core.Event) error {
	select {
	case eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateFinalResponse creates a final response event when conversation ends
func (ep *EventPublisher) CreateFinalResponse(invocationID, agentName, message string) *core.Event {
	event := core.NewEvent(invocationID, agentName)
	event.Content = &core.Content{
		Role: "assistant",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr(message),
			},
		},
	}
	event.TurnComplete = ptr.Ptr(true)
	return event
}
