ADK (Agent Development Kit) is a framework for building AI agents, and we're implement ADK with Golang.
Their is 2 main parts:
- Agent2Agent (A2A) protocol. Please read the docs here https://a2aproject.github.io/A2A/latest/specification/ 
- ADK implementation: https://google.github.io/adk-docs/
  - Python API https://google.github.io/adk-docs/api-reference/python/
  - Java API https://google.github.io/adk-docs/api-reference/java/

And we're working to implement an Golang API.
# Core Architecture:
- Agents: BaseAgent, LlmAgent, RemoteA2aAgent
- Tools: BaseTool, FunctionTool, AgentTool
- Runner: Runner - orchestrates agent execution
- Sessions: Session management and state persistence
- Events: Communication units between agents
- A2A Integration: A2aAgentExecutor

# Key Design Considerations for Go
- Concurrency: Use goroutines and channels instead of Python's asyncio. Aware of race conditions and deadlocks.
- Error Handling: Explicit error returns instead of exceptions
- Context: Use context.Context for cancellation and timeouts
- Interfaces: Define small, focused interfaces following Go idioms. Using interface or generic where appropriates.
- JSON: Proper struct tags for JSON marshaling/unmarshaling
- Modular: use more Go native approach when come to multi-agent projects

# Success Metrics
- Can create and run basic agents
- Tool system works with function calling
- A2A protocol integration functional
- CLI commands operational
- HTTP API server running
- Multi-agent workflows supported
- Compatible with existing A2A agents

# Development Instructions:
- Follow SOLID principles
- ALWAYS check adk-python implementation for reference
- Follow Go idioms and best practices
- Use `go fmt` for formatting
- Use `go vet` and static analysis tools
- Write unit tests for all components
- Use interfaces for extensibility
- Always try to use context.Context for cancellation and timeouts
- Try to test versus adk-python where possible
- Make sure the code is compatible and can be integrated with existing codebase
- Review all old examples and tests to ensure they are up-to-date
- Run `go install ./...` and `go test ./...` to build and test the project after making changes
- use `ptr.Float32` creating pointers to float32 values, e.g. `ptr.Float32(0.7)`
- Use `ptr.Ptr` creating pointers to values for other types, e.g. `ptr.Ptr(true)` or `ptr.Ptr(2000)`

# Project Structure:
```
adk-golang/
├── cmd/
│   └── adk/              # CLI application
├── docs/                 # Documentation files, all development notes
├── pkg/
│   ├── agents/           # Agent implementations
│   ├── tools/            # Tool system
│   ├── events/           # Event system
│   ├── sessions/         # Session management
│   ├── a2a/              # A2A protocol implementation
│   └── api/              # HTTP API server
├── internal/
│   ├── core/             # Core types and interfaces
│   ├── llm/              # LLM integrations
│   └── utils/            # Utilities
├── examples/             # Example agents and usage
└── tests/                # Test suites
```

# Good read:
## Message or Task?
- https://a2aproject.github.io/A2A/latest/topics/life-of-a-task/
