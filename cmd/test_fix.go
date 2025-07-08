package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/tools"
)

func main() {
	// Create the exact same static time tool as in the DuckDuckGo agent
	staticTimeTool, err := tools.NewFunctionTool(
		"get_static_time",
		"Gets the current server time without requiring any parameters",
		func() map[string]interface{} {
			now := time.Now()
			return map[string]interface{}{
				"time":     now.Format("3:04 PM"),
				"date":     now.Format("Monday, January 2, 2006"),
				"timezone": now.Format("MST"),
				"iso":      now.Format(time.RFC3339),
			}
		},
	)
	if err != nil {
		log.Fatalf("Failed to create static time tool: %v", err)
	}

	// Test the tool declaration first
	decl := staticTimeTool.GetDeclaration()
	if decl != nil {
		declJSON, _ := json.MarshalIndent(decl, "", "  ")
		fmt.Printf("Function Declaration:\n%s\n\n", string(declJSON))
	}

	// Simulate the exact scenario from the original error:
	// LLM calls get_static_time with empty args: map[]
	ctx := context.Background()
	emptyArgs := make(map[string]any) // This is what was causing the error
	toolCtx := &core.ToolContext{
		State: core.NewState(),
		InvocationContext: &core.InvocationContext{
			InvocationID: "test-invocation",
		},
	}

	fmt.Printf("Simulating LLM calling get_static_time with args: %+v\n", emptyArgs)

	// This should now work without the "invalid function call arguments" error
	result, err := staticTimeTool.RunAsync(ctx, emptyArgs, toolCtx)
	if err != nil {
		log.Fatalf("FAILED: Tool execution failed with error: %v", err)
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("SUCCESS: Tool executed successfully!\nResult:\n%s\n", string(resultJSON))

	fmt.Println("\n✅ The fix works! Functions with no parameters can now be called with empty arguments.")
}
