// Package tools provides local search tool implementation.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// DuckDuckGoSearchTool implements a local web search using DuckDuckGo Instant Answer API
type DuckDuckGoSearchTool struct {
	*BaseToolImpl
	client *http.Client
}

// SearchResult represents a search result
type SearchResult struct {
	Query   string   `json:"query"`
	Results []Result `json:"results"`
}

// Result represents a single search result
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// DuckDuckGoResponse represents the response from DuckDuckGo API
type DuckDuckGoResponse struct {
	Abstract      string                   `json:"Abstract"`
	AbstractText  string                   `json:"AbstractText"`
	AbstractURL   string                   `json:"AbstractURL"`
	Answer        string                   `json:"Answer"`
	AnswerType    string                   `json:"AnswerType"`
	Definition    string                   `json:"Definition"`
	Entity        string                   `json:"Entity"`
	Heading       string                   `json:"Heading"`
	Image         string                   `json:"Image"`
	ImageHeight   interface{}              `json:"ImageHeight"` // Can be int or string
	ImageIsLogo   interface{}              `json:"ImageIsLogo"` // Can be int or string
	ImageWidth    interface{}              `json:"ImageWidth"`  // Can be int or string
	Infobox       interface{}              `json:"Infobox"`     // Can be map or string
	Redirect      string                   `json:"Redirect"`
	RelatedTopics []map[string]interface{} `json:"RelatedTopics"`
	Results       []map[string]interface{} `json:"Results"`
	Type          string                   `json:"Type"`
}

// NewDuckDuckGoSearchTool creates a new local search tool
func NewDuckDuckGoSearchTool() *DuckDuckGoSearchTool {
	return &DuckDuckGoSearchTool{
		BaseToolImpl: NewBaseTool("google_search", "Search the web for current information"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetDeclaration returns the function declaration for this tool
func (t *DuckDuckGoSearchTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "google_search",
		Description: "Search the web for current information about any topic",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query to find information about",
				},
			},
			"required": []string{"query"},
		},
	}
}

// RunAsync executes the search
func (t *DuckDuckGoSearchTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	log.Println("Starting RunAsync for DuckDuckGoSearchTool...")
	// Extract query from args
	queryInterface, ok := args["query"]
	if !ok {
		log.Println("Missing required parameter 'query'")
		return nil, fmt.Errorf("missing required parameter 'query'")
	}

	query, ok := queryInterface.(string)
	if !ok {
		log.Println("Parameter 'query' must be a string")
		return nil, fmt.Errorf("parameter 'query' must be a string")
	}

	if query == "" {
		log.Println("Query cannot be empty")
		return nil, fmt.Errorf("query cannot be empty")
	}

	log.Printf("Performing search for query: %s", query)
	// Perform search
	results, err := t.search(ctx, query)
	if err != nil {
		log.Printf("Search failed: %v", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}

	log.Println("Search completed successfully.")
	return results, nil
}

// search performs the actual web search using DuckDuckGo Instant Answer API
func (t *DuckDuckGoSearchTool) search(ctx context.Context, query string) (*SearchResult, error) {
	// Use DuckDuckGo Instant Answer API
	// This provides basic search results without requiring API keys
	baseURL := "https://api.duckduckgo.com/"
	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	params.Add("no_html", "1")
	params.Add("skip_disambig", "1")

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "ADK-Golang/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ddgResp DuckDuckGoResponse
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert DuckDuckGo response to our format
	searchResult := &SearchResult{
		Query:   query,
		Results: make([]Result, 0),
	}

	// Add abstract as first result if available
	if ddgResp.Abstract != "" {
		searchResult.Results = append(searchResult.Results, Result{
			Title:   ddgResp.Heading,
			URL:     ddgResp.AbstractURL,
			Snippet: ddgResp.Abstract,
		})
	}

	// Add answer if available
	if ddgResp.Answer != "" {
		searchResult.Results = append(searchResult.Results, Result{
			Title:   "Direct Answer",
			URL:     "",
			Snippet: ddgResp.Answer,
		})
	}

	// Add definition if available
	if ddgResp.Definition != "" {
		searchResult.Results = append(searchResult.Results, Result{
			Title:   "Definition",
			URL:     ddgResp.AbstractURL, // Use AbstractURL for definition source
			Snippet: ddgResp.Definition,
		})
	}

	// Add related topics
	for i, topic := range ddgResp.RelatedTopics {
		if i >= 3 { // Limit to 3 related topics
			break
		}
		if topicText, ok := topic["Text"].(string); ok && topicText != "" {
			result := Result{
				Title:   "Related Information",
				Snippet: topicText,
			}
			if firstURL, ok := topic["FirstURL"].(string); ok {
				result.URL = firstURL
			}
			searchResult.Results = append(searchResult.Results, result)
		}
	}

	// If no results found, provide a helpful message
	if len(searchResult.Results) == 0 {
		searchResult.Results = append(searchResult.Results, Result{
			Title:   "Search Results",
			URL:     "",
			Snippet: fmt.Sprintf("I searched for '%s' but couldn't find specific information. You may want to try a more specific query or check the web directly.", query),
		})
	}

	return searchResult, nil
}

// ProcessLLMRequest is not needed for local tools
func (t *DuckDuckGoSearchTool) ProcessLLMRequest(ctx context.Context, toolCtx *core.ToolContext, request *core.LLMRequest) error {
	// This tool doesn't need to modify the LLM request
	return nil
}
