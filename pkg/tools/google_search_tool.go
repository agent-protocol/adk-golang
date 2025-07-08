// Package tools provides concrete implementations of tool types.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// GoogleSearchTool is a built-in tool that automatically invokes Google Search through Gemini models.
// This tool operates internally within the model and does not require local code execution.
// It modifies the LLM request to include Google Search capabilities.
type GoogleSearchTool struct {
	*BaseToolImpl
}

// NewGoogleSearchTool creates a new Google Search tool instance.
func NewGoogleSearchTool() *GoogleSearchTool {
	return &GoogleSearchTool{
		BaseToolImpl: NewBaseTool("google_search", "Built-in Google Search tool for Gemini models"),
	}
}

// GetDeclaration returns nil since this is a built-in tool that doesn't use function declarations.
// Instead, it modifies the LLM request configuration directly.
func (t *GoogleSearchTool) GetDeclaration() *core.FunctionDeclaration {
	return nil
}

// RunAsync is not implemented for this tool since it operates as a built-in model capability.
func (t *GoogleSearchTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	return nil, fmt.Errorf("GoogleSearchTool operates as a built-in model capability and does not execute locally")
}

// ProcessLLMRequest modifies the LLM request to include Google Search capabilities.
// This method adds the appropriate search configuration based on the model version.
func (t *GoogleSearchTool) ProcessLLMRequest(ctx context.Context, toolCtx *core.ToolContext, request *core.LLMRequest) error {
	// Ensure config exists
	if request.Config == nil {
		request.Config = &core.LLMConfig{}
	}

	// Get the model name from config
	model := request.Config.Model
	if model == "" {
		return fmt.Errorf("model name is required for Google Search tool")
	}

	// Check if this is a Gemini model
	if !strings.Contains(strings.ToLower(model), "gemini") {
		return fmt.Errorf("google search tool is only supported for Gemini models, got: %s", model)
	}

	// For Gemini 1.x models, Google Search cannot be used with other tools
	if strings.Contains(strings.ToLower(model), "gemini-1") {
		if len(request.Config.Tools) > 0 {
			return fmt.Errorf("google search tool cannot be used with other tools in Gemini 1.x models")
		}
		// Add Google Search Retrieval for Gemini 1.x
		t.addGoogleSearchRetrieval(request)
	} else if strings.Contains(strings.ToLower(model), "gemini-") {
		// Add Google Search for Gemini 2.x+
		t.addGoogleSearch(request)
	} else {
		return fmt.Errorf("google search tool is not supported for model: %s", model)
	}

	return nil
}

// addGoogleSearchRetrieval adds Google Search Retrieval configuration for Gemini 1.x models.
func (t *GoogleSearchTool) addGoogleSearchRetrieval(request *core.LLMRequest) {
	// Initialize tools slice if needed
	if request.Config.Tools == nil {
		request.Config.Tools = make([]*core.FunctionDeclaration, 0)
	}

	// Add a special marker for Google Search Retrieval
	// In a real implementation, this would be replaced with the actual Google API types
	googleSearchRetrieval := &core.FunctionDeclaration{
		Name:        "_google_search_retrieval",
		Description: "Built-in Google Search Retrieval for Gemini 1.x",
		Parameters: map[string]interface{}{
			"type": "google_search_retrieval",
		},
	}

	request.Config.Tools = append(request.Config.Tools, googleSearchRetrieval)
}

// addGoogleSearch adds Google Search configuration for Gemini 2.x+ models.
func (t *GoogleSearchTool) addGoogleSearch(request *core.LLMRequest) {
	// Initialize tools slice if needed
	if request.Config.Tools == nil {
		request.Config.Tools = make([]*core.FunctionDeclaration, 0)
	}

	// Add a special marker for Google Search
	// In a real implementation, this would be replaced with the actual Google API types
	googleSearch := &core.FunctionDeclaration{
		Name:        "_google_search",
		Description: "Built-in Google Search for Gemini 2.x+",
		Parameters: map[string]interface{}{
			"type": "google_search",
		},
	}

	request.Config.Tools = append(request.Config.Tools, googleSearch)
}

// GlobalGoogleSearchTool is a global instance of the Google Search tool.
var GlobalGoogleSearchTool = NewGoogleSearchTool()
