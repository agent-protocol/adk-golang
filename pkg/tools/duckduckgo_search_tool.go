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
	"regexp"
	"strings"
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
		BaseToolImpl: NewBaseTool("duckduckgo_search", "Search the web for current information"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetDeclaration returns the function declaration for this tool
func (t *DuckDuckGoSearchTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "duckduckgo_search",
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

// search performs the actual web search using DuckDuckGo search results
func (t *DuckDuckGoSearchTool) search(ctx context.Context, query string) (*SearchResult, error) {
	// First try DuckDuckGo Instant Answer API for direct answers
	instantResult, err := t.searchInstantAnswer(ctx, query)
	if err == nil && len(instantResult.Results) > 0 {
		// Check if we got a meaningful result (not just the fallback message)
		if len(instantResult.Results) == 1 &&
			instantResult.Results[0].Title == "Search Results" &&
			instantResult.Results[0].URL == "" {
			// Fall through to web search
		} else {
			return instantResult, nil
		}
	}

	// If instant answer didn't work, try web search
	return t.searchWeb(ctx, query)
}

// searchInstantAnswer uses the DuckDuckGo Instant Answer API for direct answers
func (t *DuckDuckGoSearchTool) searchInstantAnswer(ctx context.Context, query string) (*SearchResult, error) {
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
			URL:     ddgResp.AbstractURL,
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

// searchWeb performs web search by scraping DuckDuckGo search results
func (t *DuckDuckGoSearchTool) searchWeb(ctx context.Context, query string) (*SearchResult, error) {
	// Use DuckDuckGo Lite version which is simpler to parse
	baseURL := "https://lite.duckduckgo.com/lite/"
	params := url.Values{}
	params.Add("q", query)
	params.Add("kd", "-1") // No safe search

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle different status codes more gracefully
	if resp.StatusCode == 202 {
		// 202 means the request is accepted but might be rate limited
		// Return a meaningful response instead of error
		return &SearchResult{
			Query: query,
			Results: []Result{
				{
					Title:   "Search Request Accepted",
					URL:     "",
					Snippet: fmt.Sprintf("Your search for '%s' was accepted but may be temporarily rate-limited. Please try again in a moment.", query),
				},
			},
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("web search request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML to extract search results
	results, err := t.parseSearchResults(string(body), query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	return results, nil
}

// ProcessLLMRequest is not needed for local tools
func (t *DuckDuckGoSearchTool) ProcessLLMRequest(ctx context.Context, toolCtx *core.ToolContext, request *core.LLMRequest) error {
	// This tool doesn't need to modify the LLM request
	return nil
}

// parseSearchResults extracts search results from DuckDuckGo HTML response
func (t *DuckDuckGoSearchTool) parseSearchResults(html, query string) (*SearchResult, error) {
	searchResult := &SearchResult{
		Query:   query,
		Results: make([]Result, 0),
	}

	// DuckDuckGo Lite uses table-based layout with simpler structure
	// Look for search results in table rows
	results := findAllHTMLMatches(html, `<tr[^>]*>.*?<td[^>]*>.*?<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>.*?</td>.*?<td[^>]*class="result-snippet"[^>]*>(.*?)</td>.*?</tr>`)

	for i, match := range results {
		if i >= 8 { // Limit to 8 results
			break
		}

		if len(match) >= 4 {
			url := strings.TrimSpace(match[1])
			title := cleanHTML(match[2])
			snippet := cleanHTML(match[3])

			// Skip ads and sponsored results
			if strings.Contains(strings.ToLower(url), "duckduckgo.com") ||
				strings.Contains(strings.ToLower(title), "ad") ||
				strings.Contains(strings.ToLower(snippet), "sponsored") {
				continue
			}

			if title != "" && snippet != "" {
				searchResult.Results = append(searchResult.Results, Result{
					Title:   title,
					URL:     url,
					Snippet: snippet,
				})
			}
		}
	}

	// Try alternative pattern if table parsing didn't work
	if len(searchResult.Results) == 0 {
		// Look for any links with descriptions
		linkResults := findAllHTMLMatches(html, `<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)

		for i, match := range linkResults {
			if i >= 10 { // Check more links since we'll filter
				break
			}

			if len(match) >= 3 {
				url := strings.TrimSpace(match[1])
				title := cleanHTML(match[2])

				// Skip internal DuckDuckGo links and empty titles
				if strings.Contains(url, "duckduckgo.com") ||
					strings.HasPrefix(url, "/") ||
					strings.HasPrefix(url, "?") ||
					title == "" ||
					len(title) < 5 {
					continue
				}

				// Try to find snippet near this link
				snippet := t.findSnippetNearLink(html, url, title)

				if snippet != "" {
					searchResult.Results = append(searchResult.Results, Result{
						Title:   title,
						URL:     url,
						Snippet: snippet,
					})

					if len(searchResult.Results) >= 5 {
						break
					}
				}
			}
		}
	}

	// If no results found through HTML parsing, return a fallback
	if len(searchResult.Results) == 0 {
		searchResult.Results = append(searchResult.Results, Result{
			Title:   "Search Results",
			URL:     "",
			Snippet: fmt.Sprintf("Found search results for '%s', but couldn't extract detailed information. The search was successful but parsing is limited.", query),
		})
	}

	return searchResult, nil
}

// findNextIndex finds the next occurrence of pattern in text starting from start
func findNextIndex(text, pattern string, start int) int {
	if start >= len(text) {
		return -1
	}
	index := strings.Index(text[start:], pattern)
	if index == -1 {
		return -1
	}
	return start + index
}

// findMatchingDivEnd finds the closing div tag for a div starting at startIndex
func findMatchingDivEnd(html string, startIndex int) int {
	// Simple approach: find the next result div or end of results section
	// Look for the next "<div class=" or end of results
	nextDiv := strings.Index(html[startIndex+10:], `<div class="result`)
	if nextDiv != -1 {
		return startIndex + 10 + nextDiv
	}

	// Look for end of results section
	searchStart := startIndex + 100 // Skip the opening div
	depth := 1
	pos := searchStart

	for depth > 0 && pos < len(html) {
		nextOpen := strings.Index(html[pos:], `<div`)
		nextClose := strings.Index(html[pos:], `</div>`)

		if nextClose == -1 {
			break
		}

		if nextOpen != -1 && nextOpen < nextClose {
			depth++
			pos += nextOpen + 4
		} else {
			depth--
			pos += nextClose + 6
		}
	}

	if pos < len(html) {
		return pos
	}

	// Fallback: return a reasonable chunk
	end := startIndex + 1000
	if end > len(html) {
		end = len(html)
	}
	return end
}

// extractResultData extracts title, URL, and snippet from a result HTML block
func (t *DuckDuckGoSearchTool) extractResultData(resultHTML string) (title, url, snippet string) {
	// Extract title - usually in an <a> tag with class="result__a"
	titlePattern := `<a.*?class="result__a".*?href="([^"]*)".*?>(.*?)</a>`
	if titleMatch := findHTMLMatch(resultHTML, titlePattern); len(titleMatch) >= 3 {
		url = strings.TrimSpace(titleMatch[1])
		title = cleanHTML(titleMatch[2])
	}

	// Extract snippet - usually in a div with class="result__snippet"
	snippetPattern := `<div class="result__snippet".*?>(.*?)</div>`
	if snippetMatch := findHTMLMatch(resultHTML, snippetPattern); len(snippetMatch) >= 2 {
		snippet = cleanHTML(snippetMatch[1])
	}

	// Fallback patterns if the above don't work
	if title == "" {
		// Try alternative title patterns
		altTitlePattern := `<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`
		if titleMatch := findHTMLMatch(resultHTML, altTitlePattern); len(titleMatch) >= 3 {
			url = strings.TrimSpace(titleMatch[1])
			title = cleanHTML(titleMatch[2])
		}
	}

	if snippet == "" {
		// Try to find any text content as snippet
		textPattern := `<div[^>]*>(.*?)</div>`
		for _, match := range findAllHTMLMatches(resultHTML, textPattern) {
			if len(match) >= 2 {
				cleanText := cleanHTML(match[1])
				if len(cleanText) > 20 && !strings.Contains(strings.ToLower(cleanText), "advertisement") {
					snippet = cleanText
					break
				}
			}
		}
	}

	// Clean up extracted data
	title = strings.TrimSpace(title)
	snippet = strings.TrimSpace(snippet)
	url = strings.TrimSpace(url)

	// Limit snippet length
	if len(snippet) > 300 {
		snippet = snippet[:300] + "..."
	}

	return title, url, snippet
}

// findSnippetNearLink tries to find snippet text near a link in the HTML
func (t *DuckDuckGoSearchTool) findSnippetNearLink(html, url, title string) string {
	// Look for text content around the link
	linkIndex := strings.Index(html, url)
	if linkIndex == -1 {
		// Try with title
		linkIndex = strings.Index(html, title)
		if linkIndex == -1 {
			return ""
		}
	}

	// Look for text in the surrounding 800 characters
	start := linkIndex - 400
	if start < 0 {
		start = 0
	}
	end := linkIndex + 400
	if end > len(html) {
		end = len(html)
	}

	surrounding := html[start:end]

	// Look for description or snippet text patterns
	snippetPatterns := []string{
		`<span[^>]*class="[^"]*snippet[^"]*"[^>]*>(.*?)</span>`,
		`<div[^>]*class="[^"]*snippet[^"]*"[^>]*>(.*?)</div>`,
		`<td[^>]*class="[^"]*snippet[^"]*"[^>]*>(.*?)</td>`,
		`<p[^>]*>(.*?)</p>`,
		`>[^<]{30,200}<`, // Any text content between 30-200 chars
	}

	for _, pattern := range snippetPatterns {
		matches := findAllHTMLMatches(surrounding, pattern)
		for _, match := range matches {
			if len(match) >= 2 {
				text := cleanHTML(match[1])
				// Filter out bad content
				textLower := strings.ToLower(text)
				if len(text) > 20 &&
					!strings.Contains(textLower, "javascript") &&
					!strings.Contains(textLower, "cookie") &&
					!strings.Contains(textLower, "advertisement") &&
					!strings.Contains(textLower, "privacy") &&
					!strings.Contains(textLower, "terms") &&
					!strings.Contains(text, "T00:00:00") && // Skip timestamp formats
					!strings.HasPrefix(text, "http") && // Skip URLs
					!strings.Contains(text, ".com/") { // Skip partial URLs

					// Limit snippet length
					if len(text) > 200 {
						text = text[:200] + "..."
					}
					return text
				}
			}
		}
	}

	return "Information about " + title
}

// findHTMLMatch finds the first regex match in HTML
func findHTMLMatch(html, pattern string) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re.FindStringSubmatch(html)
}

// findAllHTMLMatches finds all regex matches in HTML
func findAllHTMLMatches(html, pattern string) [][]string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re.FindAllStringSubmatch(html, -1)
}

// cleanHTML removes HTML tags and decodes entities
func cleanHTML(html string) string {
	// Remove HTML tags
	re, err := regexp.Compile(`<[^>]*>`)
	if err != nil {
		return html
	}
	text := re.ReplaceAllString(html, "")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Clean up whitespace
	re2, err := regexp.Compile(`\s+`)
	if err != nil {
		return text
	}
	text = re2.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}
