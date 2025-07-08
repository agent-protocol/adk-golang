ADK (Agent Development Kit) is a framework for building AI agents, and we're implement ADK with Golang.
Their is 2 main parts:
- Agent2Agent (A2A) protocol. Please read the docs here https://a2aproject.github.io/A2A/latest/specification/ 
- ADK implementation: https://google.github.io/adk-docs/
  - Python API https://google.github.io/adk-docs/api-reference/python/
  - Java API https://google.github.io/adk-docs/api-reference/java/

And we're working to implement an Golang API with Core Architecture:
- Agents: BaseAgent, LlmAgent, RemoteA2aAgent
- Tools: BaseTool, FunctionTool, AgentTool
- Runner: Runner - orchestrates agent execution
- Sessions: Session management and state persistence
- Events: Communication units between agents
- A2A Integration: A2aAgentExecutor

Key Design Considerations for Go
- Concurrency: Use goroutines and channels instead of Python's asyncio
- Error Handling: Explicit error returns instead of exceptions
- Context: Use context.Context for cancellation and timeouts
- Interfaces: Define small, focused interfaces following Go idioms. Using interface or generic where appropriates.
- JSON: Proper struct tags for JSON marshaling/unmarshaling
- Modular: use more Go native approach when come to multi-agent projects

```
adk-golang/
├── cmd/
│   └── adk/              # CLI application
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