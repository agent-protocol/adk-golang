// Package agents provides LLM system instruction templates for better tool usage.
package agents

import (
	"fmt"
	"strings"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// SystemInstructionBuilder helps create effective system instructions for LLM agents.
type SystemInstructionBuilder struct {
	baseInstruction string
	tools           []core.BaseTool
	strictMode      bool
	preventLoops    bool
}

// NewSystemInstructionBuilder creates a new instruction builder.
func NewSystemInstructionBuilder(baseInstruction string) *SystemInstructionBuilder {
	return &SystemInstructionBuilder{
		baseInstruction: baseInstruction,
		strictMode:      false,
		preventLoops:    true,
	}
}

// WithTools sets the available tools for instruction generation.
func (b *SystemInstructionBuilder) WithTools(tools []core.BaseTool) *SystemInstructionBuilder {
	b.tools = tools
	return b
}

// WithStrictMode enables strict mode for tool usage.
func (b *SystemInstructionBuilder) WithStrictMode(enabled bool) *SystemInstructionBuilder {
	b.strictMode = enabled
	return b
}

// WithLoopPrevention enables/disables loop prevention instructions.
func (b *SystemInstructionBuilder) WithLoopPrevention(enabled bool) *SystemInstructionBuilder {
	b.preventLoops = enabled
	return b
}

// Build constructs the complete system instruction.
func (b *SystemInstructionBuilder) Build() string {
	var parts []string

	// Add base instruction
	if b.baseInstruction != "" {
		parts = append(parts, b.baseInstruction)
	}

	// Add tool usage guidelines
	if len(b.tools) > 0 {
		parts = append(parts, b.buildToolInstructions())
	}

	// Add response format guidelines
	parts = append(parts, b.buildResponseFormatInstructions())

	// Add loop prevention if enabled
	if b.preventLoops {
		parts = append(parts, b.buildLoopPreventionInstructions())
	}

	return strings.Join(parts, "\n\n")
}

// buildToolInstructions creates instructions for tool usage.
func (b *SystemInstructionBuilder) buildToolInstructions() string {
	var sb strings.Builder

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to the following tools:\n\n")

	for _, tool := range b.tools {
		decl := tool.GetDeclaration()
		if decl != nil {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", decl.Name, decl.Description))
		}
	}

	sb.WriteString("\n## Tool Usage Guidelines\n")

	if b.strictMode {
		sb.WriteString("STRICT MODE: You MUST use tools when they are available and relevant to the user's request.\n\n")
		sb.WriteString("1. Always analyze if any available tool can help answer the user's question\n")
		sb.WriteString("2. If a relevant tool exists, you MUST use it instead of providing a direct answer\n")
		sb.WriteString("3. Only provide direct answers when no relevant tools are available\n")
	} else {
		sb.WriteString("1. **Analyze First**: Before responding, determine if any available tool can help answer the user's request\n")
		sb.WriteString("2. **Tool Priority**: If a tool is available and relevant, prefer using it over providing a direct answer\n")
		sb.WriteString("3. **Direct Response**: Only provide direct answers when:\n")
		sb.WriteString("   - No relevant tools are available\n")
		sb.WriteString("   - The question is general knowledge that doesn't require external data\n")
		sb.WriteString("   - You already have sufficient information to provide a complete answer\n")
	}

	sb.WriteString("\n4. **Tool Selection**: Choose the most appropriate tool based on the user's specific needs\n")
	sb.WriteString("5. **Parameter Accuracy**: Ensure all tool parameters are filled correctly with concrete values\n")

	return sb.String()
}

// buildResponseFormatInstructions creates instructions for response format.
func (b *SystemInstructionBuilder) buildResponseFormatInstructions() string {
	return `## Response Format Guidelines

When using tools:
- Call the appropriate function with the correct parameters
- Wait for the function result before providing your final answer
- Base your response on the actual tool results, not assumptions

When providing direct answers:
- Be clear and concise
- Provide helpful information based on your knowledge
- Acknowledge any limitations in your knowledge

Always structure your responses properly:
- Use clear, readable text
- Avoid outputting malformed JSON or code fragments
- Ensure your response is complete and helpful to the user`
}

// buildLoopPreventionInstructions creates instructions to prevent infinite loops.
func (b *SystemInstructionBuilder) buildLoopPreventionInstructions() string {
	return `## Important: Loop Prevention

To prevent getting stuck in loops:
1. Do not call the same tool repeatedly with identical parameters
2. If a tool fails, try a different approach or provide a direct answer
3. After using a tool, always provide a meaningful response to the user
4. If you're unsure about tool parameters, ask the user for clarification instead of guessing
5. Limit consecutive tool calls - if you've used 2-3 tools without resolving the query, summarize what you've learned and ask for guidance

Remember: Your goal is to be helpful to the user, not to use tools for the sake of using them.`
}

// IntelligentToolSelector helps decide when to use tools vs direct responses.
type IntelligentToolSelector struct {
	tools               []core.BaseTool
	recentToolCalls     []string
	maxConsecutiveCalls int
}

// NewIntelligentToolSelector creates a new tool selector.
func NewIntelligentToolSelector(tools []core.BaseTool) *IntelligentToolSelector {
	return &IntelligentToolSelector{
		tools:               tools,
		recentToolCalls:     make([]string, 0),
		maxConsecutiveCalls: 3,
	}
}

// ShouldUseTools analyzes if tools should be used for a given query.
func (s *IntelligentToolSelector) ShouldUseTools(query string, recentEvents []*core.Event) ToolUsageRecommendation {
	// Check for recent tool usage patterns
	recentCalls := s.analyzeRecentToolUsage(recentEvents)

	// Check for loop patterns
	if s.detectLoopPattern(recentCalls) {
		return ToolUsageRecommendation{
			ShouldUse:  false,
			Reason:     "Loop pattern detected - too many consecutive tool calls",
			Suggestion: "Provide a direct answer based on information already gathered",
		}
	}

	// Analyze query for tool relevance
	relevantTools := s.findRelevantTools(query)

	if len(relevantTools) == 0 {
		return ToolUsageRecommendation{
			ShouldUse:  false,
			Reason:     "No relevant tools available for this query",
			Suggestion: "Provide a direct answer based on your knowledge",
		}
	}

	// Check if query requires external data
	requiresExternalData := s.queryRequiresExternalData(query)

	return ToolUsageRecommendation{
		ShouldUse:     requiresExternalData,
		RelevantTools: relevantTools,
		Reason:        s.buildReasonForRecommendation(requiresExternalData, relevantTools),
		Suggestion:    s.buildSuggestionForRecommendation(requiresExternalData, relevantTools),
	}
}

// ToolUsageRecommendation provides guidance on tool usage.
type ToolUsageRecommendation struct {
	ShouldUse     bool
	RelevantTools []string
	Reason        string
	Suggestion    string
}

// analyzeRecentToolUsage examines recent events for tool usage patterns.
func (s *IntelligentToolSelector) analyzeRecentToolUsage(events []*core.Event) []string {
	var calls []string

	// Look at the last 5 events
	start := len(events) - 5
	if start < 0 {
		start = 0
	}

	for i := start; i < len(events); i++ {
		if events[i].Content != nil {
			for _, part := range events[i].Content.Parts {
				if part.FunctionCall != nil {
					calls = append(calls, part.FunctionCall.Name)
				}
			}
		}
	}

	return calls
}

// detectLoopPattern checks for repetitive tool calling patterns.
func (s *IntelligentToolSelector) detectLoopPattern(recentCalls []string) bool {
	if len(recentCalls) < 3 {
		return false
	}

	// Check for same function called multiple times in a row
	consecutiveCount := 1
	for i := 1; i < len(recentCalls); i++ {
		if recentCalls[i] == recentCalls[i-1] {
			consecutiveCount++
			if consecutiveCount >= 3 {
				return true
			}
		} else {
			consecutiveCount = 1
		}
	}

	return false
}

// findRelevantTools identifies tools that might be relevant to the query.
func (s *IntelligentToolSelector) findRelevantTools(query string) []string {
	var relevant []string
	queryLower := strings.ToLower(query)

	for _, tool := range s.tools {
		decl := tool.GetDeclaration()
		if decl != nil {
			// Simple keyword matching - could be enhanced with NLP
			toolDesc := strings.ToLower(decl.Description)
			toolName := strings.ToLower(decl.Name)

			// Check for keyword overlaps
			if s.hasKeywordOverlap(queryLower, toolDesc) ||
				s.hasKeywordOverlap(queryLower, toolName) {
				relevant = append(relevant, decl.Name)
			}
		}
	}

	return relevant
}

// queryRequiresExternalData determines if a query needs external data.
func (s *IntelligentToolSelector) queryRequiresExternalData(query string) bool {
	queryLower := strings.ToLower(query)

	// Keywords that typically indicate need for external data
	externalDataKeywords := []string{
		"current", "latest", "recent", "today", "now",
		"search", "find", "lookup", "get", "fetch",
		"what is", "weather", "price", "stock",
		"time", "date", "status", "check",
	}

	for _, keyword := range externalDataKeywords {
		if strings.Contains(queryLower, keyword) {
			return true
		}
	}

	return false
}

// hasKeywordOverlap checks for keyword overlap between query and tool description.
func (s *IntelligentToolSelector) hasKeywordOverlap(query, toolText string) bool {
	queryWords := strings.Fields(query)
	toolWords := strings.Fields(toolText)

	for _, qWord := range queryWords {
		if len(qWord) < 3 { // Skip short words
			continue
		}
		for _, tWord := range toolWords {
			if qWord == tWord {
				return true
			}
		}
	}

	return false
}

// buildReasonForRecommendation creates explanation for the recommendation.
func (s *IntelligentToolSelector) buildReasonForRecommendation(shouldUse bool, relevantTools []string) string {
	if shouldUse {
		return fmt.Sprintf("Query appears to require external data and relevant tools are available: %s",
			strings.Join(relevantTools, ", "))
	}
	return "Query can be answered with existing knowledge"
}

// buildSuggestionForRecommendation creates action suggestion.
func (s *IntelligentToolSelector) buildSuggestionForRecommendation(shouldUse bool, relevantTools []string) string {
	if shouldUse && len(relevantTools) > 0 {
		if len(relevantTools) == 1 {
			return fmt.Sprintf("Use the %s tool to get the required information", relevantTools[0])
		}
		return fmt.Sprintf("Consider using one of these tools: %s", strings.Join(relevantTools, ", "))
	}
	return "Provide a direct, helpful answer based on your knowledge"
}
