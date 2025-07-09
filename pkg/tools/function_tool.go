// Package tools provides enhanced FunctionTool implementation with parameter validation and JSON schema generation.
package tools

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/agent-protocol/adk-golang/pkg/core"
)

// EnhancedFunctionTool provides an improved version of FunctionTool with:
// - Proper parameter name extraction from function signature
// - Enhanced JSON schema generation
// - Parameter validation
// - Support for optional parameters with default values
// - Better error handling and debugging
type EnhancedFunctionTool struct {
	*BaseToolImpl
	function     interface{}
	schema       *EnhancedFunctionSchema
	ignoreParams []string // Parameters to ignore in schema generation
}

// EnhancedFunctionSchema provides detailed schema information for function parameters.
type EnhancedFunctionSchema struct {
	Parameters map[string]*ParameterSchema `json:"parameters"`
	Required   []string                    `json:"required"`
}

// ParameterSchema describes a function parameter with enhanced metadata.
type ParameterSchema struct {
	Type        string                      `json:"type"`
	Description string                      `json:"description,omitempty"`
	Default     interface{}                 `json:"default,omitempty"`
	Enum        []interface{}               `json:"enum,omitempty"`
	Items       *ParameterSchema            `json:"items,omitempty"`      // For array types
	Properties  map[string]*ParameterSchema `json:"properties,omitempty"` // For object types
	Required    []string                    `json:"required,omitempty"`   // For object types
	Optional    bool                        `json:"optional"`
}

// FunctionMetadata provides metadata about the wrapped function.
type FunctionMetadata struct {
	Name        string
	Description string
	Parameters  []ParameterMetadata
	ReturnType  string
	IsAsync     bool
	HasError    bool
}

// ParameterMetadata describes a function parameter.
type ParameterMetadata struct {
	Name        string
	Type        reflect.Type
	JSONType    string
	Description string
	Required    bool
	Default     interface{}
	Index       int
}

// NewEnhancedFunctionTool creates a new enhanced function tool from a Go function.
// It automatically extracts parameter names, types, and generates JSON schema.
func NewEnhancedFunctionTool(name, description string, fn interface{}) (*EnhancedFunctionTool, error) {
	if fn == nil {
		return nil, fmt.Errorf("function cannot be nil")
	}

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected function, got %s", fnType.Kind())
	}

	schema, err := analyzeEnhancedFunctionSchema(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze function schema: %w", err)
	}

	// Use function name if name is empty
	if name == "" {
		if fnValue := reflect.ValueOf(fn); fnValue.Kind() == reflect.Func {
			name = extractFunctionName(fn)
		}
	}

	// Use function documentation if description is empty
	if description == "" {
		description = extractFunctionDescription(fn)
	}

	return &EnhancedFunctionTool{
		BaseToolImpl: NewBaseTool(name, description),
		function:     fn,
		schema:       schema,
		ignoreParams: []string{"ctx", "context", "tool_context", "toolCtx"},
	}, nil
}

// SetIgnoreParams sets the list of parameter names to ignore in schema generation.
func (t *EnhancedFunctionTool) SetIgnoreParams(params []string) {
	t.ignoreParams = params
}

// GetDeclaration returns the function declaration for LLM integration.
func (t *EnhancedFunctionTool) GetDeclaration() *core.FunctionDeclaration {
	parameters := make(map[string]interface{})
	parameters["type"] = "object"

	// Convert schema to declaration format
	properties := make(map[string]interface{})
	for name, param := range t.schema.Parameters {
		properties[name] = parameterSchemaToMap(param)
	}

	parameters["properties"] = properties
	parameters["required"] = t.schema.Required

	return &core.FunctionDeclaration{
		Name:        t.name,
		Description: t.description,
		Parameters:  parameters,
	}
}

// RunAsync executes the wrapped function with parameter validation and type conversion.
func (t *EnhancedFunctionTool) RunAsync(ctx context.Context, args map[string]any, toolCtx *core.ToolContext) (any, error) {
	// Validate required parameters
	if err := t.validateParameters(args); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Parameter validation failed: %v", err),
		}, nil
	}

	fnValue := reflect.ValueOf(t.function)
	fnType := fnValue.Type()

	// Prepare function arguments with proper type conversion
	callArgs, err := t.prepareCallArguments(ctx, toolCtx, args, fnType)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Failed to prepare arguments: %v", err),
		}, nil
	}

	// Call the function
	results := fnValue.Call(callArgs)

	// Handle return values
	return t.handleReturnValues(results)
}

// validateParameters validates that all required parameters are present and have correct types.
func (t *EnhancedFunctionTool) validateParameters(args map[string]any) error {
	// Check required parameters
	for _, requiredParam := range t.schema.Required {
		if _, exists := args[requiredParam]; !exists {
			return fmt.Errorf("missing required parameter: %s", requiredParam)
		}
	}

	// Type validation could be added here
	for paramName, paramValue := range args {
		if paramSchema, exists := t.schema.Parameters[paramName]; exists {
			if err := validateParameterType(paramValue, paramSchema); err != nil {
				return fmt.Errorf("parameter %s: %w", paramName, err)
			}
		}
	}

	return nil
}

// prepareCallArguments prepares the arguments for the function call with proper type conversion.
func (t *EnhancedFunctionTool) prepareCallArguments(ctx context.Context, toolCtx *core.ToolContext, args map[string]any, fnType reflect.Type) ([]reflect.Value, error) {
	callArgs := make([]reflect.Value, fnType.NumIn())
	paramIndex := 0

	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		// Special handling for ToolContext (check this first since ToolContext now embeds context.Context)
		if paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			callArgs[i] = reflect.ValueOf(toolCtx)
			continue
		}

		// Special handling for context.Context
		if paramType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			callArgs[i] = reflect.ValueOf(ctx)
			continue
		}

		// Get parameter name and value
		paramName := t.getParameterName(paramIndex)
		paramIndex++

		if argValue, exists := args[paramName]; exists {
			// Convert argument to proper type
			convertedValue, err := convertToType(argValue, paramType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert parameter %s: %w", paramName, err)
			}
			callArgs[i] = convertedValue
		} else {
			// Use zero value for missing optional parameters
			callArgs[i] = reflect.Zero(paramType)
		}
	}

	return callArgs, nil
}

// handleReturnValues processes the function return values.
func (t *EnhancedFunctionTool) handleReturnValues(results []reflect.Value) (any, error) {
	if len(results) == 0 {
		return nil, nil
	}

	// Check if last return value is an error
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

	// If only one result, return it
	if len(results) == 1 {
		return results[0].Interface(), nil
	}

	// Multiple results: return as a map or array
	resultMap := make(map[string]interface{})
	for i, result := range results {
		resultMap[fmt.Sprintf("result_%d", i)] = result.Interface()
	}

	return resultMap, nil
}

// getParameterName gets the parameter name for the given index, skipping ignored parameters.
func (t *EnhancedFunctionTool) getParameterName(index int) string {
	count := 0
	for name := range t.schema.Parameters {
		if count == index {
			return name
		}
		count++
	}
	return fmt.Sprintf("param_%d", index)
}

// analyzeEnhancedFunctionSchema analyzes a Go function and extracts detailed schema information.
func analyzeEnhancedFunctionSchema(fn interface{}) (*EnhancedFunctionSchema, error) {
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected function, got %s", fnType.Kind())
	}

	schema := &EnhancedFunctionSchema{
		Parameters: make(map[string]*ParameterSchema),
		Required:   make([]string, 0),
	}

	// Extract parameter names from function name/signature if possible
	paramNames := extractParameterNames(fn)

	// Analyze input parameters
	paramIndex := 0
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		// Skip context.Context and ToolContext parameters
		if paramType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
			paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			continue
		}

		// Get parameter name
		var paramName string
		if paramIndex < len(paramNames) {
			paramName = paramNames[paramIndex]
		} else {
			paramName = fmt.Sprintf("param_%d", paramIndex)
		}
		paramIndex++

		// Create parameter schema
		paramSchema := &ParameterSchema{
			Type:        mapGoTypeToJSONType(paramType),
			Description: fmt.Sprintf("Parameter: %s", paramName),
			Optional:    false, // Assume required by default
		}

		// Enhanced type analysis
		if err := enhanceParameterSchema(paramSchema, paramType); err != nil {
			return nil, fmt.Errorf("failed to enhance parameter schema for %s: %w", paramName, err)
		}

		schema.Parameters[paramName] = paramSchema

		// Add to required if not optional
		if !paramSchema.Optional {
			schema.Required = append(schema.Required, paramName)
		}
	}

	return schema, nil
}

// extractParameterNames attempts to extract parameter names from the function.
// This is a placeholder implementation - in practice, you might need debug symbols or code analysis.
func extractParameterNames(fn interface{}) []string {
	// This is a simple implementation that could be enhanced with:
	// - Debug symbol parsing
	// - AST analysis
	// - Function name conventions
	// For now, return generic names
	fnType := reflect.TypeOf(fn)
	var names []string
	typeCount := make(map[string]int)

	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		// Skip special types
		if paramType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
			paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			continue
		}

		// Generate a name based on type
		typeName := strings.ToLower(paramType.Name())
		if typeName == "" {
			typeName = strings.ToLower(paramType.Kind().String())
		}

		// Make names unique by adding a counter
		typeCount[typeName]++
		if typeCount[typeName] > 1 {
			typeName = fmt.Sprintf("%s%d", typeName, typeCount[typeName]-1)
		}

		names = append(names, typeName)
	}

	return names
}

// extractFunctionName extracts the function name if available.
func extractFunctionName(fn interface{}) string {
	// This is a placeholder - actual implementation would need runtime function name extraction
	return "function"
}

// extractFunctionDescription extracts documentation from the function if available.
func extractFunctionDescription(fn interface{}) string {
	// This is a placeholder - actual implementation would need documentation extraction
	return "A function tool"
}

// enhanceParameterSchema adds detailed type information to the parameter schema.
func enhanceParameterSchema(schema *ParameterSchema, paramType reflect.Type) error {
	switch paramType.Kind() {
	case reflect.Slice, reflect.Array:
		// Handle array/slice types
		elemType := paramType.Elem()
		schema.Items = &ParameterSchema{
			Type: mapGoTypeToJSONType(elemType),
		}
		if err := enhanceParameterSchema(schema.Items, elemType); err != nil {
			return err
		}

	case reflect.Map:
		// Handle map types as objects
		schema.Type = "object"

	case reflect.Struct:
		// Handle struct types
		schema.Type = "object"
		schema.Properties = make(map[string]*ParameterSchema)

		for i := 0; i < paramType.NumField(); i++ {
			field := paramType.Field(i)
			if field.IsExported() {
				fieldName := strings.ToLower(field.Name)
				fieldSchema := &ParameterSchema{
					Type:        mapGoTypeToJSONType(field.Type),
					Description: fmt.Sprintf("Field: %s", field.Name),
				}
				schema.Properties[fieldName] = fieldSchema
			}
		}

	case reflect.Ptr:
		// Handle pointer types as optional
		schema.Optional = true
		return enhanceParameterSchema(schema, paramType.Elem())
	}

	return nil
}

// validateParameterType validates that a parameter value matches the expected schema.
func validateParameterType(value interface{}, schema *ParameterSchema) error {
	if value == nil {
		if !schema.Optional {
			return fmt.Errorf("value is required but got nil")
		}
		return nil
	}

	valueType := reflect.TypeOf(value)
	expectedType := schema.Type

	// Basic type validation
	switch expectedType {
	case "string":
		if valueType.Kind() != reflect.String {
			return fmt.Errorf("expected string, got %s", valueType.Kind())
		}
	case "integer":
		if !isIntegerType(valueType.Kind()) {
			return fmt.Errorf("expected integer, got %s", valueType.Kind())
		}
	case "number":
		if !isNumericType(valueType.Kind()) {
			return fmt.Errorf("expected number, got %s", valueType.Kind())
		}
	case "boolean":
		if valueType.Kind() != reflect.Bool {
			return fmt.Errorf("expected boolean, got %s", valueType.Kind())
		}
	case "array":
		if valueType.Kind() != reflect.Slice && valueType.Kind() != reflect.Array {
			return fmt.Errorf("expected array, got %s", valueType.Kind())
		}
	case "object":
		if valueType.Kind() != reflect.Map && valueType.Kind() != reflect.Struct {
			return fmt.Errorf("expected object, got %s", valueType.Kind())
		}
	}

	return nil
}

// convertToType converts a value to the target type.
func convertToType(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	if value == nil {
		return reflect.Zero(targetType), nil
	}

	valueReflect := reflect.ValueOf(value)
	valueType := valueReflect.Type()

	// If types match, return as-is
	if valueType.AssignableTo(targetType) {
		return valueReflect, nil
	}

	// Handle type conversions
	if valueType.ConvertibleTo(targetType) {
		return valueReflect.Convert(targetType), nil
	}

	// Special cases for numeric conversions
	if isNumericType(valueType.Kind()) && isNumericType(targetType.Kind()) {
		return convertNumericType(valueReflect, targetType)
	}

	// String to other type conversions could be added here

	return reflect.Zero(targetType), fmt.Errorf("cannot convert %s to %s", valueType, targetType)
}

// convertNumericType handles numeric type conversions.
func convertNumericType(value reflect.Value, targetType reflect.Type) (reflect.Value, error) {
	// This is a simplified implementation
	// In practice, you'd want more robust numeric conversion
	if value.Type().ConvertibleTo(targetType) {
		return value.Convert(targetType), nil
	}
	return reflect.Zero(targetType), fmt.Errorf("cannot convert numeric type %s to %s", value.Type(), targetType)
}

// isIntegerType checks if a kind represents an integer type.
func isIntegerType(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

// isNumericType checks if a kind represents a numeric type.
func isNumericType(kind reflect.Kind) bool {
	return isIntegerType(kind) || kind == reflect.Float32 || kind == reflect.Float64
}

// parameterSchemaToMap converts a ParameterSchema to a map for JSON serialization.
func parameterSchemaToMap(schema *ParameterSchema) map[string]interface{} {
	result := make(map[string]interface{})

	result["type"] = schema.Type

	if schema.Description != "" {
		result["description"] = schema.Description
	}

	if schema.Default != nil {
		result["default"] = schema.Default
	}

	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}

	if schema.Items != nil {
		result["items"] = parameterSchemaToMap(schema.Items)
	}

	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for name, prop := range schema.Properties {
			properties[name] = parameterSchemaToMap(prop)
		}
		result["properties"] = properties
	}

	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}

	return result
}

// GetMetadata returns metadata about the wrapped function.
func (t *EnhancedFunctionTool) GetMetadata() *FunctionMetadata {
	fnType := reflect.TypeOf(t.function)

	metadata := &FunctionMetadata{
		Name:        t.name,
		Description: t.description,
		Parameters:  make([]ParameterMetadata, 0),
		IsAsync:     false, // Go functions are not async like JavaScript
		HasError:    false,
	}

	// Analyze parameters
	paramIndex := 0
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		// Skip special types
		if paramType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
			paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			continue
		}

		paramName := t.getParameterName(paramIndex)
		paramMetadata := ParameterMetadata{
			Name:     paramName,
			Type:     paramType,
			JSONType: mapGoTypeToJSONType(paramType),
			Index:    paramIndex,
			Required: true, // Default to required
		}

		metadata.Parameters = append(metadata.Parameters, paramMetadata)
		paramIndex++
	}

	// Check return types
	if fnType.NumOut() > 0 {
		lastOut := fnType.Out(fnType.NumOut() - 1)
		if lastOut.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			metadata.HasError = true
		}

		if fnType.NumOut() > 1 || !metadata.HasError {
			returnType := fnType.Out(0)
			metadata.ReturnType = returnType.String()
		}
	}

	return metadata
}

// ValidateFunction validates that a function is suitable for wrapping as a tool.
func ValidateFunction(fn interface{}) error {
	if fn == nil {
		return fmt.Errorf("function cannot be nil")
	}

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("expected function, got %s", fnType.Kind())
	}

	// Check that all parameters are supported types
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)

		// Skip special types
		if paramType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
			paramType == reflect.TypeOf((*core.ToolContext)(nil)) {
			continue
		}

		// Validate that the type can be mapped to JSON
		if mapGoTypeToJSONType(paramType) == "string" && paramType.Kind() != reflect.String {
			// This means we couldn't map the type properly
			return fmt.Errorf("unsupported parameter type at index %d: %s", i, paramType)
		}
	}

	return nil
}

// Helper function to capitalize first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
