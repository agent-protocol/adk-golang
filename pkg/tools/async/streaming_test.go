package async

import (
	"context"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

func TestStreamingToolBasicExecution(t *testing.T) {
	tool := NewStreamingTool("test_tool", "Test streaming tool", 1)

	ctx := context.Background()
	toolCtx := &core.ToolContext{
		FunctionCallID: ptr.Ptr("test_001"),
	}
	args := map[string]any{"test": "value"}

	// Test RunAsync (blocking version)
	result, err := tool.RunAsync(ctx, args, toolCtx)
	if err != nil {
		t.Fatalf("RunAsync failed: %v", err)
	}

	if result != "Task completed" {
		t.Errorf("Expected 'Task completed', got %v", result)
	}
}

func TestStreamingToolProgressUpdates(t *testing.T) {
	tool := NewStreamingTool("test_tool", "Test streaming tool", 1)

	ctx := context.Background()
	toolCtx := &core.ToolContext{
		FunctionCallID: ptr.Ptr("test_002"),
	}
	args := map[string]any{"test": "value"}

	// Test RunStream (streaming version)
	stream, err := tool.RunStream(ctx, args, toolCtx)
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}

	// Count progress updates
	progressCount := 0
	done := make(chan bool)

	// Monitor progress
	go func() {
		for progress := range stream.Progress {
			progressCount++
			if progress.Progress < 0 || progress.Progress > 1 {
				t.Errorf("Invalid progress value: %f", progress.Progress)
			}
			if progress.Message == "" {
				t.Error("Progress message should not be empty")
			}
		}
		done <- true
	}()

	// Wait for result
	select {
	case result := <-stream.Result:
		if result.Error != nil {
			t.Fatalf("Tool execution failed: %v", result.Error)
		}
		if result.Result != "Task completed" {
			t.Errorf("Expected 'Task completed', got %v", result.Result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Tool execution timed out")
	}

	// Wait for progress monitoring to complete
	<-done

	// Should have received progress updates (at least initial + completion)
	if progressCount < 2 {
		t.Errorf("Expected at least 2 progress updates, got %d", progressCount)
	}
}

func TestStreamingToolCancellation(t *testing.T) {
	tool := NewStreamingTool("test_tool", "Test streaming tool", 1)

	ctx, cancel := context.WithCancel(context.Background())
	toolCtx := &core.ToolContext{
		FunctionCallID: ptr.Ptr("test_003"),
	}
	args := map[string]any{"test": "value"}

	stream, err := tool.RunStream(ctx, args, toolCtx)
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Wait for result
	select {
	case result := <-stream.Result:
		if result.Error != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", result.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Tool execution should have been cancelled")
	}
}

func TestFileProcessorTool(t *testing.T) {
	tool := NewFileProcessorTool()

	ctx := context.Background()
	toolCtx := &core.ToolContext{
		FunctionCallID: ptr.Ptr("file_test"),
	}
	args := map[string]any{
		"file_path": "/test/file.txt",
		"operation": "analyze",
	}

	// Test function declaration
	decl := tool.GetDeclaration()
	if decl == nil {
		t.Fatal("GetDeclaration should not return nil")
	}
	if decl.Name != "file_processor" {
		t.Errorf("Expected name 'file_processor', got %s", decl.Name)
	}

	// Test execution
	result, err := tool.RunAsync(ctx, args, toolCtx)
	if err != nil {
		t.Fatalf("File processor execution failed: %v", err)
	}

	t.Logf("Result type: %T, value: %v", result, result)

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Result should be a map, got %T", result)
	}

	if resultMap["file_path"] != "/test/file.txt" {
		t.Errorf("Expected file_path '/test/file.txt', got %v", resultMap["file_path"])
	}

	if resultMap["operation"] != "analyze" {
		t.Errorf("Expected operation 'analyze', got %v", resultMap["operation"])
	}
}

func TestWebScraperTool(t *testing.T) {
	tool := NewWebScraperTool()

	ctx := context.Background()
	toolCtx := &core.ToolContext{
		FunctionCallID: ptr.Ptr("scraper_test"),
	}
	args := map[string]any{
		"url":       "https://example.com",
		"max_pages": 2,
		"selectors": []interface{}{".title", ".content"},
	}

	// Test function declaration
	decl := tool.GetDeclaration()
	if decl == nil {
		t.Fatal("GetDeclaration should not return nil")
	}
	if decl.Name != "web_scraper" {
		t.Errorf("Expected name 'web_scraper', got %s", decl.Name)
	}

	// Test execution
	result, err := tool.RunAsync(ctx, args, toolCtx)
	if err != nil {
		t.Fatalf("Web scraper execution failed: %v", err)
	}

	t.Logf("Result type: %T, value: %v", result, result)

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Result should be a map, got %T", result)
	}

	if resultMap["url"] != "https://example.com" {
		t.Errorf("Expected url 'https://example.com', got %v", resultMap["url"])
	}

	if resultMap["pages_scraped"] != 2 {
		t.Errorf("Expected 2 pages scraped, got %v", resultMap["pages_scraped"])
	}

	content, ok := resultMap["content"].([]map[string]any)
	if !ok {
		t.Fatal("Content should be a slice of maps")
	}

	if len(content) != 2 {
		t.Errorf("Expected 2 content items, got %d", len(content))
	}
}

func TestConcurrencyLimits(t *testing.T) {
	tool := NewStreamingTool("test_tool", "Test streaming tool", 2) // Max 2 concurrent

	ctx := context.Background()

	// Start 3 tasks (should exceed limit)
	streams := make([]*ToolStream, 3)
	errs := make([]error, 3)

	for i := 0; i < 3; i++ {
		toolCtx := &core.ToolContext{
			FunctionCallID: ptr.Ptr("concurrent_test"),
		}
		args := map[string]any{"test": i}

		streams[i], errs[i] = tool.RunStream(ctx, args, toolCtx)
	}

	// Count successful starts
	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}

	// Should only allow 2 concurrent executions
	if successCount > 2 {
		t.Errorf("Expected max 2 concurrent executions, got %d", successCount)
	}

	// Clean up successful streams
	for i, stream := range streams {
		if errs[i] == nil {
			stream.Cancel()
		}
	}
}
