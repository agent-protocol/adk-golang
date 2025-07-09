// Package tools provides examples of using the enhanced FunctionTool.
package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// Example functions that can be wrapped as tools

// AddNumbers is a simple function that adds two numbers.
func AddNumbers(a, b int) int {
	return a + b
}

// CalculateWithContext demonstrates using context in a tool function.
func CalculateWithContext(ctx context.Context, operation string, a, b float64) (float64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	switch operation {
	case "add":
		return a + b, nil
	case "subtract":
		return a - b, nil
	case "multiply":
		return a * b, nil
	case "divide":
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	default:
		return 0, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// FormatTextWithToolContext demonstrates using ToolContext in a function.
func FormatTextWithToolContext(toolCtx *core.ToolContext, text string, format string) (string, error) {
	// Access session state
	prefix, _ := toolCtx.GetState("text_prefix")
	if prefix == nil {
		prefix = ""
	}

	// Set some state for future use
	toolCtx.SetState("last_formatted_text", text)

	switch format {
	case "upper":
		return fmt.Sprintf("%s%s", prefix, strings.ToUpper(text)), nil
	case "lower":
		return fmt.Sprintf("%s%s", prefix, strings.ToLower(text)), nil
	case "title":
		return fmt.Sprintf("%s%s", prefix, strings.Title(text)), nil
	default:
		return fmt.Sprintf("%s%s", prefix, text), nil
	}
}

// ProcessItems demonstrates working with arrays/slices.
func ProcessItems(items []string, operation string) ([]string, error) {
	result := make([]string, len(items))

	for i, item := range items {
		switch operation {
		case "upper":
			result[i] = strings.ToUpper(item)
		case "lower":
			result[i] = strings.ToLower(item)
		case "reverse":
			runes := []rune(item)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			result[i] = string(runes)
		default:
			return nil, fmt.Errorf("unsupported operation: %s", operation)
		}
	}

	return result, nil
}

// UserInfo demonstrates working with structs.
type UserInfo struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

// CreateUserProfile creates a user profile with the given information.
func CreateUserProfile(name string, age int, email string) UserInfo {
	return UserInfo{
		Name:  name,
		Age:   age,
		Email: email,
	}
}

// AdvancedCalculation demonstrates a complex function with multiple return values.
func AdvancedCalculation(numbers []float64, operation string) (float64, string, error) {
	if len(numbers) == 0 {
		return 0, "", fmt.Errorf("no numbers provided")
	}

	var result float64
	var description string

	switch operation {
	case "sum":
		for _, n := range numbers {
			result += n
		}
		description = fmt.Sprintf("Sum of %d numbers", len(numbers))

	case "average":
		sum := 0.0
		for _, n := range numbers {
			sum += n
		}
		result = sum / float64(len(numbers))
		description = fmt.Sprintf("Average of %d numbers", len(numbers))

	case "max":
		result = numbers[0]
		for _, n := range numbers {
			if n > result {
				result = n
			}
		}
		description = fmt.Sprintf("Maximum of %d numbers", len(numbers))

	case "min":
		result = numbers[0]
		for _, n := range numbers {
			if n < result {
				result = n
			}
		}
		description = fmt.Sprintf("Minimum of %d numbers", len(numbers))

	default:
		return 0, "", fmt.Errorf("unsupported operation: %s", operation)
	}

	return result, description, nil
}

// FileOperationWithArtifacts demonstrates using ToolContext for artifacts.
func FileOperationWithArtifacts(ctx context.Context, toolCtx *core.ToolContext, filename string, content string, operation string) (string, error) {
	switch operation {
	case "save":
		// Save content as an artifact
		version, err := toolCtx.SaveArtifact(filename, []byte(content), "text/plain")
		if err != nil {
			return "", fmt.Errorf("failed to save artifact: %w", err)
		}
		return fmt.Sprintf("Saved file %s (version %d)", filename, version), nil

	case "load":
		// Load content from an artifact
		data, err := toolCtx.LoadArtifact(filename, nil)
		if err != nil {
			return "", fmt.Errorf("failed to load artifact: %w", err)
		}
		return string(data), nil

	case "list":
		// List all artifacts
		files, err := toolCtx.ListArtifacts()
		if err != nil {
			return "", fmt.Errorf("failed to list artifacts: %w", err)
		}
		return fmt.Sprintf("Available files: %s", strings.Join(files, ", ")), nil

	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}
}

// TimerFunction demonstrates a long-running operation with proper cancellation.
func TimerFunction(ctx context.Context, duration string, message string) (string, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return "", fmt.Errorf("invalid duration: %w", err)
	}

	select {
	case <-time.After(d):
		return fmt.Sprintf("Timer finished after %s: %s", duration, message), nil
	case <-ctx.Done():
		return "", fmt.Errorf("timer cancelled: %w", ctx.Err())
	}
}

// LongRunningTask demonstrates a more complex long-running operation with progress reporting.
func LongRunningTask(ctx context.Context, toolCtx *core.ToolContext, steps int, stepDuration string) (map[string]interface{}, error) {
	d, err := time.ParseDuration(stepDuration)
	if err != nil {
		return nil, fmt.Errorf("invalid step duration: %w", err)
	}

	result := map[string]interface{}{
		"total_steps":     steps,
		"completed_steps": 0,
		"status":          "running",
		"start_time":      time.Now(),
	}

	// Save initial state
	toolCtx.SetState("task_progress", 0)
	toolCtx.SetState("task_status", "running")

	for i := 1; i <= steps; i++ {
		// Check for cancellation at each step
		select {
		case <-ctx.Done():
			result["status"] = "cancelled"
			result["completed_steps"] = i - 1
			result["end_time"] = time.Now()
			result["error"] = ctx.Err().Error()
			toolCtx.SetState("task_status", "cancelled")
			return result, fmt.Errorf("task cancelled at step %d: %w", i-1, ctx.Err())
		case <-time.After(d):
			// Step completed
			progress := float64(i) / float64(steps)
			result["completed_steps"] = i
			result["progress"] = progress

			// Update state
			toolCtx.SetState("task_progress", progress)

			if i == steps {
				result["status"] = "completed"
				result["end_time"] = time.Now()
				toolCtx.SetState("task_status", "completed")
			}
		}
	}

	return result, nil
}

// FileProcessingTask demonstrates file processing with cancellation support.
func FileProcessingTask(ctx context.Context, toolCtx *core.ToolContext, filenames []string, operation string) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"total_files":     len(filenames),
		"processed_files": 0,
		"failed_files":    []string{},
		"results":         []map[string]interface{}{},
		"status":          "processing",
	}

	for i, filename := range filenames {
		// Check for cancellation before processing each file
		select {
		case <-ctx.Done():
			result["status"] = "cancelled"
			result["error"] = ctx.Err().Error()
			return result, fmt.Errorf("file processing cancelled: %w", ctx.Err())
		default:
		}

		// Simulate file processing
		fileResult := map[string]interface{}{
			"filename": filename,
			"index":    i,
		}

		switch operation {
		case "analyze":
			// Simulate analysis with potential cancellation
			select {
			case <-ctx.Done():
				result["status"] = "cancelled"
				result["error"] = ctx.Err().Error()
				return result, fmt.Errorf("file processing cancelled during analysis: %w", ctx.Err())
			case <-time.After(100 * time.Millisecond): // Simulate work
				fileResult["analysis"] = map[string]interface{}{
					"size":     1024 + i*512,
					"type":     "text",
					"encoding": "utf-8",
					"lines":    50 + i*10,
				}
			}

		case "backup":
			// Simulate backup with cancellation check
			select {
			case <-ctx.Done():
				result["status"] = "cancelled"
				result["error"] = ctx.Err().Error()
				return result, fmt.Errorf("file processing cancelled during backup: %w", ctx.Err())
			case <-time.After(200 * time.Millisecond): // Simulate work
				backupName := fmt.Sprintf("%s.backup", filename)
				fileResult["backup_created"] = backupName

				// Save as artifact
				content := fmt.Sprintf("Backup content for %s", filename)
				version, err := toolCtx.SaveArtifact(backupName, []byte(content), "text/plain")
				if err != nil {
					result["failed_files"] = append(result["failed_files"].([]string), filename)
					fileResult["error"] = err.Error()
				} else {
					fileResult["artifact_version"] = version
				}
			}

		default:
			result["failed_files"] = append(result["failed_files"].([]string), filename)
			fileResult["error"] = fmt.Sprintf("unsupported operation: %s", operation)
		}

		result["results"] = append(result["results"].([]map[string]interface{}), fileResult)
		result["processed_files"] = i + 1

		// Update progress in state
		progress := float64(i+1) / float64(len(filenames))
		toolCtx.SetState("file_processing_progress", progress)
	}

	result["status"] = "completed"
	return result, nil
}

// NetworkRequestWithRetry demonstrates network operations with cancellation and retry logic.
func NetworkRequestWithRetry(ctx context.Context, url string, maxRetries int, retryDelay string) (map[string]interface{}, error) {
	delay, err := time.ParseDuration(retryDelay)
	if err != nil {
		return nil, fmt.Errorf("invalid retry delay: %w", err)
	}

	result := map[string]interface{}{
		"url":         url,
		"max_retries": maxRetries,
		"attempts":    0,
		"success":     false,
		"start_time":  time.Now(),
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check for cancellation before each attempt
		select {
		case <-ctx.Done():
			result["status"] = "cancelled"
			result["error"] = ctx.Err().Error()
			result["end_time"] = time.Now()
			return result, fmt.Errorf("network request cancelled: %w", ctx.Err())
		default:
		}

		result["attempts"] = attempt

		// Simulate network request
		select {
		case <-ctx.Done():
			result["status"] = "cancelled"
			result["error"] = ctx.Err().Error()
			result["end_time"] = time.Now()
			return result, fmt.Errorf("network request cancelled during attempt %d: %w", attempt, ctx.Err())
		case <-time.After(500 * time.Millisecond): // Simulate network delay
			// Simulate success/failure (succeed on last attempt for demo)
			if attempt == maxRetries || attempt >= 2 {
				result["success"] = true
				result["status"] = "completed"
				result["response"] = map[string]interface{}{
					"status_code": 200,
					"headers":     map[string]string{"Content-Type": "application/json"},
					"body":        fmt.Sprintf("Response from %s", url),
				}
				result["end_time"] = time.Now()
				return result, nil
			}
		}

		// If not the last attempt, wait before retrying
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				result["status"] = "cancelled"
				result["error"] = ctx.Err().Error()
				result["end_time"] = time.Now()
				return result, fmt.Errorf("network request cancelled during retry delay: %w", ctx.Err())
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	result["status"] = "failed"
	result["error"] = "max retries exceeded"
	result["end_time"] = time.Now()
	return result, fmt.Errorf("network request failed after %d attempts", maxRetries)
}

// ConcurrentProcessor demonstrates concurrent processing with proper cancellation.
func ConcurrentProcessor(ctx context.Context, toolCtx *core.ToolContext, items []string, workers int) (map[string]interface{}, error) {
	if workers <= 0 {
		workers = 1
	}
	if workers > len(items) {
		workers = len(items)
	}

	result := map[string]interface{}{
		"total_items":     len(items),
		"workers":         workers,
		"processed_items": 0,
		"failed_items":    []string{},
		"results":         []map[string]interface{}{},
		"status":          "processing",
		"start_time":      time.Now(),
	}

	// Create channels for work distribution
	workChan := make(chan string, len(items))
	resultChan := make(chan map[string]interface{}, len(items))

	// Create cancellation context for workers
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case item, ok := <-workChan:
					if !ok {
						return
					}

					// Process item with cancellation check
					itemResult := map[string]interface{}{
						"item":      item,
						"worker_id": workerID,
					}

					// Simulate processing
					select {
					case <-workerCtx.Done():
						return
					case <-time.After(100 * time.Millisecond):
						itemResult["processed"] = true
						itemResult["length"] = len(item)
						itemResult["uppercase"] = strings.ToUpper(item)
					}

					select {
					case resultChan <- itemResult:
					case <-workerCtx.Done():
						return
					}
				}
			}
		}(i)
	}

	// Send work to workers
	go func() {
		defer close(workChan)
		for _, item := range items {
			select {
			case <-ctx.Done():
				return
			case workChan <- item:
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results with cancellation support
	processedCount := 0
	for {
		select {
		case <-ctx.Done():
			result["status"] = "cancelled"
			result["error"] = ctx.Err().Error()
			result["end_time"] = time.Now()
			result["processed_items"] = processedCount
			return result, fmt.Errorf("concurrent processing cancelled: %w", ctx.Err())
		case itemResult, ok := <-resultChan:
			if !ok {
				// All results processed
				result["status"] = "completed"
				result["end_time"] = time.Now()
				result["processed_items"] = processedCount
				return result, nil
			}

			result["results"] = append(result["results"].([]map[string]interface{}), itemResult)
			processedCount++

			// Update progress
			progress := float64(processedCount) / float64(len(items))
			toolCtx.SetState("concurrent_processing_progress", progress)
		}
	}
}

// ExampleUsage demonstrates how to create and use enhanced function tools.
func ExampleUsage() {
	// Create tools from functions
	addTool, err := NewEnhancedFunctionTool("add_numbers", "Adds two integers", AddNumbers)
	if err != nil {
		fmt.Printf("Error creating add tool: %v\n", err)
		return
	}

	calcTool, err := NewEnhancedFunctionTool("calculate", "Performs mathematical operations", CalculateWithContext)
	if err != nil {
		fmt.Printf("Error creating calc tool: %v\n", err)
		return
	}

	formatTool, err := NewEnhancedFunctionTool("format_text", "Formats text with different styles", FormatTextWithToolContext)
	if err != nil {
		fmt.Printf("Error creating format tool: %v\n", err)
		return
	}

	// Display tool information
	fmt.Printf("Created tools:\n")
	fmt.Printf("1. %s: %s\n", addTool.Name(), addTool.Description())
	fmt.Printf("2. %s: %s\n", calcTool.Name(), calcTool.Description())
	fmt.Printf("3. %s: %s\n", formatTool.Name(), formatTool.Description())

	// Show function declarations
	fmt.Printf("\nFunction declarations:\n")

	addDecl := addTool.GetDeclaration()
	fmt.Printf("Add Tool Declaration: %+v\n", addDecl)

	calcDecl := calcTool.GetDeclaration()
	fmt.Printf("Calc Tool Declaration: %+v\n", calcDecl)

	// Show metadata
	fmt.Printf("\nMetadata:\n")
	addMetadata := addTool.GetMetadata()
	fmt.Printf("Add Tool Metadata: %+v\n", addMetadata)

	// Example tool execution would happen in an agent context
	fmt.Printf("\nTools created successfully and ready for use!\n")
}

// ValidationExamples demonstrates function validation.
func ValidationExamples() {
	// Valid functions
	validFunctions := []interface{}{
		AddNumbers,
		CalculateWithContext,
		FormatTextWithToolContext,
		ProcessItems,
		CreateUserProfile,
	}

	fmt.Printf("Validating functions:\n")
	for i, fn := range validFunctions {
		if err := ValidateFunction(fn); err != nil {
			fmt.Printf("%d. INVALID: %v\n", i+1, err)
		} else {
			fmt.Printf("%d. VALID\n", i+1)
		}
	}

	// Invalid examples
	invalidFunctions := []interface{}{
		nil,
		"not a function",
		123,
	}

	fmt.Printf("\nValidating invalid examples:\n")
	for i, fn := range invalidFunctions {
		if err := ValidateFunction(fn); err != nil {
			fmt.Printf("%d. INVALID (expected): %v\n", i+1, err)
		} else {
			fmt.Printf("%d. VALID (unexpected!)\n", i+1)
		}
	}
}

// BenchmarkFunction is a simple function for performance testing.
func BenchmarkFunction(iterations int, delay string) (string, error) {
	d, err := time.ParseDuration(delay)
	if err != nil {
		return "", fmt.Errorf("invalid delay: %w", err)
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		time.Sleep(d)
	}
	elapsed := time.Since(start)

	return fmt.Sprintf("Completed %d iterations in %s (avg: %s per iteration)",
		iterations, elapsed, elapsed/time.Duration(iterations)), nil
}

// ComplexDataProcessor demonstrates handling complex data structures.
type ProcessingConfig struct {
	Mode      string            `json:"mode"`
	Options   map[string]string `json:"options"`
	Enabled   bool              `json:"enabled"`
	Threshold float64           `json:"threshold"`
}

type ProcessingResult struct {
	Status    string    `json:"status"`
	Processed int       `json:"processed"`
	Errors    []string  `json:"errors"`
	Timestamp time.Time `json:"timestamp"`
}

func ComplexDataProcessor(data []string, config ProcessingConfig) (ProcessingResult, error) {
	result := ProcessingResult{
		Status:    "processing",
		Processed: 0,
		Errors:    make([]string, 0),
		Timestamp: time.Now(),
	}

	if !config.Enabled {
		result.Status = "disabled"
		return result, nil
	}

	for i, item := range data {
		if len(item) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("empty item at index %d", i))
			continue
		}

		// Simulate processing based on mode
		switch config.Mode {
		case "validate":
			// Validate the item
			if len(item) < int(config.Threshold) {
				result.Errors = append(result.Errors, fmt.Sprintf("item %d too short", i))
			} else {
				result.Processed++
			}
		case "transform":
			// Transform the item (just count it as processed)
			result.Processed++
		default:
			result.Errors = append(result.Errors, fmt.Sprintf("unknown mode: %s", config.Mode))
		}
	}

	if len(result.Errors) == 0 {
		result.Status = "completed"
	} else {
		result.Status = "completed_with_errors"
	}

	return result, nil
}

// StringUtilities provides various string manipulation functions.
type StringUtilities struct{}

// Reverse reverses a string.
func (su StringUtilities) Reverse(input string) string {
	runes := []rune(input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Count counts occurrences of a substring.
func (su StringUtilities) Count(input, substring string) int {
	return strings.Count(input, substring)
}

// Split splits a string by delimiter.
func (su StringUtilities) Split(input, delimiter string) []string {
	return strings.Split(input, delimiter)
}

// WordCount counts words in a string.
func (su StringUtilities) WordCount(input string) map[string]interface{} {
	words := strings.Fields(input)
	wordCount := make(map[string]int)

	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?;:"))
		wordCount[word]++
	}

	return map[string]interface{}{
		"total_words":  len(words),
		"unique_words": len(wordCount),
		"word_counts":  wordCount,
	}
}

// MathUtilities provides mathematical utility functions.
type MathUtilities struct{}

// IsPrime checks if a number is prime.
func (mu MathUtilities) IsPrime(n int) bool {
	if n < 2 {
		return false
	}
	for i := 2; i*i <= n; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
}

// Factorial calculates factorial of a number.
func (mu MathUtilities) Factorial(n int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("factorial not defined for negative numbers")
	}
	if n > 20 {
		return 0, fmt.Errorf("factorial too large (max 20)")
	}

	result := 1
	for i := 2; i <= n; i++ {
		result *= i
	}
	return result, nil
}

// GCD calculates the greatest common divisor.
func (mu MathUtilities) GCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// LCM calculates the least common multiple.
func (mu MathUtilities) LCM(a, b int) int {
	return (a * b) / mu.GCD(a, b)
}

// ToBase converts a number to a different base.
func (mu MathUtilities) ToBase(number int, base int) (string, error) {
	if base < 2 || base > 36 {
		return "", fmt.Errorf("base must be between 2 and 36")
	}
	return strconv.FormatInt(int64(number), base), nil
}

// FromBase converts a number from a different base to base 10.
func (mu MathUtilities) FromBase(number string, base int) (int, error) {
	if base < 2 || base > 36 {
		return 0, fmt.Errorf("base must be between 2 and 36")
	}
	result, err := strconv.ParseInt(number, base, 64)
	return int(result), err
}
