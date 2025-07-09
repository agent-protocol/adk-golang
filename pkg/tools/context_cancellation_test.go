package tools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// TestContextCancellationInFunctions demonstrates proper context cancellation handling
func TestContextCancellationInFunctions(t *testing.T) {
	t.Run("TimerFunction with cancellation", func(t *testing.T) {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Call TimerFunction with a longer duration than the timeout
		result, err := TimerFunction(ctx, "200ms", "should be cancelled")

		// Should return an error due to cancellation
		if err == nil {
			t.Errorf("Expected cancellation error, but got none. Result: %s", result)
		}

		if err != nil && err.Error() != "timer cancelled: context deadline exceeded" {
			t.Errorf("Expected cancellation error, got: %v", err)
		}
	})

	t.Run("LongRunningTask with cancellation", func(t *testing.T) {
		// Create a new session and context for testing
		session := core.NewSession("test-session", "test-app", "test-user")
		invocationCtx := core.NewInvocationContext(context.Background(), "test-invocation", nil, session, nil)
		toolCtx := core.NewToolContext(invocationCtx)

		// Create a context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())

		// Start the task in a goroutine
		type result struct {
			data map[string]interface{}
			err  error
		}
		resultChan := make(chan result, 1)

		go func() {
			data, err := LongRunningTask(ctx, toolCtx, 10, "100ms")
			resultChan <- result{data: data, err: err}
		}()

		// Cancel the context after a short delay
		time.Sleep(150 * time.Millisecond)
		cancel()

		// Wait for the result
		select {
		case res := <-resultChan:
			if res.err == nil {
				t.Errorf("Expected cancellation error, but got none")
			}
			if res.data != nil {
				if status, ok := res.data["status"]; ok && status != "cancelled" {
					t.Errorf("Expected status to be 'cancelled', got: %v", status)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for cancellation")
		}
	})

	t.Run("FileProcessingTask with cancellation", func(t *testing.T) {
		// Create a new session and context for testing
		session := core.NewSession("test-session", "test-app", "test-user")
		invocationCtx := core.NewInvocationContext(context.Background(), "test-invocation", nil, session, nil)
		toolCtx := core.NewToolContext(invocationCtx)

		// Create a context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())

		files := []string{"file1.txt", "file2.txt", "file3.txt", "file4.txt", "file5.txt"}

		// Start the task in a goroutine
		type result struct {
			data map[string]interface{}
			err  error
		}
		resultChan := make(chan result, 1)

		go func() {
			data, err := FileProcessingTask(ctx, toolCtx, files, "analyze")
			resultChan <- result{data: data, err: err}
		}()

		// Cancel the context after a short delay
		time.Sleep(250 * time.Millisecond)
		cancel()

		// Wait for the result
		select {
		case res := <-resultChan:
			if res.err == nil {
				t.Errorf("Expected cancellation error, but got none")
			}
			if res.data != nil {
				if status, ok := res.data["status"]; ok && status != "cancelled" {
					t.Errorf("Expected status to be 'cancelled', got: %v", status)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for cancellation")
		}
	})

	t.Run("ConcurrentProcessor with cancellation", func(t *testing.T) {
		// Create a new session and context for testing
		session := core.NewSession("test-session", "test-app", "test-user")
		invocationCtx := core.NewInvocationContext(context.Background(), "test-invocation", nil, session, nil)
		toolCtx := core.NewToolContext(invocationCtx)

		// Create a context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())

		items := []string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8"}

		// Start the task in a goroutine
		type result struct {
			data map[string]interface{}
			err  error
		}
		resultChan := make(chan result, 1)

		go func() {
			data, err := ConcurrentProcessor(ctx, toolCtx, items, 3)
			resultChan <- result{data: data, err: err}
		}()

		// Cancel the context after a short delay
		time.Sleep(150 * time.Millisecond)
		cancel()

		// Wait for the result
		select {
		case res := <-resultChan:
			if res.err == nil {
				t.Errorf("Expected cancellation error, but got none")
			}
			if res.data != nil {
				if status, ok := res.data["status"]; ok && status != "cancelled" {
					t.Errorf("Expected status to be 'cancelled', got: %v", status)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for cancellation")
		}
	})
}

// TestGoroutineLeakPrevention demonstrates that goroutines properly exit on context cancellation
func TestGoroutineLeakPrevention(t *testing.T) {
	t.Run("Multiple concurrent tasks with cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Start multiple goroutines that should all be cancelled
		numTasks := 10
		resultChans := make([]chan error, numTasks)

		for i := 0; i < numTasks; i++ {
			resultChans[i] = make(chan error, 1)

			go func(taskID int, resultChan chan error) {
				// Simulate long-running work that checks for cancellation
				for j := 0; j < 100; j++ {
					select {
					case <-ctx.Done():
						resultChan <- fmt.Errorf("task %d cancelled: %w", taskID, ctx.Err())
						return
					case <-time.After(10 * time.Millisecond):
						// Continue working
					}
				}
				resultChan <- nil // Task completed without cancellation
			}(i, resultChans[i])
		}

		// Cancel all tasks after a short delay
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for all tasks to complete or be cancelled
		cancelledCount := 0
		completedCount := 0

		for i := 0; i < numTasks; i++ {
			select {
			case err := <-resultChans[i]:
				if err != nil {
					cancelledCount++
				} else {
					completedCount++
				}
			case <-time.After(1 * time.Second):
				t.Errorf("Task %d did not respond to cancellation within timeout", i)
			}
		}

		t.Logf("Tasks cancelled: %d, Tasks completed: %d", cancelledCount, completedCount)

		if cancelledCount == 0 {
			t.Error("Expected at least some tasks to be cancelled")
		}
	})
}

// TestProperContextPropagation demonstrates context propagation through function calls
func TestProperContextPropagation(t *testing.T) {
	t.Run("Context timeout propagates through function stack", func(t *testing.T) {
		// Create a context with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Call a function that should respect the timeout
		start := time.Now()
		_, err := TimerFunction(ctx, "200ms", "test message")
		elapsed := time.Since(start)

		// Should have been cancelled before 200ms elapsed
		if elapsed >= 200*time.Millisecond {
			t.Errorf("Function took too long (%v), should have been cancelled", elapsed)
		}

		if err == nil {
			t.Error("Expected cancellation error due to timeout")
		}
	})

	t.Run("NetworkRequestWithRetry respects context cancellation", func(t *testing.T) {
		// Create a context that will be cancelled during retry
		ctx, cancel := context.WithCancel(context.Background())

		// Start the network request in a goroutine
		type result struct {
			data map[string]interface{}
			err  error
		}
		resultChan := make(chan result, 1)

		go func() {
			data, err := NetworkRequestWithRetry(ctx, "https://example.com", 5, "100ms")
			resultChan <- result{data: data, err: err}
		}()

		// Cancel the context after a short delay
		time.Sleep(150 * time.Millisecond)
		cancel()

		// Wait for the result
		select {
		case res := <-resultChan:
			if res.err == nil {
				t.Errorf("Expected cancellation error, but got none")
			}
			if res.data != nil {
				if status, ok := res.data["status"]; ok && status != "cancelled" {
					t.Errorf("Expected status to be 'cancelled', got: %v", status)
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out waiting for cancellation")
		}
	})
}
