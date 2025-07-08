# ADK CLI Implementation

This document describes the CLI implementation for the Agent Development Kit (ADK) in Go, providing similar functionality to the Python version.

## Overview

The ADK CLI provides a comprehensive command-line interface for creating, running, evaluating, and deploying AI agents. It's built using the [urfave/cli](https://github.com/urfave/cli) library and follows the same patterns as the Python implementation.

## Commands

### Core Commands

- **`create`** - Create new agent projects with templates
- **`run`** - Run agents interactively or with automation
- **`web`** - Start web server with UI for agents
- **`api-server`** - Start HTTP API server for agents
- **`eval`** - Evaluate agents against test sets
- **`deploy`** - Deploy agents to cloud platforms

### Command Structure

```
adk [global-options] command [command-options] [arguments]
```

## Usage Examples

### Creating a New Agent

```bash
# Basic agent creation
adk create my-agent

# Agent with model configuration
adk create my-agent --model gemini-1.5-pro --api-key $GOOGLE_AI_API_KEY

# Agent with Google Cloud settings
adk create my-agent --model gemini-1.5-pro --project my-project --region us-central1
```

### Running Agents

```bash
# Interactive mode
adk run ./my-agent

# With session management
adk run ./my-agent --save-session --session-id my-session

# Replay from file
adk run ./my-agent --replay session-replay.json

# Resume previous session
adk run ./my-agent --resume my-session.json
```

### Web Server

```bash
# Start web server for agents directory
adk web ./agents --port 8080

# With additional CORS origins
adk web ./agents --allow-origins http://localhost:3000,http://myapp.com

# With A2A protocol support
adk web ./agents --a2a
```

### API Server

```bash
# Start API server
adk api-server ./agents --port 8000

# With custom services
adk api-server ./agents --session-service-uri sqlite://./sessions.db
```

### Evaluation

```bash
# Evaluate agent against test sets
adk eval ./my-agent eval-set-1.json eval-set-2.json

# With detailed results
adk eval ./my-agent eval-set.json --print-detailed-results

# With custom configuration
adk eval ./my-agent eval-set.json --config-file eval-config.yaml
```

### Deployment

```bash
# Deploy to Cloud Run
adk deploy cloud-run ./my-agent --project my-project --region us-central1

# Deploy to Agent Engine
adk deploy agent-engine ./my-agent --project my-project --region us-central1 --staging-bucket my-bucket
```

## Agent Discovery

The CLI includes a sophisticated agent discovery system that supports multiple agent formats:

### Supported Agent Formats

1. **Go Plugins** (`.so` files)
   ```
   my-agent/
   ├── agent.so     # Compiled plugin with RootAgent symbol
   └── .env         # Optional environment variables
   ```

2. **YAML Configuration**
   ```
   my-agent/
   ├── agent.yml    # Declarative agent configuration
   └── .env
   ```

3. **Executable Agents**
   ```
   my-agent/
   ├── main         # Compiled executable
   └── .env
   ```

4. **Go Source**
   ```
   my-agent/
   ├── agent.go     # Go source with RootAgent variable
   ├── go.mod
   └── .env
   ```

### Agent Directory Structure

The agent loader follows the Python ADK pattern:

```
agents/
├── agent-1/
│   ├── agent.go
│   ├── .env
│   └── README.md
├── agent-2/
│   ├── agent.yml
│   └── .env
└── agent-3/
    ├── main
    └── .env
```

## Configuration

### Global Options

- `--verbose` - Enable verbose logging
- `--help` - Show help information
- `--version` - Show version information

### Service Configuration

All commands that need services support these options:

- `--session-service-uri` - Session storage URI
- `--artifact-service-uri` - Artifact storage URI  
- `--memory-service-uri` - Memory service URI
- `--eval-storage-uri` - Evaluation storage URI

#### Supported URIs

- **Session Service**: `sqlite://path/to/db.sqlite`, `agentengine://resource_id`
- **Artifact Service**: `gs://bucket-name`
- **Memory Service**: `rag://corpus_id`, `agentengine://resource_id`
- **Eval Storage**: `gs://bucket-name`

### Web Server Options

- `--host` - Bind host (default: 127.0.0.1)
- `--port` - Bind port (default: 8000)
- `--allow-origins` - CORS origins
- `--log-level` - Logging level
- `--trace-to-cloud` - Enable cloud tracing
- `--reload` - Enable auto-reload
- `--a2a` - Enable A2A protocol endpoint

## Implementation Details

### Project Structure

```
pkg/cli/
├── app.go                 # Main CLI application setup
├── run_command.go         # Agent execution command
├── create_command.go      # Agent creation command
├── web_command.go         # Web server command
├── api_server_command.go  # API server command
├── eval_command.go        # Evaluation command
├── deploy_command.go      # Deployment commands
└── utils/
    └── agent_loader.go    # Agent discovery and loading
```

### Key Components

1. **Agent Loader** (`utils/agent_loader.go`)
   - Multi-format agent discovery
   - Caching for performance
   - Environment variable loading
   - Validation and error handling

2. **Command Structure**
   - Consistent flag patterns across commands
   - Proper validation and error handling
   - Help text and usage examples

3. **Service Integration**
   - Session management
   - Artifact storage
   - Memory services
   - Credential management

## Building and Installation

### Build from Source

```bash
# Build CLI binary
go build -o bin/adk ./cmd/adk

# Install to system PATH
go install ./cmd/adk
```

### Development Build

```bash
# Build with debug information
go build -ldflags "-X main.Version=dev -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/adk ./cmd/adk
```

## Agent Template

When creating agents with `adk create`, the following template structure is generated:

### Generated Files

1. **`agent.go`** - Main agent implementation
2. **`go.mod`** - Go module definition
3. **`.env`** - Environment variables (if provided)
4. **`README.md`** - Agent documentation

### Agent Template Example

```go
package main

import (
    "context"
    "log"
    
    "github.com/agent-protocol/adk-golang/internal/core"
    "github.com/agent-protocol/adk-golang/pkg/agents"
)

// RootAgent is the main agent that will be loaded by the CLI
var RootAgent core.BaseAgent

func init() {
    // Initialize your agent here
    RootAgent = agents.NewLLMAgent(&agents.LLMAgentConfig{
        Name:        "my-agent",
        Description: "A sample agent",
        Model:       "gemini-1.5-pro",
        Instruction: "You are a helpful assistant.",
    })
}
```

## Error Handling

The CLI provides comprehensive error handling:

- **Validation Errors** - Missing required arguments, invalid paths
- **Agent Loading Errors** - Invalid agent structure, missing files
- **Service Errors** - Connection failures, authentication issues
- **Runtime Errors** - Agent execution failures, timeouts

## Session Management

The CLI supports sophisticated session management:

### Session Operations

- **Create** - New sessions for agent interactions
- **Save** - Persist sessions to JSON files
- **Resume** - Continue from saved sessions
- **Replay** - Automated execution from session files

### Session Files

Sessions are saved in JSON format compatible with the Python implementation:

```json
{
  "id": "session-123",
  "app_name": "my-agent",
  "user_id": "test_user",
  "state": {},
  "events": [],
  "last_update_time": "2024-01-01T00:00:00Z"
}
```

## Future Enhancements

### Planned Features

1. **Web UI Implementation** - Complete web interface
2. **API Server Implementation** - Full REST API
3. **Evaluation Framework** - Complete evaluation system
4. **Deployment Automation** - Full deployment pipeline
5. **Plugin System** - Custom command extensions
6. **Configuration Management** - Advanced configuration options

### Extension Points

The CLI is designed to be extensible:

- Custom agent loaders
- Additional deployment targets
- Custom evaluation metrics
- Plugin-based commands

## Compatibility

The CLI maintains compatibility with:

- **Python ADK** - Session files, agent structure
- **A2A Protocol** - Agent-to-Agent communication
- **Google Cloud** - VertexAI, Cloud Run, Agent Engine
- **Standard Formats** - JSON, YAML, environment files

## Testing

### Unit Tests

```bash
# Run CLI tests
go test ./pkg/cli/...

# Run with coverage
go test -cover ./pkg/cli/...
```

### Integration Tests

```bash
# Test agent creation
adk create test-agent
adk run test-agent --help

# Test command validation
adk run --help
adk web --help
```

## Contributing

When contributing to the CLI:

1. Follow Go conventions and idioms
2. Maintain compatibility with Python ADK
3. Add comprehensive error handling
4. Include help text and examples
5. Test all command variations

## See Also

- [Python ADK CLI Documentation](https://python-adk-docs.com/cli/)
- [urfave/cli Documentation](https://cli.urfave.org/)
- [A2A Protocol Specification](https://a2a-protocol.com/)
- [ADK Core Types](../internal/core/README.md)
