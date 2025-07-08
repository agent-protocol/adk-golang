// Package tools provides examples of using the enhanced FunctionTool.
package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
		version, err := toolCtx.SaveArtifact(ctx, filename, []byte(content), "text/plain")
		if err != nil {
			return "", fmt.Errorf("failed to save artifact: %w", err)
		}
		return fmt.Sprintf("Saved file %s (version %d)", filename, version), nil

	case "load":
		// Load content from an artifact
		data, err := toolCtx.LoadArtifact(ctx, filename, nil)
		if err != nil {
			return "", fmt.Errorf("failed to load artifact: %w", err)
		}
		return string(data), nil

	case "list":
		// List all artifacts
		files, err := toolCtx.ListArtifacts(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list artifacts: %w", err)
		}
		return fmt.Sprintf("Available files: %s", strings.Join(files, ", ")), nil

	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}
}

// TimerFunction demonstrates a long-running operation.
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
