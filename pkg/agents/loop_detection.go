// Package agents provides enhanced LLM agent implementation with comprehensive tool execution
// and sophisticated loop detection mechanisms.
//
// # Loop Detection in Enhanced LLM Agent
//
// The Enhanced LLM Agent includes sophisticated loop detection to prevent infinite loops
// during conversation flows. This is critical for production environments where agents
// could potentially get stuck in repetitive behavior patterns.
//
// ## Overview
//
// The loop detection system handles three main scenarios:
//  1. The agent makes too many tool calls in total across the conversation
//  2. The agent calls the same tool repeatedly (pattern detection)
//  3. The conversation exceeds the maximum number of turns
//
// ## Architecture
//
// The loop detection is implemented using three main components that follow SOLID principles:
//
// ### ConversationFlowManager
//
// The ConversationFlowManager orchestrates the conversation flow and coordinates between
// different loop detection mechanisms:
//
//	flowManager := NewConversationFlowManager(agent, invocationCtx)
//	// Manages turn limits and overall flow control
//	// Coordinates between different loop detection mechanisms
//
// ### LoopDetector
//
// The LoopDetector provides two key detection mechanisms:
//
// Tool Call Limit Detection tracks the total number of tool calls across the conversation:
//
//	func (ld *LoopDetector) CheckToolCallLimit(functionCalls []*core.FunctionCall, maxToolCalls int) bool {
//	    ld.totalToolCalls += len(functionCalls)
//	    return ld.totalToolCalls > maxToolCalls
//	}
//
// Repeating Pattern Detection identifies when the same tool is called consecutively:
//
//	func (ld *LoopDetector) CheckRepeatingPattern(events []*core.Event, turn int) bool {
//	    // Analyzes the last 6 events to identify patterns
//	    // Triggers when the same tool is called 3+ times in a row
//	}
//
// ### EventPublisher
//
// The EventPublisher handles event creation and publishing, creating appropriate
// final response events when loops are detected:
//
//	func (ep *EventPublisher) CreateFinalResponse(invocationID, agentName, message string) *core.Event {
//	    // Creates graceful termination events with meaningful messages
//	}
//
// ## Detection Mechanisms
//
// ### 1. Tool Call Limit Detection
//
// Tracks the total number of tool calls across the entire conversation:
//   - Default limit: MaxToolCalls * 2 (configurable via agent config)
//   - Prevents excessive tool usage that could lead to infinite loops
//   - Provides resource protection against runaway agents
//
// ### 2. Repeating Pattern Detection
//
// Analyzes conversation history to identify problematic patterns:
//   - Analyzes the last 6 events to identify patterns
//   - Triggers when the same tool is called 3+ times in a row
//   - Helps identify stuck behavior patterns
//   - Prevents agents from getting trapped in repetitive loops
//
// ### 3. Max Turns Limit
//
// Provides natural conversation termination:
//   - Default: 10 turns (configurable via RunConfig.MaxTurns)
//   - Prevents conversations from running indefinitely
//   - Ensures bounded execution time
//
// ## Configuration
//
// Loop detection behavior can be configured through LlmAgentConfig and RunConfig:
//
//	config := &LlmAgentConfig{
//	    MaxToolCalls:    10,                    // Per-turn limit
//	    ToolCallTimeout: 30 * time.Second,     // Individual tool timeout
//	    // Total limit calculated as MaxToolCalls * 2
//	}
//
//	invocationCtx.RunConfig = &core.RunConfig{
//	    MaxTurns: ptr.Ptr(15),                  // Maximum conversation turns
//	}
//
// ## Example Scenarios
//
// ### Scenario 1: Repeating Tool Calls
//
//	Turn 1: LLM calls get_weather()
//	Turn 2: LLM calls get_weather() again
//	Turn 3: LLM calls get_weather() again
//	-> Loop detected! Conversation terminated with appropriate message
//
// ### Scenario 2: Tool Call Limit Exceeded
//
//	Conversation with 20+ tool calls across multiple turns
//	-> Total limit exceeded, conversation terminated
//
// ### Scenario 3: Max Turns Exceeded
//
//	Conversation reaches 10+ turns (default limit)
//	-> Natural termination due to turn limit
//
// ## Testing
//
// The loop detection functionality is thoroughly tested with unit tests:
//
//   - TestLoopDetector_CheckToolCallLimit: Verifies tool call limit detection
//   - TestLoopDetector_CheckRepeatingPattern: Tests pattern detection logic
//   - TestEnhancedLlmAgent_LoopDetection_*: Integration tests for real conversation scenarios
//
// Run tests with:
//
//	go test ./pkg/agents/ -v -run "TestLoopDetector"
//	go test ./pkg/agents/ -v -run "TestEnhancedLlmAgent_LoopDetection"
//
// ## Benefits
//
//  1. Prevents Infinite Loops: Multiple safety mechanisms ensure conversations don't run indefinitely
//  2. Graceful Termination: When loops are detected, the agent provides meaningful final responses
//  3. Configurable Limits: All limits can be adjusted based on use case requirements
//  4. Pattern Recognition: Intelligent detection of problematic tool usage patterns
//  5. Resource Protection: Prevents excessive API calls and resource consumption
//  6. Production Ready: Robust safeguards for production deployments
//
// ## SOLID Principles Applied
//
//   - Single Responsibility: Each component has a focused purpose
//   - Open/Closed: Easy to extend with new loop detection strategies
//   - Dependency Inversion: Components depend on abstractions, not concrete implementations
//
// The refactored architecture makes the loop detection logic clean, testable, and
// maintainable while following Go best practices and SOLID principles.
package agents

import (
	"log"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// LoopDetector handles loop detection logic
type LoopDetector struct {
	totalToolCalls int
}

// NewLoopDetector creates a new loop detector
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		totalToolCalls: 0,
	}
}

// CheckToolCallLimit checks if the tool call limit has been exceeded
func (ld *LoopDetector) CheckToolCallLimit(functionCalls []*core.FunctionCall, maxToolCalls int) bool {
	ld.totalToolCalls += len(functionCalls)
	return ld.totalToolCalls > maxToolCalls
}

// CheckRepeatingPattern checks for repeating tool call patterns
func (ld *LoopDetector) CheckRepeatingPattern(events []*core.Event, turn int) bool {
	if turn <= 2 || len(events) < 4 {
		return false
	}

	// Look for pattern where the same function is called multiple times in a row
	var lastFunctionName string
	consecutiveCallCount := 0

	// Check the last few events for repeated function calls
	for i := len(events) - 1; i >= 0 && i >= len(events)-6; i-- {
		if events[i].Content == nil || events[i].Content.Role != "assistant" {
			continue
		}

		functionCalls := events[i].GetFunctionCalls()
		if len(functionCalls) == 0 {
			continue
		}

		currentFunctionName := functionCalls[0].Name
		if lastFunctionName == "" {
			lastFunctionName = currentFunctionName
			consecutiveCallCount = 1
		} else if lastFunctionName == currentFunctionName {
			consecutiveCallCount++
			if consecutiveCallCount >= 3 {
				log.Printf("Detected loop: function %s called %d times consecutively", currentFunctionName, consecutiveCallCount)
				return true
			}
		} else {
			lastFunctionName = currentFunctionName
			consecutiveCallCount = 1
		}
	}

	return false
}
