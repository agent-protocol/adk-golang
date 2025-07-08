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
- Follow Go idioms and best practices
- Use gofmt for formatting
- Use go vet and static analysis tools
- Write unit tests for all components
- Use interfaces for extensibility
- Always try to use context.Context for cancellation and timeouts
- Try to test versus adk-python where possible
- Make sure the code is compatible and can be integrates with existing codebase
- Review all old examples and tests to ensure they are up-to-date
- Run `go install ./...` and `go test ./...` to build and test the project after making changes

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

# ADK Python Implementation Summary for adk-golang Reference

Based on my analysis of the adk-python codebase, here's a comprehensive summary of the core components and architecture that should guide the adk-golang implementation:

## 1. Core Architecture Overview

ADK follows a **event-driven, multi-agent architecture** with these key principles:
- **Code-first**: Everything defined in code (not GUI-based)
- **Modularity**: Complex systems built by composing smaller, specialized agents
- **Deployment-agnostic**: Same agent logic runs locally, via API, or in cloud
- **Event-based communication**: All interactions flow through structured Events

## 2. Core Components

### 2.1 Event System

**Event Structure** (event.py):
```python
class Event(LlmResponse):
    invocation_id: str  # Links all events in single user interaction
    author: str         # 'user' or agent name
    content: Optional[types.Content]  # Message content (text, function_call, etc.)
    actions: EventActions             # Side effects and control flow
    branch: Optional[str]            # Agent hierarchy path (agent1.agent2.agent3)
    id: str                          # Unique event identifier
    timestamp: float                 # Event creation time
    long_running_tool_ids: Optional[set[str]]  # IDs of async tools
    custom_metadata: Optional[dict]   # Additional metadata
```

**EventActions Structure** (event_actions.py):
```python
class EventActions(BaseModel):
    skip_summarization: Optional[bool] = None
    state_delta: dict[str, object] = {}      # Session state updates
    artifact_delta: dict[str, int] = {}     # Artifact version updates
    transfer_to_agent: Optional[str] = None  # Agent handoff
    escalate: Optional[bool] = None          # Escalation signal
    requested_auth_configs: dict[str, AuthConfig] = {}  # Auth requests
```

### 2.2 Session Management

**Session Structure** (session.py):
```python
class Session(BaseModel):
    id: str                           # Unique session identifier
    app_name: str                     # Application name
    user_id: str                      # User identifier  
    state: dict[str, Any] = {}        # Session state data
    events: list[Event] = []          # Conversation history
    last_update_time: float = 0.0     # Last modification timestamp
```

**Session Service Interface** (base_session_service.py):
```python
class BaseSessionService(ABC):
    async def create_session(*, app_name: str, user_id: str, 
                           state: Optional[dict] = None, 
                           session_id: Optional[str] = None) -> Session
    async def get_session(*, app_name: str, user_id: str, session_id: str,
                         config: Optional[GetSessionConfig] = None) -> Optional[Session]
    async def append_event(session: Session, event: Event) -> Event
    async def delete_session(*, app_name: str, user_id: str, session_id: str) -> None
    async def list_sessions(*, app_name: str, user_id: str) -> ListSessionsResponse
```

**State Management**:
- **App State**: `app:key` - shared across all users of an application
- **User State**: `user:key` - shared across all sessions for a user
- **Session State**: `key` - specific to a single session
- **Temp State**: `temp:key` - not persisted, only available during processing

### 2.3 Agent System

**Base Agent Interface** (base_agent.py):
```python
class BaseAgent(BaseModel):
    name: str                        # Agent identifier
    description: str                 # Agent purpose description
    instruction: Optional[str]       # System instructions
    sub_agents: list[BaseAgent] = [] # Child agents in hierarchy
    parent_agent: Optional[BaseAgent] = None  # Parent reference
    
    # Lifecycle callbacks
    before_agent_callback: Optional[BeforeAgentCallback] = None
    after_agent_callback: Optional[AfterAgentCallback] = None
    
    async def run_async(ctx: InvocationContext) -> AsyncGenerator[Event, None]
    def find_agent(name: str) -> Optional[BaseAgent]
    def find_sub_agent(name: str) -> Optional[BaseAgent]
```

**LLM Agent** (llm_agent.py):
```python
class LlmAgent(BaseAgent):
    model: str                       # LLM model name
    tools: list[BaseTool] = []      # Available tools
    input_schema: Optional[BaseModel] = None   # Input validation
    output_schema: Optional[BaseModel] = None  # Output validation
    
    # Additional callbacks
    before_model_callback: Optional[BeforeModelCallback] = None
    after_model_callback: Optional[AfterModelCallback] = None
    before_tool_callback: Optional[BeforeToolCallback] = None
    after_tool_callback: Optional[AfterToolCallback] = None
```

**Agent Types**:
- **LlmAgent**: Uses language models for reasoning
- **SequentialAgent**: Executes sub-agents in sequence
- **ParallelAgent**: Executes sub-agents concurrently
- **LoopAgent**: Repeats sub-agent execution with conditions
- **RemoteA2aAgent**: Communicates with remote A2A protocol agents

### 2.4 Tool System

**Base Tool Interface** (base_tool.py):
```python
class BaseTool(ABC):
    name: str                        # Tool identifier
    description: str                 # Tool purpose
    is_long_running: bool = False   # Async operation flag
    
    async def run_async(*, args: dict[str, Any], 
                       tool_context: ToolContext) -> Any
    def _get_declaration() -> Optional[FunctionDeclaration]
    async def process_llm_request(*, tool_context: ToolContext, 
                                 llm_request: LlmRequest) -> None
```

**Function Tool** (function_tool.py):
```python
class FunctionTool(BaseTool):
    func: Callable  # Python function to wrap
    
    # Automatically handles:
    # - Function signature inspection
    # - Argument validation
    # - Async/sync function support
    # - tool_context injection
```

**Tool Context** (tool_context.py):
```python
class ToolContext:
    invocation_context: InvocationContext  # Current execution context
    state: State                          # Session state access
    actions: EventActions                 # Event actions to apply
    function_call_id: Optional[str]       # Function call identifier
    
    # Artifact management methods
    async def save_artifact(filename: str, content: bytes, mime_type: str) -> int
    async def load_artifact(filename: str, version: Optional[int] = None) -> Optional[bytes]
```

### 2.5 Runner (Orchestration Engine)

**Runner Structure** (runners.py):
```python
class Runner:
    app_name: str
    agent: BaseAgent                 # Root agent
    session_service: BaseSessionService
    artifact_service: Optional[BaseArtifactService]
    memory_service: Optional[BaseMemoryService]
    credential_service: Optional[BaseCredentialService]
    
    async def run_async(*, user_id: str, session_id: str, 
                       new_message: types.Content,
                       run_config: RunConfig = RunConfig()) -> AsyncGenerator[Event, None]
    
    def run(*, user_id: str, session_id: str,
           new_message: types.Content) -> Generator[Event, None, None]  # Sync wrapper
```

**Execution Flow**:
1. Load/create session
2. Append user message to session
3. Determine which agent should handle the request
4. Create InvocationContext
5. Execute agent.run_async()
6. Process events (apply state changes, save artifacts)
7. Yield events to caller

### 2.6 Invocation Context

**InvocationContext Structure**:
```python
class InvocationContext:
    invocation_id: str               # Unique invocation identifier
    agent: BaseAgent                 # Current executing agent
    session: Session                 # Session state
    session_service: BaseSessionService
    artifact_service: Optional[BaseArtifactService]
    memory_service: Optional[BaseMemoryService]
    credential_service: Optional[BaseCredentialService]
    user_content: Optional[types.Content]  # User input
    branch: Optional[str]            # Agent hierarchy path
    run_config: RunConfig            # Execution configuration
```

## 3. Key Design Patterns

### 3.1 Async Event Streaming
- All agent execution returns `AsyncGenerator[Event, None]`
- Events yielded in real-time for responsive UIs
- Events processed by Runner and appended to sessions

### 3.2 Tool Integration
- Tools wrapped as `FunctionTool` with automatic signature inspection
- `ToolContext` provides access to session state and artifacts
- Tools can modify session state via `tool_context.actions.state_delta`
- Long-running tools supported with async response handling

### 3.3 Agent Composition
- Agents contain `sub_agents` list for hierarchical organization
- Agent transfer via `event.actions.transfer_to_agent`
- Branch tracking enables isolated conversations between agent groups

### 3.4 State Management
- Three-tier state system (app/user/session)
- State changes applied via `EventActions.state_delta`
- Immediate availability in context, persisted after event processing

## 4. Service Interfaces

### 4.1 Artifact Service
```python
class BaseArtifactService(ABC):
    async def save_artifact(*, app_name: str, user_id: str, session_id: str,
                           filename: str, artifact: types.Part) -> int
    async def load_artifact(*, app_name: str, user_id: str, session_id: str,
                           filename: str, version: Optional[int] = None) -> Optional[types.Part]
    async def list_artifact_keys(*, app_name: str, user_id: str, session_id: str) -> list[str]
    async def delete_artifact(*, app_name: str, user_id: str, session_id: str, filename: str) -> None
    async def list_versions(*, app_name: str, user_id: str, session_id: str, filename: str) -> list[int]
```

### 4.2 Memory Service
```python
class BaseMemoryService(ABC):
    async def add_session_to_memory(session: Session) -> None
    async def retrieve_relevant_events(*, app_name: str, user_id: str,
                                      query: str, limit: int = 10) -> list[Event]
```
