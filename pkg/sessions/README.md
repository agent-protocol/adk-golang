# Session Management System

The ADK Go session management system provides comprehensive session persistence, state management, and lifecycle handling for AI agent conversations. It supports multiple storage backends and offers advanced features like scoped state, event handlers, backup/restore, and automatic cleanup.

## Features

- **Multiple Storage Backends**: In-memory and file-based persistence
- **Scoped State Management**: App, user, session, and temporary state scopes
- **Event Lifecycle Handlers**: Logging, metrics, validation, and custom handlers  
- **Session Utilities**: Backup/restore, filtering, merging, and helper functions
- **Auto-cleanup**: Configurable session expiration and cleanup
- **Thread-safe Operations**: Concurrent access with proper locking
- **Extensible Architecture**: Easy to add new backends and handlers

## Quick Start

### Basic Usage with In-Memory Storage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/agent-protocol/adk-golang/internal/core"
    "github.com/agent-protocol/adk-golang/pkg/sessions"
)

func main() {
    ctx := context.Background()
    
    // Create in-memory session service
    service := sessions.NewInMemorySessionService()
    defer service.Close(ctx)
    
    // Create a session
    createReq := &core.CreateSessionRequest{
        AppName: "my_app",
        UserID:  "user123",
        State:   map[string]any{"started_at": time.Now()},
    }
    
    session, err := service.CreateSession(ctx, createReq)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Created session: %s\n", session.ID)
    
    // Add an event
    event := &core.Event{
        ID:           "evt_001",
        InvocationID: "inv_001", 
        Author:       "user",
        Content: &core.Content{
            Role: "user",
            Parts: []core.Part{
                {Type: "text", Text: &[]string{"Hello!"}[0]},
            },
        },
        Actions: core.EventActions{
            StateDelta: map[string]any{"message_count": 1},
        },
        Timestamp: time.Now(),
    }
    
    err = service.AppendEvent(ctx, session, event)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Added event to session")
}
```

### File-based Persistence

```go
// Create file-based session service
config := &sessions.SessionConfiguration{
    MaxSessionsPerUser:  100,
    MaxEventsPerSession: 1000,
    SessionTTL:          24 * time.Hour,
    AutoCleanupInterval: time.Hour,
    PersistenceBackend:  "file",
    BackendConfig: map[string]any{
        "base_directory": "./sessions_data",
    },
}

service, err := sessions.NewFileSessionService("./sessions_data", config)
if err != nil {
    panic(err)
}
defer service.Close(ctx)
```

### Using the Session Builder

```go
// Build a session service with fluent API
service, err := sessions.NewSessionServiceBuilder().
    WithFileBackend("./sessions").
    WithMaxSessionsPerUser(50).
    WithMaxEventsPerSession(500).
    WithSessionTTL(12 * time.Hour).
    WithAutoCleanup(30 * time.Minute).
    WithEventHandlers().
    WithMetrics().
    Build()
if err != nil {
    panic(err)
}
defer service.Close(ctx)
```

## State Management

### Scoped State

The system supports four types of state scope:

- **Session State**: `key` - Specific to a single session
- **User State**: `user:key` - Shared across all sessions for a user  
- **App State**: `app:key` - Shared across all users of an application
- **Temp State**: `temp:key` - Not persisted, only available during processing

```go
stateManager := sessions.NewDefaultStateManager()

// Set different scoped state
err = stateManager.SetAppState(ctx, "my_app", "version", "1.0.0")
err = stateManager.SetUserState(ctx, "my_app", "user123", "timezone", "UTC")
err = stateManager.SetState(ctx, session, "current_step", "welcome")

// Access via scoped keys
version, exists, err := stateManager.GetState(ctx, session, "app:version")
timezone, exists, err := stateManager.GetState(ctx, session, "user:timezone")

// Get combined effective state
effectiveState, err := stateManager.GetEffectiveState(ctx, session)
```

### State Helpers

```go
helper := sessions.NewStateHelper(stateManager)

// Get with default value
theme, err := helper.GetOrDefault(ctx, session, "theme", "dark")

// Increment counters
count, err := helper.Increment(ctx, session, "message_count", 1)

// Toggle boolean flags
isActive, err := helper.Toggle(ctx, session, "notifications_enabled")

// Work with lists
err = helper.Push(ctx, session, "recent_topics", "AI Ethics")
lastTopic, err := helper.Pop(ctx, session, "recent_topics")
```

## Event Handlers

### Built-in Handlers

```go
// Logging handler
logger := &sessions.DefaultLogger{}
loggingHandler := sessions.NewLoggingEventHandler(logger)

// Metrics handler  
metrics := &sessions.DefaultMetricsCollector{}
metricsHandler := sessions.NewMetricsEventHandler(metrics)

// Validation handler
validationConfig := &sessions.ValidationConfig{
    MaxStateSize:   1024 * 1024, // 1MB
    MaxEventSize:   512 * 1024,  // 512KB
    AllowedAuthors: []string{"user", "assistant", "system"},
    ForbiddenKeys:  []string{"password", "secret"},
}
validationHandler := sessions.NewValidationEventHandler(validationConfig)

// Combine multiple handlers
compositeHandler := sessions.NewCompositeEventHandler(
    loggingHandler,
    metricsHandler, 
    validationHandler,
)

// Add to file session service
if fileService, ok := service.(*sessions.FileSessionService); ok {
    fileService.AddEventHandler(compositeHandler)
}
```

### Custom Event Handlers

```go
type CustomHandler struct{}

func (h *CustomHandler) OnSessionCreated(ctx context.Context, session *core.Session) error {
    // Custom logic for session creation
    fmt.Printf("Custom: Session %s created for user %s\n", session.ID, session.UserID)
    return nil
}

func (h *CustomHandler) OnSessionUpdated(ctx context.Context, session *core.Session, oldState map[string]any) error {
    // Custom logic for session updates
    return nil
}

func (h *CustomHandler) OnSessionDeleted(ctx context.Context, appName, userID, sessionID string) error {
    // Custom logic for session deletion
    return nil
}

func (h *CustomHandler) OnEventAdded(ctx context.Context, session *core.Session, event *core.Event) error {
    // Custom logic for event addition
    return nil
}
```

## Session Utilities

### Session Service Utils

```go
stateManager := sessions.NewDefaultStateManager() 
utils := sessions.NewSessionServiceUtils(service, stateManager)

// Create session with default values
defaults := map[string]any{
    "created_at": time.Now(),
    "theme": "dark",
    "tutorial_completed": false,
}
session, err := utils.CreateSessionWithDefaults(ctx, "my_app", "user123", defaults)

// Get existing or create new session  
session, err := utils.GetOrCreateSession(ctx, "my_app", "user123", "session456")

// Duplicate a session
newSession, err := utils.DuplicateSession(ctx, "my_app", "user123", "old_session", "new_session")

// Filter session events
filter := sessions.EventFilter{
    Authors:    []string{"user"},
    FromTime:   &yesterday,
    HasErrors:  &[]bool{false}[0],
    EventTypes: []string{"text", "function_call"},
}
events, err := utils.GetSessionHistory(ctx, "my_app", "user123", "session123", filter)

// Merge state from multiple sessions
err = utils.MergeSessionState(ctx, "my_app", "user123", "target_session", []string{"source1", "source2"})
```

### Backup and Restore

```go
backup := sessions.NewSessionBackupManager(service)

// Backup all sessions for a user
err = backup.BackupSessions(ctx, "my_app", "user123", "./backup.json")

// Restore sessions from backup
err = backup.RestoreSessions(ctx, "./backup.json", false) // false = don't overwrite existing
```

## Advanced Features

### Session Metadata

```go
// Get lightweight session metadata without loading full content
metadata, err := service.GetSessionMetadata(ctx, "my_app", "user123", "session456")
fmt.Printf("Session has %d events and %d state keys\n", 
    metadata.EventCount, len(metadata.StateKeys))
```

### Bulk Operations

```go
// Get all sessions for a user
sessions, err := service.GetSessionsByUser(ctx, "my_app", "user123")

// Get sessions modified after a specific time
recent, err := service.GetSessionsModifiedAfter(ctx, "my_app", "user123", yesterday)

// Bulk delete multiple sessions
sessionIDs := []string{"session1", "session2", "session3"}
err = service.BulkDeleteSessions(ctx, "my_app", "user123", sessionIDs)

// Clean up expired sessions
deleted, err := service.CleanupExpiredSessions(ctx, 24*time.Hour)
fmt.Printf("Cleaned up %d expired sessions\n", deleted)
```

### Session Configuration

```go
config := &sessions.SessionConfiguration{
    MaxSessionsPerUser:   100,           // Limit sessions per user
    MaxEventsPerSession:  1000,          // Limit events per session
    SessionTTL:           24 * time.Hour, // Session expiration time
    AutoCleanupInterval:  time.Hour,     // How often to run cleanup
    PersistenceBackend:   "file",        // "memory" or "file"
    BackendConfig: map[string]any{       // Backend-specific config
        "base_directory": "./sessions",
    },
    EnableEventHandlers: true,           // Enable lifecycle handlers
    EnableMetrics:       true,           // Enable metrics collection
}
```

## Error Handling

The session management system uses Go's standard error handling patterns:

```go
session, err := service.CreateSession(ctx, createReq)
if err != nil {
    // Handle creation error
    return fmt.Errorf("failed to create session: %w", err)
}

// Getting a non-existent session returns nil, not an error
session, err := service.GetSession(ctx, getReq)  
if err != nil {
    // Handle actual errors (I/O, validation, etc.)
    return err
}
if session == nil {
    // Session doesn't exist
    fmt.Println("Session not found")
}
```

## Performance Considerations

### Memory Management

- In-memory service stores all sessions in RAM - suitable for development/testing
- File-based service loads sessions on-demand - better for production
- Configure `MaxEventsPerSession` to prevent unbounded growth
- Use auto-cleanup to remove old sessions

### Concurrency 

- All session services are thread-safe
- Multiple goroutines can safely access the same service
- File operations use proper locking to prevent corruption
- Consider using read replicas for high-read workloads

### Storage

```go
// For file-based storage, organize by app/user hierarchy
./sessions/
├── my_app/
│   ├── user123/
│   │   ├── session1.json
│   │   └── session2.json
│   └── user456/
│       └── session3.json
└── state/
    ├── user_states.json
    └── app_states.json
```

## Testing

```go
// Run tests for all session implementations
go test -v ./pkg/sessions/

// Run specific test
go test -v ./pkg/sessions/ -run TestInMemorySessionService

// Run with race detection
go test -race ./pkg/sessions/

// Run example
go run ./examples/sessions/
```

## Integration with ADK Framework

The session management system integrates seamlessly with other ADK components:

```go
// In your agent implementation
func (a *MyAgent) RunAsync(ctx context.Context, invocationCtx *core.InvocationContext) (core.EventStream, error) {
    session := invocationCtx.Session
    
    // Access session state
    userPrefs, exists := session.State["user_preferences"]
    
    // Update state through event actions
    event := core.NewEvent(invocationCtx.InvocationID, a.Name())
    event.Actions.StateDelta = map[string]any{
        "last_interaction": time.Now(),
        "interaction_count": getCurrentCount(session) + 1,
    }
    
    return a.createEventStream(event), nil
}

// In your runner implementation  
func (r *Runner) RunAsync(ctx context.Context, req *core.RunRequest) (core.EventStream, error) {
    session, err := r.sessionService.GetSession(ctx, &core.GetSessionRequest{
        AppName:   r.appName,
        UserID:    req.UserID,
        SessionID: req.SessionID,
    })
    if err != nil {
        return nil, err
    }
    
    // Create invocation context with session
    invocationCtx := core.NewInvocationContext(
        generateInvocationID(),
        r.agent,
        session,
        r.sessionService,
    )
    
    return r.agent.RunAsync(ctx, invocationCtx)
}
```

## Best Practices

1. **Choose the Right Backend**: Use in-memory for testing, file-based for development, database for production
2. **Set Appropriate Limits**: Configure session and event limits to prevent resource exhaustion  
3. **Use Scoped State**: Leverage app/user/session scopes for proper data organization
4. **Handle Errors Gracefully**: Always check for errors and handle missing sessions appropriately
5. **Monitor Session Health**: Use event handlers to track metrics and detect issues
6. **Clean Up Regularly**: Enable auto-cleanup or implement custom cleanup logic
7. **Backup Important Data**: Use backup/restore for data protection and migration
8. **Test Concurrent Access**: Verify your usage patterns work under concurrent load

## Migration from Python ADK

The Go session management system closely mirrors the Python ADK architecture:

| Python ADK | Go ADK |
|------------|--------|
| `BaseSessionService` | `core.SessionService` |
| `InMemorySessionService` | `sessions.InMemorySessionService` |
| `Session` | `core.Session` |
| `Event` | `core.Event` |
| `EventActions` | `core.EventActions` |
| `state_delta` | `StateDelta` |
| Session callbacks | `SessionEventHandler` |

Key differences:
- Go uses explicit error handling instead of exceptions
- Go uses channels for event streaming instead of async generators
- Go provides builder patterns for fluent configuration
- Go includes additional utilities and helpers not in Python version
