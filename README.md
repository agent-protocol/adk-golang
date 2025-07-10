# WARNING: still under development
# ADK for Go

Agent Development Kit (ADK) is a framework for building AI agents, and we're implementing ADK with Go. This project provides Go interfaces and implementations that mirror the Python ADK's architecture while following Go idioms and best practices.

# Quick Start:
```sh
ollama serve # the example using llama3.2
git clone https://github.com/agent-protocol/adk-golang.git
cd adk-golang
go run ./cmd/adk web examples/agents
```
Open the browser, select one of the demo app and check it.

## Overview

ADK consists of two main parts:
- **Agent2Agent (A2A) protocol**: Inter-agent communication protocol. See the [specification](https://a2aproject.github.io/A2A/latest/specification/)
- **ADK implementation**: The core framework for building agents. See [ADK documentation](https://google.github.io/adk-docs/)
  - [Python API](https://google.github.io/adk-docs/api-reference/python/)
  - [Java API](https://google.github.io/adk-docs/api-reference/java/)

The goal is to be compatible with the other ADK ecosystem by using A2A protocol.
The ADK API maybe somewhat different with Python and Java implementation, I mean... this is Golang, why do you need `ParallelAgent` since `ConcurrentAgent` is far superior :joy: 

### Core Components
- **Agents**:
  - [x] `CustomAgent`
  - [x] `LLMAgent`
  - [ ] `RemoteA2aAgent`
  - [x] `SequentialAgent`
  - [ ] `ConcurrentAgent`
- **Tools**:
  - [x] `FunctionTool`
  - [ ] `AgentTool`
  - [ ] `MCPTool`
- **Runner**: Orchestrates agent execution with real-time event streaming
- **Sessions**: Advanced session management with scoped state and persistence
- **Events**: Communication units between agents with streaming support
- **A2A Integration**: Complete A2A protocol implementation for remote agents
- **CLI**: Comprehensive command-line interface for all operations
- **API Server**: HTTP API with Web UI for testing and production deployment

## Project Structure

```
adk-golang/
├── cmd/
│   └── adk/              # CLI application with create, run, web, eval commands
├── pkg/
│   ├── agents/           # Agent implementations (Base, LLM, Sequential)
│   ├── tools/            # Tool system (Function, Google Search, Agent tools)
│   ├── sessions/         # Session management (Memory, File, State, Handlers)
│   ├── runners/          # Execution orchestration with event streaming
│   ├── a2a/              # A2A protocol implementation and converters
│   ├── api/              # HTTP API server and Web UI
│   └── cli/              # CLI command implementations
├── internal/
│   ├── core/             # Core types and interfaces
│   ├── llm/              # LLM integrations (interface ready)
│   └── utils/            # Utilities and helpers
├── examples/             # Comprehensive examples and demos
└── docs/                 # Detailed implementation documentation
```
