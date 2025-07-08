# Enhanced Session Management System

This directory contains a comprehensive session management system for the ADK Golang implementation, providing both basic session operations and advanced utilities for state management, event handling, and session lifecycle management.

## Core Components

### 1. Session Structure (`pkg/core/context.go`)

The `Session` struct provides the foundation for session management:

```go
type Session struct {
    ID             string         `json:"id"`
    AppName        string         `json:"app_name"`
    UserID         string         `json:"user_id"`
    State          map[string]any `json:"state"`
    Events         []*Event       `json:"events"`
    LastUpdateTime time.Time      `json:"last_update_time"`
}
```

Key features:
- **30+ methods** for comprehensive session operations
- **State management** with scoped keys (app:, user:, temp:)
- **Event handling** with automatic timestamp management
- **Validation** and **cloning** capabilities
- **Thread-safe** operations

### 2. Session Service Interfaces (`pkg/sessions/interfaces.go`)

Defines the core interfaces for session management:

- `SessionService` - Main interface with 11 methods for session CRUD operations
- `StateManager` - Advanced state management with scoped operations
- `EventHandler` - Pluggable event lifecycle management
- `SessionConfiguration` - Flexible configuration options

### 3. Storage Implementations

#### In-Memory Storage (`pkg/sessions/memory.go`)
- Thread-safe concurrent operations using `sync.RWMutex`
- Automatic session cleanup with configurable TTL
- Memory-efficient operations with map-based storage

#### File-Based Storage (`pkg/sessions/file.go`)
- Hierarchical file organization: `sessions/{app_name}/{user_id}/{session_id}.json`
- Atomic write operations for data consistency
- Automatic directory creation and cleanup

### 4. Advanced State Management (`pkg/sessions/state.go`)

The `DefaultStateManager` provides:
- **Scoped state keys**: `app:`, `user:`, `session:`, `temp:`
- **Helper operations**: increment, toggle, list management
- **Type-safe** state operations with validation
- **Context-aware** operations for better error handling

### 5. Session Utilities (`pkg/core/session_utils.go`)

The `SessionStateHelper` provides convenient methods for:

#### Type-Safe State Operations
- String, int, bool, float64, time.Time operations
- Default value fallbacks
- Automatic type conversions where possible

#### Collection Operations
- Slice operations: append, prepend, remove, pop
- Map operations: nested key management
- JSON marshaling/unmarshaling

#### Advanced Features
- Session metrics and analytics
- Snapshot creation and restoration
- State diffing between sessions
- Comprehensive validation

### 6. Event Handlers (`pkg/sessions/handlers.go`)

Pluggable event system with multiple handler types:
- `LoggingEventHandler` - Event logging and debugging
- `MetricsEventHandler` - Performance and usage metrics
- `ValidationEventHandler` - Event validation and sanitization
- `CompositeEventHandler` - Multiple handler composition

### 7. Utilities and Factory (`pkg/sessions/utils.go`)

- `SessionServiceBuilder` - Fluent API for service configuration
- `SessionBackupManager` - Backup and restore operations
- `SessionServiceUtils` - Common utility functions

## Usage Examples

### Basic Session Operations

```go
// Create a new session
session := core.NewSession("session-1", "my-app", "user-123")

// Basic state operations
session.SetState("username", "Alice")
session.SetState("score", 100)

username, exists := session.GetState("username")
session.UpdateState(map[string]any{
    "level": 5,
    "premium": true,
})

// Event management
event := &core.Event{
    ID:        "evt-1",
    Author:    "user",
    Timestamp: time.Now(),
    Content:   &core.Content{...},
}
session.AddEvent(event)

lastEvent := session.GetLastEvent()
eventCount := session.GetEventCount()
```

### Advanced State Management with Helper

```go
helper := core.NewSessionStateHelper(session)

// Type-safe operations
helper.SetString("username", "Alice")
helper.SetInt("score", 100)
helper.SetBool("premium", true)

// Numeric operations
newScore, _ := helper.Increment("score", 25)
helper.Toggle("premium")

// Collection operations
helper.SetSlice("tags", []any{"beginner", "active"})
helper.AppendToSlice("tags", "premium")
tags, _ := helper.GetSlice("tags")

// JSON operations
type Profile struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}
profile := Profile{Name: "Alice", Age: 28}
helper.SetJSON("profile", profile)

var retrieved Profile
helper.GetJSON("profile", &retrieved)
```

### Session Services

```go
// In-memory service
memoryService := sessions.NewInMemorySessionService(&sessions.SessionConfiguration{
    MaxSessionsPerUser: 10,
    SessionTTL:         time.Hour * 24,
    EnableAutoCleanup:  true,
})

// File-based service
fileService := sessions.NewFileSessionService(&sessions.SessionConfiguration{
    BasePath:           "./data/sessions",
    EnableCompression:  true,
    BackupEnabled:      true,
})

// Create session
session, err := memoryService.CreateSession(context.Background(), "my-app", "user-123", nil)

// Get session with events
config := &sessions.GetSessionConfig{
    IncludeEvents: true,
    MaxEvents:     100,
}
session, err = memoryService.GetSession(context.Background(), "my-app", "user-123", "session-id", config)

// Update session state
stateDelta := map[string]any{
    "score": 150,
    "level": 6,
}
err = memoryService.UpdateSessionState(context.Background(), session, stateDelta)
```

### Metrics and Analytics

```go
// Get comprehensive metrics
metrics := session.GetMetrics()
fmt.Printf("Events: %d, Errors: %d, Function Calls: %d\n", 
    metrics.EventCount, metrics.ErrorCount, metrics.FunctionCallCount)

// Create snapshots
snapshot := session.CreateSnapshot()
// ... modify session ...
session.RestoreFromSnapshot(snapshot)

// State diffing
otherState := map[string]any{"score": 200, "level": 7}
diff := session.DiffState(otherState)
fmt.Printf("Added: %v, Modified: %v, Removed: %v\n", 
    diff.Added, diff.Modified, diff.Removed)
```

## Architecture Benefits

1. **Modularity**: Clear separation between interfaces, implementations, and utilities
2. **Extensibility**: Plugin architecture for event handlers and custom storage backends
3. **Performance**: Efficient memory usage and optional compression
4. **Reliability**: Thread-safe operations and atomic writes
5. **Developer Experience**: Type-safe operations and comprehensive error handling
6. **Observability**: Built-in metrics, logging, and debugging capabilities

## Testing

The system includes comprehensive tests covering:
- Core session operations
- State management utilities
- Service implementations
- Event handling
- Error conditions and edge cases

Run tests with:
```bash
go test ./pkg/sessions/... -v
go test ./pkg/core/... -v
```

## Examples

- `examples/sessions/main.go` - Basic session service usage
- `examples/enhanced_sessions/main.go` - Advanced features demonstration

This session management system provides a solid foundation for building stateful AI agents with the ADK framework, supporting both simple use cases and complex multi-agent workflows.
