package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/tools/async"
)

func main() {
	fmt.Println("🚀 ADK Go Async Tool Demo")
	fmt.Println("==========================")

	ctx := context.Background()

	// Demo 1: Basic streaming tool execution
	fmt.Println("\n📁 Demo 1: File Processor Tool")
	if err := demonstrateFileProcessor(ctx); err != nil {
		log.Printf("File processor demo failed: %v", err)
	}

	// Demo 2: Concurrent tool execution
	fmt.Println("\n🌐 Demo 2: Concurrent Web Scraper Tools")
	if err := demonstrateConcurrentScraping(ctx); err != nil {
		log.Printf("Concurrent scraping demo failed: %v", err)
	}

	// Demo 3: Tool cancellation
	fmt.Println("\n❌ Demo 3: Tool Cancellation")
	if err := demonstrateCancellation(ctx); err != nil {
		log.Printf("Cancellation demo failed: %v", err)
	}

	// Demo 4: Tool with timeout
	fmt.Println("\n⏰ Demo 4: Tool with Timeout")
	if err := demonstrateTimeout(); err != nil {
		log.Printf("Timeout demo failed: %v", err)
	}

	fmt.Println("\n✅ All async tool demos completed!")
}

func stringPtr(s string) *string {
	return &s
}

func demonstrateFileProcessor(ctx context.Context) error {
	tool := async.NewFileProcessorTool()

	// Create tool context
	toolCtx := &core.ToolContext{
		FunctionCallID: stringPtr("file_proc_001"),
	}

	// Prepare arguments
	args := map[string]any{
		"file_path": "/path/to/large_file.txt",
		"operation": "analyze",
		"options": map[string]any{
			"detailed": true,
		},
	}

	// Execute tool with streaming
	stream, err := tool.RunStream(ctx, args, toolCtx)
	if err != nil {
		return fmt.Errorf("failed to start file processor: %v", err)
	}

	fmt.Println("  Starting file processing...")

	// Monitor progress and result
	go func() {
		for progress := range stream.Progress {
			fmt.Printf("    Progress: %.1f%% - %s\n", progress.Progress*100, progress.Message)
			if progress.Metadata != nil {
				if stage, ok := progress.Metadata["stage"]; ok {
					if total, ok := progress.Metadata["total"]; ok {
						fmt.Printf("      Stage %v of %v\n", stage, total)
					}
				}
			}
		}
	}()

	// Wait for result
	select {
	case result := <-stream.Result:
		if result.Error != nil {
			return fmt.Errorf("file processing failed: %v", result.Error)
		}
		fmt.Printf("  ✅ File processing completed!\n")
		if resultMap, ok := result.Result.(map[string]any); ok {
			fmt.Printf("     Duration: %.0f ms\n", resultMap["duration_ms"])
			fmt.Printf("     Size: %.1f MB\n", resultMap["size_mb"])
		}
	case <-time.After(10 * time.Second):
		stream.Cancel()
		return fmt.Errorf("file processing timed out")
	}

	return nil
}

func demonstrateConcurrentScraping(ctx context.Context) error {
	tool := async.NewWebScraperTool()

	// Define multiple scraping tasks
	tasks := []map[string]any{
		{
			"url":       "https://example.com/news",
			"max_pages": 3,
			"selectors": []string{".title", ".summary"},
		},
		{
			"url":       "https://example.com/products",
			"max_pages": 2,
			"selectors": []string{".product-name", ".price"},
		},
		{
			"url":       "https://example.com/blog",
			"max_pages": 1,
		},
	}

	// Start all tasks concurrently
	type taskResult struct {
		taskID int
		result any
		err    error
	}

	resultChan := make(chan taskResult, len(tasks))

	for i, task := range tasks {
		go func(taskID int, args map[string]any) {
			toolCtx := &core.ToolContext{
				FunctionCallID: stringPtr(fmt.Sprintf("scraper_%d", taskID)),
			}

			stream, err := tool.RunStream(ctx, args, toolCtx)
			if err != nil {
				resultChan <- taskResult{taskID, nil, err}
				return
			}

			fmt.Printf("  Task %d: Starting scraping %s\n", taskID+1, args["url"])

			// Monitor progress
			go func() {
				for progress := range stream.Progress {
					fmt.Printf("    Task %d: %.1f%% - %s\n", taskID+1, progress.Progress*100, progress.Message)
				}
			}()

			// Wait for result
			select {
			case result := <-stream.Result:
				resultChan <- taskResult{taskID, result.Result, result.Error}
			case <-time.After(8 * time.Second):
				stream.Cancel()
				resultChan <- taskResult{taskID, nil, fmt.Errorf("task %d timed out", taskID+1)}
			}
		}(i, task)
	}

	// Collect results
	for i := 0; i < len(tasks); i++ {
		result := <-resultChan
		if result.err != nil {
			fmt.Printf("  ❌ Task %d failed: %v\n", result.taskID+1, result.err)
		} else {
			fmt.Printf("  ✅ Task %d completed successfully\n", result.taskID+1)
			if resultMap, ok := result.result.(map[string]any); ok {
				fmt.Printf("     Pages scraped: %v\n", resultMap["pages_scraped"])
			}
		}
	}

	return nil
}

func demonstrateCancellation(ctx context.Context) error {
	tool := async.NewFileProcessorTool()

	toolCtx := &core.ToolContext{
		FunctionCallID: stringPtr("cancel_demo"),
	}

	args := map[string]any{
		"file_path": "/path/to/huge_file.txt",
		"operation": "compress",
	}

	// Start the tool
	stream, err := tool.RunStream(ctx, args, toolCtx)
	if err != nil {
		return fmt.Errorf("failed to start tool for cancellation demo: %v", err)
	}

	fmt.Println("  Starting long-running operation...")

	// Monitor progress and cancel after a few updates
	progressCount := 0
	go func() {
		for progress := range stream.Progress {
			progressCount++
			fmt.Printf("    Progress: %.1f%% - %s\n", progress.Progress*100, progress.Message)

			// Cancel after 3 progress updates
			if progressCount >= 3 {
				fmt.Println("    🛑 Cancelling operation...")
				stream.Cancel()
				break
			}
		}
	}()

	// Wait for result or cancellation
	select {
	case result := <-stream.Result:
		if result.Error != nil {
			if result.Error == context.Canceled {
				fmt.Println("  ✅ Operation successfully cancelled")
				return nil
			}
			return fmt.Errorf("operation failed: %v", result.Error)
		}
		fmt.Println("  ⚠️  Operation completed before cancellation")
	case <-time.After(10 * time.Second):
		stream.Cancel()
		fmt.Println("  ⚠️  Demo timed out")
	}

	return nil
}

func demonstrateTimeout() error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tool := async.NewWebScraperTool()
	toolCtx := &core.ToolContext{
		FunctionCallID: stringPtr("timeout_demo"),
	}

	// This should timeout because of the context deadline
	args := map[string]any{
		"url":       "https://slow-website.example.com",
		"max_pages": 5, // Many pages to make it take longer
	}

	fmt.Println("  Starting operation with 3-second timeout...")

	stream, err := tool.RunStream(ctx, args, toolCtx)
	if err != nil {
		return fmt.Errorf("failed to start tool for timeout demo: %v", err)
	}

	// Monitor progress
	go func() {
		for progress := range stream.Progress {
			fmt.Printf("    Progress: %.1f%% - %s\n", progress.Progress*100, progress.Message)
		}
	}()

	// Wait for result or timeout
	select {
	case result := <-stream.Result:
		if result.Error != nil {
			if result.Error == context.DeadlineExceeded {
				fmt.Println("  ✅ Operation timed out as expected")
				return nil
			}
			return fmt.Errorf("operation failed: %v", result.Error)
		}
		fmt.Println("  ⚠️  Operation completed before timeout")
	case <-ctx.Done():
		fmt.Println("  ✅ Context timeout triggered")
	}

	return nil
}
