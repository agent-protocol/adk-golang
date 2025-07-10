// Package main demonstrates how to use SequentialAgent to create conversation loops
// between multiple LLM agents, similar to student-teacher interactions.
//
// This example shows:
// - Creating two LLM agents with different personalities/roles using Ollama
// - Setting up a SequentialAgent to manage their conversation
// - Running multiple rounds of conversation
// - Proper A2A protocol integration for multi-agent workflows
package main

import (
	"os"
	"time"

	"github.com/agent-protocol/adk-golang/pkg/agents"
	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/llmconnect/ollama"
	"github.com/agent-protocol/adk-golang/pkg/ptr"
)

var RootAgent core.BaseAgent

func init() {
	// Get model name from environment or use default
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.2"
	}

	// Create Ollama configuration
	ollamaConfig := &ollama.OllamaConfig{
		BaseURL:     "http://localhost:11434",
		Model:       modelName,
		Temperature: ptr.Float32(0.7),
		Timeout:     30 * time.Second,
		Stream:      false,
	}

	// Allow environment variable overrides
	if baseURL := os.Getenv("OLLAMA_API_BASE"); baseURL != "" {
		ollamaConfig.BaseURL = baseURL
	}

	// Create Ollama connection
	ollamaConnection := ollama.NewOllamaConnection(ollamaConfig)

	// Create empiricist philosopher
	empiricistConfig := agents.DefaultLlmAgentConfig()
	empiricistConfig.Model = modelName
	empiricistConfig.Temperature = ptr.Float32(0.3)
	empiricist := agents.NewLLMAgent("Empiricist", "Philosopher advocating empiricism", empiricistConfig)
	empiricist.SetInstruction(`You are a philosopher who strongly believes in empiricism - 
		that knowledge comes primarily from sensory experience and observation. 
		Present logical arguments for your position, engage thoughtfully with opposing views, 
		and use historical examples from philosophers like Hume and Locke.`)
	empiricist.SetLLMConnection(ollamaConnection)

	// Create rationalist philosopher
	rationalistConfig := agents.DefaultLlmAgentConfig()
	rationalistConfig.Model = modelName
	rationalistConfig.Temperature = ptr.Float32(0.3)
	rationalist := agents.NewLLMAgent("Rationalist", "Philosopher advocating rationalism", rationalistConfig)
	rationalist.SetInstruction(`You are a philosopher who strongly believes in rationalism - 
		that reason and logic are the primary sources of knowledge. 
		Present logical arguments for your position, engage thoughtfully with opposing views, 
		and use historical examples from philosophers like Descartes and Leibniz.`)
	rationalist.SetLLMConnection(ollamaConnection)

	// Create debate with custom configuration
	debateConfig := &agents.SequentialAgentConfig{
		MaxRounds:           4, // 4 rounds of back-and-forth
		StopOnError:         true,
		PassCompleteHistory: true, // Important for debate context
		AddTurnMarkers:      true,
	}

	debate := agents.NewSequentialAgentWithConfig(
		"PhilosophicalDebate",
		"Structured debate between empiricist and rationalist philosophers",
		[]core.BaseAgent{empiricist, rationalist},
		debateConfig,
	)

	RootAgent = debate
}
