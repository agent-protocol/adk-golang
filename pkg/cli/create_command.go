package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

// createCommand creates the 'create' command
func createCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Creates a new agent project with a template",
		ArgsUsage: "APP_NAME",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "model",
				Usage: "Model to use for the root agent (e.g., 'gemini-1.5-pro')",
			},
			&cli.StringFlag{
				Name:  "api-key",
				Usage: "API key for the model (e.g., Google AI API Key)",
			},
			&cli.StringFlag{
				Name:  "project",
				Usage: "Google Cloud project for VertexAI backend",
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: "Google Cloud region for VertexAI backend",
			},
		},
		Action: createCommandAction,
	}
}

func createCommandAction(c *cli.Context) error {
	appName := c.Args().First()
	if appName == "" {
		return fmt.Errorf("APP_NAME is required")
	}

	model := c.String("model")
	apiKey := c.String("api-key")
	project := c.String("project")
	region := c.String("region")

	// Create the agent directory
	agentPath := filepath.Join(".", appName)
	if err := os.MkdirAll(agentPath, 0755); err != nil {
		return fmt.Errorf("failed to create agent directory: %w", err)
	}

	// Create basic agent structure
	if err := createAgentTemplate(agentPath, appName, model, apiKey, project, region); err != nil {
		return fmt.Errorf("failed to create agent template: %w", err)
	}

	fmt.Printf("Successfully created agent '%s' in %s\n", appName, agentPath)
	fmt.Printf("To run your agent:\n")
	fmt.Printf("  adk run %s\n", agentPath)

	return nil
}

func createAgentTemplate(agentPath, appName, model, apiKey, project, region string) error {
	// Create agent.go file
	agentContent := fmt.Sprintf(`package main

import (
	"context"
	"log"

	"github.com/agent-protocol/adk-golang/pkg/core"
	"github.com/agent-protocol/adk-golang/pkg/agents"
)

// RootAgent is the main agent that will be loaded by the CLI
var RootAgent core.BaseAgent

func init() {
	// TODO: Initialize your agent here
	// Example:
	// RootAgent = agents.NewLLMAgent(&agents.LLMAgentConfig{
	//     Name:        "%s",
	//     Description: "A sample agent",
	//     Model:       "%s",
	//     Instruction: "You are a helpful assistant.",
	// })
	
	log.Printf("Agent %s initialized", "%s")
}

func main() {
	// This main function is used when running the agent as a standalone executable
	ctx := context.Background()
	
	if RootAgent == nil {
		log.Fatal("RootAgent not initialized")
	}
	
	log.Printf("Starting agent %s", RootAgent.Name())
	// TODO: Add your agent execution logic here
}`, appName, model, appName, model, appName)

	if err := os.WriteFile(filepath.Join(agentPath, "agent.go"), []byte(agentContent), 0644); err != nil {
		return fmt.Errorf("failed to create agent.go: %w", err)
	}

	// Create .env file if API key is provided
	if apiKey != "" || project != "" || region != "" {
		envContent := ""
		if apiKey != "" {
			envContent += fmt.Sprintf("GOOGLE_AI_API_KEY=%s\n", apiKey)
		}
		if project != "" {
			envContent += fmt.Sprintf("GOOGLE_CLOUD_PROJECT=%s\n", project)
		}
		if region != "" {
			envContent += fmt.Sprintf("GOOGLE_CLOUD_LOCATION=%s\n", region)
		}

		if err := os.WriteFile(filepath.Join(agentPath, ".env"), []byte(envContent), 0644); err != nil {
			return fmt.Errorf("failed to create .env: %w", err)
		}
	}

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s Agent

This agent was created using ADK (Agent Development Kit) for Go.

## Structure

- `+"`agent.go`"+` - Main agent implementation
- `+"`.env`"+` - Environment variables (optional)
- `+"`README.md`"+` - This file

## Usage

To run this agent:

`+"```bash"+`
adk run .
`+"```"+`

To run in web mode:

`+"```bash"+`
adk web .
`+"```"+`

## Configuration

Configure your agent by editing `+"`agent.go`"+` and setting up the RootAgent variable.

## Environment Variables

- `+"`GOOGLE_AI_API_KEY`"+` - Google AI API key for Gemini models
- `+"`GOOGLE_CLOUD_PROJECT`"+` - Google Cloud project for VertexAI
- `+"`GOOGLE_CLOUD_LOCATION`"+` - Google Cloud region for VertexAI
`, appName)

	if err := os.WriteFile(filepath.Join(agentPath, "README.md"), []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	// Create go.mod file
	goModContent := fmt.Sprintf(`module %s

go 1.24

require (
    github.com/agent-protocol/adk-golang v0.1.0
)

// Use local development version
replace github.com/agent-protocol/adk-golang => ../adk-golang
`, appName)

	if err := os.WriteFile(filepath.Join(agentPath, "go.mod"), []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	return nil
}
