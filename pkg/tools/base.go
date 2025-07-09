// Package tools provides concrete implementations of tool types.
package tools

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// BaseToolImpl provides a basic implementation of the BaseTool interface.
type BaseToolImpl struct {
	name          string
	description   string
	isLongRunning bool
}

// NewBaseTool creates a new base tool implementation.
func NewBaseTool(name, description string) *BaseToolImpl {
	return &BaseToolImpl{
		name:        name,
		description: description,
	}
}

// Name returns the tool's unique identifier.
func (t *BaseToolImpl) Name() string {
	return t.name
}

// Description returns a description of the tool's purpose.
func (t *BaseToolImpl) Description() string {
	return t.description
}

// IsLongRunning indicates if this is a long-running operation.
func (t *BaseToolImpl) IsLongRunning() bool {
	return t.isLongRunning
}

// SetLongRunning sets whether this tool is long-running.
func (t *BaseToolImpl) SetLongRunning(longRunning bool) {
	t.isLongRunning = longRunning
}

// GetDeclaration returns the function declaration for LLM integration.
// Base implementation returns nil - concrete tools should override this.
func (t *BaseToolImpl) GetDeclaration() *core.FunctionDeclaration {
	return nil
}

// RunAsync executes the tool with the given arguments and context.
// Base implementation returns an error - concrete tools must override this.
func (t *BaseToolImpl) RunAsync(toolCtx *core.ToolContext, args map[string]any) (any, error) {
	return nil, fmt.Errorf("tool %s does not implement RunAsync", t.name)
}

// ProcessLLMRequest allows the tool to modify LLM requests.
// Base implementation does nothing - tools can override as needed.
func (t *BaseToolImpl) ProcessLLMRequest(toolCtx *core.ToolContext, request *core.LLMRequest) error {
	// If the tool has a declaration, add it to the request
	if decl := t.GetDeclaration(); decl != nil {
		if request.Config == nil {
			request.Config = &core.LLMConfig{}
		}
		if request.Config.Tools == nil {
			request.Config.Tools = make([]*core.FunctionDeclaration, 0)
		}
		request.Config.Tools = append(request.Config.Tools, decl)
	}
	return nil
}

// FunctionTool wraps a Go function as a tool.
type FunctionTool struct {
	*BaseToolImpl
	function interface{}
	schema   *FunctionSchema
}

// FunctionSchema describes the parameters of a function.
type FunctionSchema struct {
	Parameters map[string]ParameterInfo `json:"parameters"`
	Required   []string                 `json:"required"`
}

// ParameterInfo describes a function parameter.
type ParameterInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// NewFunctionTool creates a new function tool from a Go function.
func NewFunctionTool(name, description string, fn interface{}) (*FunctionTool, error) {
	schema, err := analyzeFunctionSchema(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze function schema: %w", err)
	}

	return &FunctionTool{
		BaseToolImpl: NewBaseTool(name, description),
		function:     fn,
		schema:       schema,
	}, nil
}

// GetDeclaration returns the function declaration for LLM integration.
func (t *FunctionTool) GetDeclaration() *core.FunctionDeclaration {
	// Convert schema to declaration format
	parameters := make(map[string]interface{})
	parameters["type"] = "object"
	parameters["properties"] = t.schema.Parameters
	parameters["required"] = t.schema.Required

	return &core.FunctionDeclaration{
		Name:        t.name,
		Description: t.description,
		Parameters:  parameters,
	}
}

// RunAsync executes the wrapped function with the given arguments.
func (t *FunctionTool) RunAsync(toolCtx *core.ToolContext, args map[string]any) (any, error) {
	fnValue := reflect.ValueOf(t.function)
	fnType := fnValue.Type()

	// Prepare function arguments
	callArgs := make([]reflect.Value, fnType.NumIn())

	log.Printf("Function %s called with arguments: %+v", t.name, args)

	argIndex := 0 // Track non-context arguments
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		// Special handling for ToolContext
		if paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			callArgs[i] = reflect.ValueOf(toolCtx)
			continue
		}

		// For regular parameters, look for them by index
		paramName := fmt.Sprintf("param%d", argIndex)
		argIndex++

		// Check if argument is provided
		if argValue, exists := args[paramName]; exists {
			// Convert and assign the argument
			convertedValue, err := convertArgument(argValue, paramType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert argument %s: %w", paramName, err)
			}
			callArgs[i] = convertedValue
		} else {
			// Use zero value for missing arguments
			callArgs[i] = reflect.Zero(paramType)
		}
	}

	// Call the function
	results := fnValue.Call(callArgs)

	// Handle return values
	if len(results) == 0 {
		return nil, nil
	}

	// If last return value is error, check it
	if len(results) >= 2 {
		lastResult := results[len(results)-1]
		if lastResult.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !lastResult.IsNil() {
				return nil, lastResult.Interface().(error)
			}
			// Return the first non-error result
			if len(results) > 1 {
				return results[0].Interface(), nil
			}
		}
	}

	// Return the first result
	return results[0].Interface(), nil
}

// AgentTool wraps another agent as a tool.
type AgentTool struct {
	*BaseToolImpl
	agent core.BaseAgent
}

// NewAgentTool creates a new agent tool.
func NewAgentTool(agent core.BaseAgent) *AgentTool {
	return &AgentTool{
		BaseToolImpl: NewBaseTool(agent.Name(), agent.Description()),
		agent:        agent,
	}
}

// GetDeclaration returns the function declaration for the agent tool.
func (t *AgentTool) GetDeclaration() *core.FunctionDeclaration {
	return &core.FunctionDeclaration{
		Name:        t.name,
		Description: t.description,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"request": map[string]interface{}{
					"type":        "string",
					"description": "The request to send to the agent",
				},
			},
			"required": []string{"request"},
		},
	}
}

// RunAsync executes the wrapped agent with the given request.
func (t *AgentTool) RunAsync(toolCtx *core.ToolContext, args map[string]any) (any, error) {
	request, ok := args["request"].(string)
	if !ok {
		return nil, fmt.Errorf("request parameter must be a string")
	}

	// Create a new invocation context for the agent
	agentCtx := toolCtx.InvocationContext

	// Set the user content
	agentCtx.UserContent = &core.Content{
		Role: "user",
		Parts: []core.Part{
			{
				Type: "text",
				Text: &request,
			},
		},
	}

	// Run the agent
	eventStream, err := t.agent.RunAsync(toolCtx.InvocationContext)
	if err != nil {
		return nil, fmt.Errorf("failed to run agent %s: %w", t.agent.Name(), err)
	}

	// Collect all events and return the final result
	var lastEvent *core.Event
	for event := range eventStream {
		lastEvent = event

		// Apply state changes from the agent to the tool context
		if len(event.Actions.StateDelta) > 0 {
			toolCtx.State.Update(event.Actions.StateDelta)
		}
	}

	if lastEvent == nil || lastEvent.Content == nil {
		return "", nil
	}

	// Extract text from the last event
	var result string
	for _, part := range lastEvent.Content.Parts {
		if part.Text != nil {
			result += *part.Text + "\n"
		}
	}

	return result, nil
}

// analyzeFunctionSchema analyzes a Go function and extracts its schema.
func analyzeFunctionSchema(fn interface{}) (*FunctionSchema, error) {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected function, got %s", fnType.Kind())
	}

	schema := &FunctionSchema{
		Parameters: make(map[string]ParameterInfo),
		Required:   make([]string, 0),
	}

	// Analyze input parameters
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		paramName := fmt.Sprintf("param%d", i) // This should be improved

		// Skip context.Context and ToolContext parameters
		if paramType == reflect.TypeOf((*context.Context)(nil)).Elem() ||
			paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			continue
		}

		schema.Parameters[paramName] = ParameterInfo{
			Type:        mapGoTypeToJSONType(paramType),
			Description: fmt.Sprintf("Parameter %d", i),
		}
		schema.Required = append(schema.Required, paramName)
	}

	return schema, nil
}

// mapGoTypeToJSONType maps Go types to JSON schema types.
func mapGoTypeToJSONType(goType reflect.Type) string {
	switch goType.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string" // Default to string for unknown types
	}
}

// convertArgument converts an interface{} value to the target type using reflection
func convertArgument(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	if value == nil {
		return reflect.Zero(targetType), nil
	}

	sourceValue := reflect.ValueOf(value)
	sourceType := sourceValue.Type()

	// If types match exactly, return as-is
	if sourceType == targetType {
		return sourceValue, nil
	}

	// If source type is convertible to target type
	if sourceType.ConvertibleTo(targetType) {
		return sourceValue.Convert(targetType), nil
	}

	// Handle some common conversions
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("%v", value)), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if sourceType.Kind() == reflect.Float64 || sourceType.Kind() == reflect.Float32 {
			// Convert float to int
			floatVal := sourceValue.Float()
			return reflect.ValueOf(int64(floatVal)).Convert(targetType), nil
		}
	case reflect.Float32, reflect.Float64:
		if sourceType.Kind() >= reflect.Int && sourceType.Kind() <= reflect.Int64 {
			// Convert int to float
			intVal := sourceValue.Int()
			return reflect.ValueOf(float64(intVal)).Convert(targetType), nil
		}
	case reflect.Bool:
		if sourceType.Kind() == reflect.String {
			str := sourceValue.String()
			boolVal := str == "true" || str == "1" || str == "yes"
			return reflect.ValueOf(boolVal), nil
		}
	}

	return reflect.Zero(targetType), fmt.Errorf("cannot convert %v (%s) to %s", value, sourceType, targetType)
}
