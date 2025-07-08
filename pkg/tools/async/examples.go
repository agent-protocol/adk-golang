// Package async provides examples of async tool implementations.
package async

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
)

// FileProcessorTool demonstrates a long-running file processing tool.
type FileProcessorTool struct {
	*StreamingTool
}

// NewFileProcessorTool creates a new file processor tool.
func NewFileProcessorTool() *FileProcessorTool {
	tool := &FileProcessorTool{
		StreamingTool: NewStreamingTool(
			"file_processor",
			"Processes files with progress updates",
			5, // max 5 concurrent operations
		),
	}

	// Set the custom execution function
	tool.SetExecuteFunc(tool.executeFileProcessing)
	return tool
}

// GetDeclaration returns the function declaration for LLM integration.
func (t *FileProcessorTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "file_processor",
		Description: "Processes files with real-time progress updates",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file to process",
				},
				"operation": map[string]any{
					"type":        "string",
					"description": "Operation to perform: 'analyze', 'compress', 'convert'",
					"enum":        []string{"analyze", "compress", "convert"},
				},
				"options": map[string]any{
					"type":        "object",
					"description": "Additional options for the operation",
				},
			},
			"required": []string{"file_path", "operation"},
		},
	}
}

// executeInternal overrides the base implementation with actual file processing logic.
func (t *FileProcessorTool) executeFileProcessing(ctx context.Context, args map[string]any, toolCtx *core.ToolContext, progressChan chan<- *ToolProgress, toolID string) (any, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("file_path is required and must be a string")
	}

	operation, ok := args["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation is required and must be a string")
	}

	// Simulate file processing with realistic progress updates
	stages := []string{
		"Reading file metadata",
		"Loading file content",
		"Processing data",
		"Applying transformations",
		"Writing results",
		"Cleanup and finalization",
	}

	result := map[string]any{
		"file_path":    filePath,
		"operation":    operation,
		"processed_at": time.Now(),
		"size_mb":      rand.Float64() * 100, // Simulated file size
		"duration_ms":  0,
	}

	startTime := time.Now()

	for i, stage := range stages {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Send progress update
		progress := float64(i) / float64(len(stages))
		progressChan <- &ToolProgress{
			ID:       toolID,
			Progress: progress,
			Message:  stage,
			Metadata: map[string]any{
				"stage":     i + 1,
				"total":     len(stages),
				"file_path": filePath,
				"operation": operation,
			},
			Timestamp:  time.Now(),
			Cancelable: true,
		}

		// Simulate processing time (varies by stage)
		var processingTime time.Duration
		switch i {
		case 0, 1: // Metadata and loading are fast
			processingTime = time.Duration(rand.Intn(500)) * time.Millisecond
		case 2, 3: // Processing and transformations take longer
			processingTime = time.Duration(1000+rand.Intn(2000)) * time.Millisecond
		case 4, 5: // Writing and cleanup are medium
			processingTime = time.Duration(500+rand.Intn(1000)) * time.Millisecond
		}

		// Wait for processing time with context cancellation support
		timer := time.NewTimer(processingTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
			// Continue to next stage
		}
	}

	// Send final progress
	progressChan <- &ToolProgress{
		ID:       toolID,
		Progress: 1.0,
		Message:  "Processing complete",
		Metadata: map[string]any{
			"duration_ms": time.Since(startTime).Milliseconds(),
		},
		Timestamp:  time.Now(),
		Cancelable: false,
	}

	result["duration_ms"] = time.Since(startTime).Milliseconds()
	return result, nil
}

// WebScraperTool demonstrates another async tool with different characteristics.
type WebScraperTool struct {
	*StreamingTool
	userAgent string
}

// NewWebScraperTool creates a new web scraper tool.
func NewWebScraperTool() *WebScraperTool {
	tool := &WebScraperTool{
		StreamingTool: NewStreamingTool(
			"web_scraper",
			"Scrapes web pages with progress tracking",
			3, // max 3 concurrent scraping operations
		),
		userAgent: "ADK-Go WebScraper/1.0",
	}

	// Set the custom execution function
	tool.SetExecuteFunc(tool.executeWebScraping)
	return tool
}

// GetDeclaration returns the function declaration for LLM integration.
func (t *WebScraperTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        "web_scraper",
		Description: "Scrapes web pages and extracts content with progress updates",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to scrape",
				},
				"selectors": map[string]any{
					"type":        "array",
					"description": "CSS selectors to extract specific content",
					"items": map[string]any{
						"type": "string",
					},
				},
				"max_pages": map[string]any{
					"type":        "integer",
					"description": "Maximum number of pages to scrape (for pagination)",
					"default":     1,
				},
			},
			"required": []string{"url"},
		},
	}
}

// executeInternal implements web scraping with progress updates.
func (t *WebScraperTool) executeWebScraping(ctx context.Context, args map[string]any, toolCtx *core.ToolContext, progressChan chan<- *ToolProgress, toolID string) (any, error) {
	url, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required and must be a string")
	}

	maxPages := 1
	if mp, exists := args["max_pages"]; exists {
		if mpInt, ok := mp.(int); ok {
			maxPages = mpInt
		}
	}

	var selectors []string
	if sels, exists := args["selectors"]; exists {
		if selArray, ok := sels.([]interface{}); ok {
			for _, sel := range selArray {
				if selStr, ok := sel.(string); ok {
					selectors = append(selectors, selStr)
				}
			}
		}
	}

	result := map[string]any{
		"url":           url,
		"pages_scraped": 0,
		"content":       []map[string]any{},
		"scraped_at":    time.Now(),
	}

	// Simulate scraping multiple pages
	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		currentURL := fmt.Sprintf("%s?page=%d", url, page)
		progress := float64(page-1) / float64(maxPages)

		progressChan <- &ToolProgress{
			ID:       toolID,
			Progress: progress,
			Message:  fmt.Sprintf("Scraping page %d of %d", page, maxPages),
			Metadata: map[string]any{
				"current_url": currentURL,
				"page":        page,
				"total_pages": maxPages,
			},
			Timestamp:  time.Now(),
			Cancelable: true,
		}

		// Simulate HTTP request and parsing time
		requestTime := time.Duration(500+rand.Intn(2000)) * time.Millisecond
		timer := time.NewTimer(requestTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		// Simulate extracted content
		pageContent := map[string]any{
			"url":     currentURL,
			"title":   fmt.Sprintf("Page %d Title", page),
			"content": fmt.Sprintf("Extracted content from page %d", page),
			"links":   rand.Intn(20) + 5, // Random number of links
			"images":  rand.Intn(10),     // Random number of images
		}

		if len(selectors) > 0 {
			pageContent["selected_elements"] = make(map[string]string)
			for _, selector := range selectors {
				pageContent["selected_elements"].(map[string]string)[selector] = fmt.Sprintf("Content for %s", selector)
			}
		}

		result["content"] = append(result["content"].([]map[string]any), pageContent)
		result["pages_scraped"] = page
	}

	// Send completion progress
	progressChan <- &ToolProgress{
		ID:       toolID,
		Progress: 1.0,
		Message:  fmt.Sprintf("Scraping complete - processed %d pages", maxPages),
		Metadata: map[string]any{
			"total_pages":   maxPages,
			"total_content": len(result["content"].([]map[string]any)),
		},
		Timestamp:  time.Now(),
		Cancelable: false,
	}

	return result, nil
}
