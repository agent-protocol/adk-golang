package agents

import (
	"context"
	"testing"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

func TestBaseAgentInterface(t *testing.T) {
	ctx := context.Background()

	// Create a session service
	sessionService := sessions.NewInMemorySessionService()

	// Create a session
	session, err := sessionService.CreateSession(ctx, &core.CreateSessionRequest{
		AppName: "test_app",
		UserID:  "test_user",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create a base agent
	agent := NewCustomAgent("test_agent", "A test agent")
	agent.SetInstruction("Test instruction")

	// Test basic properties
	if agent.Name() != "test_agent" {
		t.Errorf("Expected name 'test_agent', got '%s'", agent.Name())
	}

	if agent.Description() != "A test agent" {
		t.Errorf("Expected description 'A test agent', got '%s'", agent.Description())
	}

	if agent.Instruction() != "Test instruction" {
		t.Errorf("Expected instruction 'Test instruction', got '%s'", agent.Instruction())
	}

	// Test agent hierarchy
	subAgent := NewCustomAgent("sub_agent", "A sub agent")
	agent.AddSubAgent(subAgent)

	if len(agent.SubAgents()) != 1 {
		t.Errorf("Expected 1 sub-agent, got %d", len(agent.SubAgents()))
	}

	if subAgent.ParentAgent() != agent {
		t.Errorf("Expected parent agent to be set correctly")
	}

	// Test FindAgent functionality
	found := agent.FindAgent("test_agent")
	if found != agent {
		t.Errorf("FindAgent should find the root agent")
	}

	found = agent.FindAgent("sub_agent")
	if found != subAgent {
		t.Errorf("FindAgent should find the sub-agent")
	}

	found = agent.FindAgent("nonexistent")
	if found != nil {
		t.Errorf("FindAgent should return nil for nonexistent agent")
	}

	// Test FindSubAgent functionality
	found = agent.FindSubAgent("sub_agent")
	if found != subAgent {
		t.Errorf("FindSubAgent should find the sub-agent")
	}

	found = agent.FindSubAgent("test_agent")
	if found != nil {
		t.Errorf("FindSubAgent should not find the root agent")
	}

	// Test callbacks
	callbackCalled := false
	agent.SetBeforeAgentCallback(func(invocationCtx *core.InvocationContext) error {
		callbackCalled = true
		return nil
	})

	// Create invocation context
	invocationCtx := core.NewInvocationContext(
		ctx,
		"test_invocation",
		agent,
		session,
		sessionService,
	)

	// Run the agent
	events, err := agent.Run(invocationCtx)
	if err != nil {
		t.Fatalf("Failed to run agent: %v", err)
	}

	if !callbackCalled {
		t.Errorf("Before agent callback should have been called")
	}

	if len(events) == 0 {
		t.Errorf("Expected at least one event")
	}

	// Verify event properties
	event := events[0]
	if event.Author != "test_agent" {
		t.Errorf("Expected event author 'test_agent', got '%s'", event.Author)
	}

	if event.InvocationID != "test_invocation" {
		t.Errorf("Expected invocation ID 'test_invocation', got '%s'", event.InvocationID)
	}
}

func TestInvocationContext(t *testing.T) {
	ctx := context.Background()
	sessionService := sessions.NewInMemorySessionService()

	session, err := sessionService.CreateSession(ctx, &core.CreateSessionRequest{
		AppName: "test_app",
		UserID:  "test_user",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	agent := NewCustomAgent("test_agent", "A test agent")

	// Test InvocationContext creation
	invocationCtx := core.NewInvocationContext(
		ctx,
		"test_invocation",
		agent,
		session,
		sessionService,
	)

	if invocationCtx.InvocationID != "test_invocation" {
		t.Errorf("Expected invocation ID 'test_invocation', got '%s'", invocationCtx.InvocationID)
	}

	if invocationCtx.Agent != agent {
		t.Errorf("Expected agent to be set correctly")
	}

	if invocationCtx.Session != session {
		t.Errorf("Expected session to be set correctly")
	}

	// Test builder pattern methods
	userContent := &core.Content{
		Role: "user",
		Parts: []core.Part{
			{Type: "text", Text: ptr.Ptr("Hello")},
		},
	}

	invocationCtx = invocationCtx.
		WithUserContent(userContent).
		WithBranch("test_branch").
		WithRunConfig(&core.RunConfig{MaxTurns: ptr.Ptr(5)})

	if invocationCtx.UserContent != userContent {
		t.Errorf("Expected user content to be set correctly")
	}

	if invocationCtx.GetBranch() != "test_branch" {
		t.Errorf("Expected branch 'test_branch', got '%s'", invocationCtx.GetBranch())
	}

	if invocationCtx.RunConfig.MaxTurns == nil || *invocationCtx.RunConfig.MaxTurns != 5 {
		t.Errorf("Expected max turns to be 5")
	}

	// Test service availability checks
	if invocationCtx.HasArtifactService() {
		t.Errorf("Should not have artifact service")
	}

	if invocationCtx.HasMemoryService() {
		t.Errorf("Should not have memory service")
	}

	if invocationCtx.HasCredentialService() {
		t.Errorf("Should not have credential service")
	}

	// Test Clone functionality
	clone := invocationCtx.Clone()
	if clone.InvocationID != invocationCtx.InvocationID {
		t.Errorf("Clone should have same invocation ID")
	}

	if clone.GetBranch() != invocationCtx.GetBranch() {
		t.Errorf("Clone should have same branch")
	}

	// Modifying clone should not affect original
	clone.SetEndInvocation(true)
	if invocationCtx.IsEndInvocation() {
		t.Errorf("Original context should not be affected by clone modification")
	}

	// Test CreateSubContext
	subAgent := NewCustomAgent("sub_agent", "A sub agent")
	subCtx := invocationCtx.CreateSubContext(subAgent, "sub_branch")

	if subCtx.Agent != subAgent {
		t.Errorf("Sub context should have the sub agent")
	}

	expectedBranch := "test_branch.sub_branch"
	if subCtx.GetBranch() != expectedBranch {
		t.Errorf("Expected sub branch '%s', got '%s'", expectedBranch, subCtx.GetBranch())
	}
}

func TestAgentCallbacks(t *testing.T) {
	ctx := context.Background()
	sessionService := sessions.NewInMemorySessionService()

	session, err := sessionService.CreateSession(ctx, &core.CreateSessionRequest{
		AppName: "test_app",
		UserID:  "test_user",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	agent := NewCustomAgent("callback_agent", "Agent with callbacks")

	beforeCalled := false
	afterCalled := false
	var capturedEvents []*core.Event

	agent.SetBeforeAgentCallback(func(invocationCtx *core.InvocationContext) error {
		beforeCalled = true
		return nil
	})

	agent.SetAfterAgentCallback(func(invocationCtx *core.InvocationContext, events []*core.Event) error {
		afterCalled = true
		capturedEvents = events
		return nil
	})

	invocationCtx := core.NewInvocationContext(
		ctx,
		"callback_test",
		agent,
		session,
		sessionService,
	)

	events, err := agent.Run(invocationCtx)
	if err != nil {
		t.Fatalf("Failed to run agent: %v", err)
	}

	if !beforeCalled {
		t.Errorf("Before callback should have been called")
	}

	if !afterCalled {
		t.Errorf("After callback should have been called")
	}

	if len(capturedEvents) != len(events) {
		t.Errorf("After callback should receive all events")
	}
}

func TestAgentCleanup(t *testing.T) {
	ctx := context.Background()

	parentAgent := NewCustomAgent("parent", "Parent agent")
	subAgent := NewCustomAgent("sub", "Sub agent")

	parentAgent.AddSubAgent(subAgent)

	// Test cleanup
	err := parentAgent.Cleanup(ctx)
	if err != nil {
		t.Errorf("Cleanup should not return error: %v", err)
	}
}
