# Enhanced FunctionTool Implementation

This implementation provides an advanced FunctionTool system for the ADK Go framework that can wrap Go functions and make them callable by AI agents. It includes comprehensive parameter validation, JSON schema generation, and enhanced ToolContext functionality.

## Features

### 🚀 Enhanced FunctionTool

- **Automatic Parameter Extraction**: Analyzes Go function signatures and generates parameter schemas
- **Type-Safe Parameter Validation**: Validates parameters before function execution
- **JSON Schema Generation**: Creates OpenAPI-compatible schemas for LLM integration
- **Smart Type Conversion**: Automatically converts between JSON and Go types
- **Error Handling**: Returns validation errors in a format compatible with the Python ADK
- **Multiple Return Values**: Handles functions with multiple return values
- **Context Support**: Seamlessly integrates with Go's `context.Context` and ADK's `ToolContext`

### 🛠️ Enhanced ToolContext

The ToolContext has been significantly enhanced with methods similar to the Python ADK implementation:

```go
type ToolContext struct {
    InvocationContext *InvocationContext
    State             *State
    Actions           *EventActions
    FunctionCallID    *string
}
```

#### Available Methods:

- **Artifact Management**:
  - `SaveArtifact(ctx, filename, content, mimeType) (int, error)`
  - `LoadArtifact(ctx, filename, version) ([]byte, error)`
  - `ListArtifacts(ctx) ([]string, error)`

- **Memory Operations**:
  - `SearchMemory(ctx, query, limit) ([]*Event, error)`

- **Authentication**:
  - `RequestCredential(credentialID, authConfig) error`
  - `GetCredential(ctx, credentialID) (*Credential, error)`

- **State Management**:
  - `SetState(key, value)`
  - `GetState(key) (any, bool)`
  - `GetStateWithDefault(key, defaultValue) any`

- **Control Flow**:
  - `TransferToAgent(agentName)`
  - `Escalate()`
  - `SkipSummarization()`

## Usage Examples

### Basic Function Wrapping

```go
// Simple function
func AddNumbers(a, b int) int {
    return a + b
}

// Create a tool
tool, err := tools.NewEnhancedFunctionTool(
    "add_numbers",
    "Adds two integers",
    AddNumbers,
)

// Get the function declaration for LLM
decl := tool.GetDeclaration()
// Returns JSON schema with parameters for 'a' and 'b' as integers
```

### Context-Aware Functions

```go
func CalculateWithContext(ctx context.Context, operation string, a, b float64) (float64, error) {
    select {
    case <-ctx.Done():
        return 0, ctx.Err()
    default:
    }

    switch operation {
    case "add":
        return a + b, nil
    case "divide":
        if b == 0 {
            return 0, fmt.Errorf("division by zero")
        }
        return a / b, nil
    default:
        return 0, fmt.Errorf("unsupported operation: %s", operation)
    }
}

tool, _ := tools.NewEnhancedFunctionTool("calculator", "Mathematical operations", CalculateWithContext)
```

### ToolContext Integration

```go
func FormatTextWithToolContext(toolCtx *core.ToolContext, text string, format string) (string, error) {
    // Access session state
    prefix, _ := toolCtx.GetState("text_prefix")
    if prefix == nil {
        prefix = ""
    }

    // Set state for future use
    toolCtx.SetState("last_formatted_text", text)

    switch format {
    case "upper":
        return fmt.Sprintf("%s%s", prefix, strings.ToUpper(text)), nil
    case "lower":
        return fmt.Sprintf("%s%s", prefix, strings.ToLower(text)), nil
    default:
        return fmt.Sprintf("%s%s", prefix, text), nil
    }
}
```

### Complex Data Types

```go
type UserInfo struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email"`
}

func CreateUserProfile(name string, age int, email string) UserInfo {
    return UserInfo{Name: name, Age: age, Email: email}
}

// Automatically generates schema for struct return type
tool, _ := tools.NewEnhancedFunctionTool("create_user", "Creates user profile", CreateUserProfile)
```

### Array Processing

```go
func ProcessItems(items []string, operation string) ([]string, error) {
    result := make([]string, len(items))
    for i, item := range items {
        switch operation {
        case "upper":
            result[i] = strings.ToUpper(item)
        case "lower":
            result[i] = strings.ToLower(item)
        default:
            return nil, fmt.Errorf("unsupported operation: %s", operation)
        }
    }
    return result, nil
}
```

## Generated JSON Schema

The FunctionTool automatically generates OpenAPI-compatible JSON schemas:

```json
{
  "name": "calculator",
  "description": "Mathematical operations",
  "parameters": {
    "type": "object",
    "properties": {
      "string": {
        "type": "string",
        "description": "Parameter: string"
      },
      "float64": {
        "type": "number",
        "description": "Parameter: float64"
      },
      "float641": {
        "type": "number",
        "description": "Parameter: float641"
      }
    },
    "required": ["string", "float64", "float641"]
  }
}
```

## Parameter Validation

The system provides comprehensive parameter validation:

- **Type Checking**: Ensures parameters match expected Go types
- **Required Parameters**: Validates all required parameters are present
- **Type Conversion**: Automatically converts compatible types
- **Error Reporting**: Returns descriptive error messages

```go
// If validation fails, returns:
{
  "error": "Parameter validation failed: missing required parameter: operation"
}
```

## Error Handling

Following the Python ADK pattern, errors are returned as part of the result object rather than Go errors:

```go
// Division by zero example:
result := map[string]interface{}{
    "error": "division by zero"
}
```

## Performance Features

- **Efficient Reflection**: Minimal runtime reflection overhead
- **Schema Caching**: Function schemas are analyzed once at creation time
- **Concurrent Execution**: Safe for concurrent use across multiple goroutines
- **Memory Efficient**: Reuses reflection data and avoids unnecessary allocations

## Advanced Features

### Function Validation

```go
func ValidateFunction(fn interface{}) error {
    // Validates that a function is suitable for wrapping
    // - Must be a function
    // - Parameters must be mappable to JSON types
    // - Return types must be supported
}
```

### Metadata Extraction

```go
type FunctionMetadata struct {
    Name        string
    Description string
    Parameters  []ParameterMetadata
    ReturnType  string
    IsAsync     bool
    HasError    bool
}

metadata := tool.GetMetadata()
```

### Custom Parameter Names

The system attempts to generate meaningful parameter names based on types, with automatic disambiguation:

- `int` → "int"
- `int, int` → "int", "int1"
- `string, float64, float64` → "string", "float64", "float641"

## Integration with ADK Agents

The FunctionTool integrates seamlessly with ADK agents:

```go
// Create an agent
agent := agents.NewEnhancedLlmAgent("my-agent", "AI Assistant", nil)

// Add function tools
calculatorTool, _ := tools.NewEnhancedFunctionTool("calc", "Calculator", CalculateWithContext)
textTool, _ := tools.NewEnhancedFunctionTool("format", "Text formatter", FormatTextWithToolContext)

agent.AddTool(calculatorTool)
agent.AddTool(textTool)

// The agent can now call these functions based on LLM function calling
```

## Testing

Comprehensive test suite covers:

- Function tool creation and validation
- Parameter extraction and schema generation
- Type conversion and validation
- Error handling and edge cases
- Context integration
- Concurrent execution
- Performance benchmarks

Run tests with:

```bash
go test ./pkg/tools/...
```

## Comparison with Python ADK

| Feature | Python ADK | Go ADK (Enhanced) |
|---------|------------|-------------------|
| Function Wrapping | ✅ | ✅ |
| Parameter Validation | ✅ | ✅ |
| JSON Schema Generation | ✅ | ✅ |
| Context Integration | ✅ | ✅ |
| Type Safety | ⚠️ | ✅ (Compile-time) |
| Performance | Good | Excellent |
| Async Support | ✅ | ✅ (via context) |
| Error Handling | ✅ | ✅ |

## Future Enhancements

- **Debug Symbol Integration**: Extract actual parameter names from debug info
- **Struct Tag Support**: Use JSON tags for parameter naming and validation
- **OpenAPI Extensions**: Support for more advanced OpenAPI features
- **Custom Validators**: Allow custom validation functions
- **Streaming Support**: Integration with streaming tools for long-running operations

## Files Structure

```
pkg/tools/
├── function_tool.go              # Enhanced FunctionTool implementation
├── function_tool_examples.go     # Example functions and usage
├── function_tool_test.go         # Comprehensive test suite
└── base.go                       # Base tool implementations

internal/core/
└── context.go                    # Enhanced ToolContext with new methods

examples/enhanced_function_tool_demo/
└── main.go                       # Complete demonstration
```

This implementation provides a robust, type-safe, and performant foundation for creating AI agents that can seamlessly interact with Go functions while maintaining compatibility with the broader ADK ecosystem.
