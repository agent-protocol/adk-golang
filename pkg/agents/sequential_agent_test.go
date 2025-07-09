package agents

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

// MockAgent is a simple agent for testing that generates predictable responses
type MockAgent struct {
	*CustomAgent
	responseTemplate string
	callCount        int
}

// NewMockAgent creates a new mock agent for testing
func NewMockAgent(name, description, responseTemplate string) *MockAgent {
	agent := &MockAgent{
		CustomAgent:      NewBaseAgent(name, description),
		responseTemplate: responseTemplate,
		callCount:        0,
	}

	agent.CustomAgent.SetExecute(agent.mockExecute)
	return agent
}

// mockExecute generates a predictable response for testing
func (m *MockAgent) mockExecute(invocationCtx *core.InvocationContext, eventChan chan<- *core.Event) error {
	m.callCount++

	// Create response based on template and call count
	response := core.NewEvent(invocationCtx.InvocationID, m.Name())
	response.Content = &core.Content{
		Role: "agent",
		Parts: []core.Part{
			{
				Type: "text",
				Text: ptr.Ptr(fmt.Sprintf(m.responseTemplate, m.callCount)),
			},
		},
	}

	// Add some test metadata
	if response.CustomMetadata == nil {
		response.CustomMetadata = make(map[string]any)
	}
	response.CustomMetadata["mock:call_count"] = m.callCount
	response.CustomMetadata["mock:agent_name"] = m.Name()

	select {
	case eventChan <- response:
		return nil
	case <-invocationCtx.Context.Done():
		return invocationCtx.Context.Err()
	}
}

// GetCallCount returns the number of times this agent has been called
func (m *MockAgent) GetCallCount() int {
	return m.callCount
}

// TestSequentialAgent_BasicExecution tests basic sequential execution
func TestSequentialAgent_BasicExecution(t *testing.T) {
	// Create mock agents
	student := NewMockAgent("Student", "Ask questions", "Question %d: What is the capital of France?")
	teacher := NewMockAgent("Teacher", "Answer questions", "Answer %d: The capital of France is Paris.")

	// Create sequential agent
	sequential := NewSequentialAgent("StudySession", "Student-teacher conversation",
		[]core.BaseAgent{student, teacher}, 2)

	// Create test context
	ctx := context.Background()
	session := &core.Session{
		ID:      "test-session",
		UserID:  "test-user",
		AppName: "test-app",
		State:   make(map[string]any),
		Events:  make([]*core.Event, 0),
	}

	invocationCtx := &core.InvocationContext{
		InvocationID: "test-invocation",
		Session:      session,
		Context:      ctx,
		UserContent: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr("Let's start a study session about geography."),
				},
			},
		},
	}

	// Execute the sequential agent
	events, err := sequential.Run(invocationCtx)
	if err != nil {
		t.Fatalf("Sequential agent execution failed: %v", err)
	}

	// Verify results
	if len(events) == 0 {
		t.Fatal("Expected events from sequential agent execution")
	}

	// Each agent should have been called twice (2 rounds)
	if student.GetCallCount() != 2 {
		t.Errorf("Expected student to be called 2 times, got %d", student.GetCallCount())
	}

	if teacher.GetCallCount() != 2 {
		t.Errorf("Expected teacher to be called 2 times, got %d", teacher.GetCallCount())
	}

	// Check that events have proper metadata
	agentEvents := 0
	for _, event := range events {
		if event.Author != "StudySession" { // Skip completion event
			agentEvents++
			if event.CustomMetadata == nil {
				t.Error("Event should have custom metadata")
				continue
			}

			if _, exists := event.CustomMetadata["a2a:sequential_agent"]; !exists {
				t.Error("Event should have sequential agent metadata")
			}

			if _, exists := event.CustomMetadata["a2a:round"]; !exists {
				t.Error("Event should have round metadata")
			}
		}
	}

	// Should have 4 agent events (2 rounds × 2 agents) + 1 completion event
	expectedEvents := 5
	if len(events) != expectedEvents {
		t.Errorf("Expected %d events, got %d", expectedEvents, len(events))
	}
}

// TestSequentialAgent_Configuration tests different configuration options
func TestSequentialAgent_Configuration(t *testing.T) {
	student := NewMockAgent("Student", "Ask questions", "Question %d")
	teacher := NewMockAgent("Teacher", "Answer questions", "Answer %d")

	// Test custom configuration
	config := &SequentialAgentConfig{
		MaxRounds:           3,
		StopOnError:         false,
		PassCompleteHistory: false,
		AddTurnMarkers:      true,
	}

	sequential := NewSequentialAgentWithConfig("TestSession", "Test conversation",
		[]core.BaseAgent{student, teacher}, config)

	// Verify configuration
	if sequential.Config().MaxRounds != 3 {
		t.Errorf("Expected MaxRounds to be 3, got %d", sequential.Config().MaxRounds)
	}

	if sequential.Config().StopOnError != false {
		t.Error("Expected StopOnError to be false")
	}

	if sequential.Config().PassCompleteHistory != false {
		t.Error("Expected PassCompleteHistory to be false")
	}

	if sequential.Config().AddTurnMarkers != true {
		t.Error("Expected AddTurnMarkers to be true")
	}
}

// TestSequentialAgent_EmptyAgents tests behavior with no sub-agents
func TestSequentialAgent_EmptyAgents(t *testing.T) {
	sequential := NewSequentialAgent("EmptySession", "No agents", []core.BaseAgent{}, 1)

	ctx := context.Background()
	session := &core.Session{
		ID:      "test-session",
		UserID:  "test-user",
		AppName: "test-app",
		State:   make(map[string]any),
		Events:  make([]*core.Event, 0),
	}

	invocationCtx := &core.InvocationContext{
		InvocationID: "test-invocation",
		Session:      session,
		Context:      ctx,
		UserContent: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr("Hello"),
				},
			},
		},
	}

	// Should return an error for empty agents
	_, err := sequential.Run(invocationCtx)
	if err == nil {
		t.Error("Expected error for sequential agent with no sub-agents")
	}
}

// TestSequentialAgent_CancellationHandling tests context cancellation
func TestSequentialAgent_CancellationHandling(t *testing.T) {
	// Create agents that would normally run for a long time
	student := NewMockAgent("Student", "Ask questions", "Question %d")
	teacher := NewMockAgent("Teacher", "Answer questions", "Answer %d")

	sequential := NewSequentialAgent("CancelTest", "Test cancellation",
		[]core.BaseAgent{student, teacher}, 10) // 10 rounds

	// Create context with quick timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	session := &core.Session{
		ID:      "test-session",
		UserID:  "test-user",
		AppName: "test-app",
		State:   make(map[string]any),
		Events:  make([]*core.Event, 0),
	}

	invocationCtx := &core.InvocationContext{
		InvocationID: "test-invocation",
		Session:      session,
		Context:      ctx,
		UserContent: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr("Start session"),
				},
			},
		},
	}

	// Should be cancelled due to timeout
	_, err := sequential.Run(invocationCtx)
	if err == nil {
		t.Error("Expected cancellation error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected deadline exceeded error, got %v", err)
	}
}

// TestSequentialAgent_AgentManagement tests adding and removing agents
func TestSequentialAgent_AgentManagement(t *testing.T) {
	student := NewMockAgent("Student", "Ask questions", "Question %d")
	teacher := NewMockAgent("Teacher", "Answer questions", "Answer %d")

	sequential := NewSequentialAgent("ManagementTest", "Test agent management",
		[]core.BaseAgent{student}, 1)

	// Verify initial state
	if len(sequential.Agents()) != 1 {
		t.Errorf("Expected 1 agent initially, got %d", len(sequential.Agents()))
	}

	// Add another agent
	sequential.AddAgent(teacher)
	if len(sequential.Agents()) != 2 {
		t.Errorf("Expected 2 agents after adding, got %d", len(sequential.Agents()))
	}

	// Remove an agent
	removed := sequential.RemoveAgent("Student")
	if !removed {
		t.Error("Expected agent removal to succeed")
	}

	if len(sequential.Agents()) != 1 {
		t.Errorf("Expected 1 agent after removal, got %d", len(sequential.Agents()))
	}

	// Try to remove non-existent agent
	removed = sequential.RemoveAgent("NonExistent")
	if removed {
		t.Error("Expected agent removal to fail for non-existent agent")
	}
}

// TestSequentialAgent_EventMetadata tests that events have proper A2A metadata
func TestSequentialAgent_EventMetadata(t *testing.T) {
	student := NewMockAgent("Student", "Ask questions", "Question %d")
	teacher := NewMockAgent("Teacher", "Answer questions", "Answer %d")

	sequential := NewSequentialAgent("MetadataTest", "Test A2A metadata",
		[]core.BaseAgent{student, teacher}, 1)

	ctx := context.Background()
	session := &core.Session{
		ID:      "test-session",
		UserID:  "test-user",
		AppName: "test-app",
		State:   make(map[string]any),
		Events:  make([]*core.Event, 0),
	}

	invocationCtx := &core.InvocationContext{
		InvocationID: "test-invocation",
		Session:      session,
		Context:      ctx,
		UserContent: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr("Test metadata"),
				},
			},
		},
	}

	events, err := sequential.Run(invocationCtx)
	if err != nil {
		t.Fatalf("Sequential agent execution failed: %v", err)
	}

	// Check metadata on agent events (skip completion event)
	agentEventCount := 0
	for _, event := range events {
		if event.Author != "MetadataTest" { // Not the completion event
			agentEventCount++

			// Verify required A2A metadata
			requiredMetadata := []string{
				"a2a:sequential_agent",
				"a2a:round",
				"a2a:agent_index",
				"a2a:agent_name",
				"a2a:role",
			}

			for _, key := range requiredMetadata {
				if _, exists := event.CustomMetadata[key]; !exists {
					t.Errorf("Event missing required metadata: %s", key)
				}
			}

			// Verify branch is set
			if event.Branch == nil {
				t.Error("Event should have branch set")
			} else if *event.Branch == "" {
				t.Error("Event branch should not be empty")
			}
		}
	}

	if agentEventCount != 2 {
		t.Errorf("Expected 2 agent events, got %d", agentEventCount)
	}
}

// TestSequentialAgent_SessionStateManagement tests state management across agents
func TestSequentialAgent_SessionStateManagement(t *testing.T) {
	// Create agents that modify state
	student := NewMockAgent("Student", "Ask questions", "Question %d")
	teacher := NewMockAgent("Teacher", "Answer questions", "Answer %d")

	sequential := NewSequentialAgent("StateTest", "Test state management",
		[]core.BaseAgent{student, teacher}, 1)

	ctx := context.Background()
	session := &core.Session{
		ID:      "test-session",
		UserID:  "test-user",
		AppName: "test-app",
		State:   make(map[string]any),
		Events:  make([]*core.Event, 0),
	}

	// Set initial state
	session.SetState("topic", "geography")
	session.SetState("question_count", 0)

	invocationCtx := &core.InvocationContext{
		InvocationID: "test-invocation",
		Session:      session,
		Context:      ctx,
		UserContent: &core.Content{
			Role: "user",
			Parts: []core.Part{
				{
					Type: "text",
					Text: ptr.Ptr("Let's study geography"),
				},
			},
		},
	}

	_, err := sequential.Run(invocationCtx)
	if err != nil {
		t.Fatalf("Sequential agent execution failed: %v", err)
	}

	// Verify state is preserved
	topic, exists := session.GetState("topic")
	if !exists {
		t.Error("Topic should be preserved in state")
	}

	if topic != "geography" {
		t.Errorf("Expected topic to be 'geography', got %v", topic)
	}

	// Verify events were added to session
	if len(session.Events) == 0 {
		t.Error("Events should be added to session")
	}
}
