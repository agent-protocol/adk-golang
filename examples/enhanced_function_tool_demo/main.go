// Package main demonstrates the enhanced FunctionTool implementation.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

func main() {
	fmt.Println("=== Enhanced FunctionTool Demo ===")
	fmt.Println()

	// 1. Demonstrate basic function wrapping
	demonstrateBasicFunctionTool()
	fmt.Println()

	// 2. Demonstrate parameter validation
	demonstrateParameterValidation()
	fmt.Println()

	// 3. Demonstrate schema generation
	demonstrateSchemaGeneration()
	fmt.Println()

	// 4. Demonstrate ToolContext usage
	demonstrateToolContextUsage()
	fmt.Println()

	// 5. Demonstrate complex data types
	demonstrateComplexDataTypes()
	fmt.Println()

	// 6. Run the examples from the tools package
	fmt.Println("=== Running Package Examples ===")
	tools.ExampleUsage()
	fmt.Println()
	tools.ValidationExamples()
}

// demonstrateBasicFunctionTool shows basic function wrapping.
func demonstrateBasicFunctionTool() {
	fmt.Println("--- Basic Function Tool ---")

	// Create a tool from the AddNumbers function
	addTool, err := tools.NewEnhancedFunctionTool(
		"add_numbers",
		"Adds two integers and returns the sum",
		tools.AddNumbers,
	)
	if err != nil {
		log.Fatalf("Failed to create add tool: %v", err)
	}

	fmt.Printf("Tool Name: %s\n", addTool.Name())
	fmt.Printf("Description: %s\n", addTool.Description())

	// Get the function declaration
	decl := addTool.GetDeclaration()
	fmt.Printf("Function Declaration:\n")
	fmt.Printf("  Name: %s\n", decl.Name)
	fmt.Printf("  Description: %s\n", decl.Description)
	fmt.Printf("  Parameters: %+v\n", decl.Parameters)

	// Get metadata
	metadata := addTool.GetMetadata()
	fmt.Printf("Metadata:\n")
	fmt.Printf("  Parameters Count: %d\n", len(metadata.Parameters))
	for i, param := range metadata.Parameters {
		fmt.Printf("  Param %d: %s (%s)\n", i+1, param.Name, param.JSONType)
	}

	// Execute the tool
	ctx := context.Background()
	session := core.NewSession("demo-session", "demo-app", "demo-user")
	invocationCtx := core.NewInvocationContext("demo-invocation", nil, session, nil)
	toolCtx := core.NewToolContext(invocationCtx)

	args := map[string]interface{}{
		"int":  10,
		"int1": 25,
	}

	result, err := addTool.RunAsync(ctx, args, toolCtx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %v\n", result)
	}
}

// demonstrateParameterValidation shows parameter validation in action.
func demonstrateParameterValidation() {
	fmt.Println("--- Parameter Validation ---")

	calcTool, err := tools.NewEnhancedFunctionTool(
		"calculator",
		"Performs mathematical operations with validation",
		tools.CalculateWithContext,
	)
	if err != nil {
		log.Fatalf("Failed to create calculator tool: %v", err)
	}

	ctx := context.Background()
	session := core.NewSession("demo-session", "demo-app", "demo-user")
	invocationCtx := core.NewInvocationContext("demo-invocation", nil, session, nil)
	toolCtx := core.NewToolContext(invocationCtx)

	// Test cases
	testCases := []struct {
		name string
		args map[string]interface{}
	}{
		{
			"Valid division",
			map[string]interface{}{
				"string":   "divide",
				"float64":  20.0,
				"float641": 4.0,
			},
		},
		{
			"Division by zero",
			map[string]interface{}{
				"string":   "divide",
				"float64":  10.0,
				"float641": 0.0,
			},
		},
		{
			"Missing parameter",
			map[string]interface{}{
				"string": "add",
				// Missing float64 parameters
			},
		},
		{
			"Unknown operation",
			map[string]interface{}{
				"string":   "unknown_op",
				"float64":  1.0,
				"float641": 2.0,
			},
		},
	}

	for _, tc := range testCases {
		fmt.Printf("\nTest: %s\n", tc.name)
		fmt.Printf("Args: %+v\n", tc.args)

		result, err := calcTool.RunAsync(ctx, tc.args, toolCtx)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Result: %v\n", result)
		}
	}
}

// demonstrateSchemaGeneration shows JSON schema generation for different function types.
func demonstrateSchemaGeneration() {
	fmt.Println("--- Schema Generation ---")

	// Test with different function signatures
	functions := []struct {
		name string
		fn   interface{}
	}{
		{"simple_add", tools.AddNumbers},
		{"process_items", tools.ProcessItems},
		{"create_user", tools.CreateUserProfile},
		{"advanced_calc", tools.AdvancedCalculation},
	}

	for _, f := range functions {
		fmt.Printf("\nFunction: %s\n", f.name)

		tool, err := tools.NewEnhancedFunctionTool(f.name, "Demo function", f.fn)
		if err != nil {
			fmt.Printf("Error creating tool: %v\n", err)
			continue
		}

		decl := tool.GetDeclaration()
		fmt.Printf("Schema:\n")
		fmt.Printf("  Type: %s\n", decl.Parameters["type"])

		if props, ok := decl.Parameters["properties"].(map[string]interface{}); ok {
			fmt.Printf("  Properties:\n")
			for name, prop := range props {
				if propMap, ok := prop.(map[string]interface{}); ok {
					fmt.Printf("    %s: %s\n", name, propMap["type"])
				}
			}
		}

		if required, ok := decl.Parameters["required"].([]string); ok {
			fmt.Printf("  Required: %v\n", required)
		}
	}
}

// demonstrateToolContextUsage shows how ToolContext can be used within functions.
func demonstrateToolContextUsage() {
	fmt.Println("--- ToolContext Usage ---")

	formatTool, err := tools.NewEnhancedFunctionTool(
		"format_text",
		"Formats text using session state and ToolContext",
		tools.FormatTextWithToolContext,
	)
	if err != nil {
		log.Fatalf("Failed to create format tool: %v", err)
	}

	ctx := context.Background()
	session := core.NewSession("demo-session", "demo-app", "demo-user")
	invocationCtx := core.NewInvocationContext("demo-invocation", nil, session, nil)
	toolCtx := core.NewToolContext(invocationCtx)

	// Set some initial state
	toolCtx.SetState("text_prefix", "[FORMATTED] ")

	fmt.Printf("Initial state set: text_prefix = '[FORMATTED] '\n")

	// Test different formatting operations
	operations := []struct {
		text   string
		format string
	}{
		{"hello world", "upper"},
		{"GOODBYE WORLD", "lower"},
		{"this is a test", "title"},
	}

	for _, op := range operations {
		fmt.Printf("\nFormatting '%s' with format '%s'\n", op.text, op.format)

		args := map[string]interface{}{
			"string":  op.text,
			"string1": op.format,
		}

		result, err := formatTool.RunAsync(ctx, args, toolCtx)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Result: %v\n", result)
		}

		// Check what was stored in state
		if lastText, exists := toolCtx.GetState("last_formatted_text"); exists {
			fmt.Printf("State updated: last_formatted_text = '%v'\n", lastText)
		}
	}
}

// demonstrateComplexDataTypes shows handling of arrays, structs, and multiple return values.
func demonstrateComplexDataTypes() {
	fmt.Println("--- Complex Data Types ---")

	// Array processing
	fmt.Println("\nArray Processing:")
	processTool, err := tools.NewEnhancedFunctionTool(
		"process_items",
		"Processes an array of strings",
		tools.ProcessItems,
	)
	if err != nil {
		log.Fatalf("Failed to create process tool: %v", err)
	}

	ctx := context.Background()
	session := core.NewSession("demo-session", "demo-app", "demo-user")
	invocationCtx := core.NewInvocationContext("demo-invocation", nil, session, nil)
	toolCtx := core.NewToolContext(invocationCtx)

	args := map[string]interface{}{
		"slice":  []string{"hello", "world", "test"},
		"string": "upper",
	}

	result, err := processTool.RunAsync(ctx, args, toolCtx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Array processing result: %v\n", result)
	}

	// Struct creation
	fmt.Println("\nStruct Creation:")
	userTool, err := tools.NewEnhancedFunctionTool(
		"create_user_profile",
		"Creates a user profile struct",
		tools.CreateUserProfile,
	)
	if err != nil {
		log.Fatalf("Failed to create user tool: %v", err)
	}

	userArgs := map[string]interface{}{
		"string":  "John Doe",
		"int":     30,
		"string1": "john.doe@example.com",
	}

	userResult, err := userTool.RunAsync(ctx, userArgs, toolCtx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("User creation result: %v\n", userResult)
	}

	// Multiple return values
	fmt.Println("\nMultiple Return Values:")
	advancedTool, err := tools.NewEnhancedFunctionTool(
		"advanced_calculation",
		"Performs calculations with multiple return values",
		tools.AdvancedCalculation,
	)
	if err != nil {
		log.Fatalf("Failed to create advanced tool: %v", err)
	}

	advancedArgs := map[string]interface{}{
		"slice":  []float64{1.5, 2.5, 3.5, 4.5, 5.5},
		"string": "average",
	}

	advancedResult, err := advancedTool.RunAsync(ctx, advancedArgs, toolCtx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Advanced calculation result: %v\n", advancedResult)
	}
}

// Additional helper function to show validation
func showValidation() {
	fmt.Println("--- Function Validation ---")

	validFunctions := []interface{}{
		tools.AddNumbers,
		tools.CalculateWithContext,
		tools.ProcessItems,
	}

	invalidInputs := []interface{}{
		nil,
		"not a function",
		42,
	}

	fmt.Println("Valid functions:")
	for i, fn := range validFunctions {
		if err := tools.ValidateFunction(fn); err != nil {
			fmt.Printf("%d. INVALID: %v\n", i+1, err)
		} else {
			fmt.Printf("%d. VALID\n", i+1)
		}
	}

	fmt.Println("\nInvalid inputs:")
	for i, input := range invalidInputs {
		if err := tools.ValidateFunction(input); err != nil {
			fmt.Printf("%d. INVALID (expected): %v\n", i+1, err)
		} else {
			fmt.Printf("%d. VALID (unexpected!)\n", i+1)
		}
	}
}
