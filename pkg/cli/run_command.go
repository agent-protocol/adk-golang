package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/agent-protocol/adk-golang/internal/core"
	"github.com/agent-protocol/adk-golang/pkg/cli/utils"
	"github.com/agent-protocol/adk-golang/pkg/runners"
	"github.com/agent-protocol/adk-golang/pkg/sessions"
)

// runCommand creates the 'run' command
func runCommand() *cli.Command {
	return &cli.Command{
		Name:      "run",
		Usage:     "Runs an interactive CLI for a specific agent",
		ArgsUsage: "AGENT_PATH",
		Flags: append(commonServiceFlags(), []cli.Flag{
			&cli.BoolFlag{
				Name:  "save-session",
				Usage: "Save the session to a JSON file on exit",
			},
			&cli.StringFlag{
				Name:  "session-id",
				Usage: "Session ID to save the session to on exit when --save-session is set",
			},
			&cli.StringFlag{
				Name:  "replay",
				Usage: "JSON file containing initial session state and user queries",
			},
			&cli.StringFlag{
				Name:  "resume",
				Usage: "JSON file containing a previously saved session to resume",
			},
		}...),
		Action: runCommandAction,
	}
}

func runCommandAction(c *cli.Context) error {
	agentPath := c.Args().First()
	if agentPath == "" {
		return fmt.Errorf("AGENT_PATH is required")
	}

	// Validate mutually exclusive options
	replay := c.String("replay")
	resume := c.String("resume")
	if replay != "" && resume != "" {
		return fmt.Errorf("--replay and --resume options cannot be used together")
	}

	// Get absolute path
	absAgentPath, err := filepath.Abs(agentPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if agent directory exists
	if _, err := os.Stat(absAgentPath); os.IsNotExist(err) {
		return fmt.Errorf("agent directory not found: %s", absAgentPath)
	}

	agentParentDir := filepath.Dir(absAgentPath)
	agentFolderName := filepath.Base(absAgentPath)

	fmt.Printf("Loading agent from: %s\n", absAgentPath)

	// Load the agent
	loader := utils.NewAgentLoader(agentParentDir)

	// Load .env file if present
	if err := loader.LoadDotEnv(agentFolderName); err != nil {
		return fmt.Errorf("failed to load .env: %w", err)
	}

	rootAgent, err := loader.LoadAgent(agentFolderName)
	if err != nil {
		return fmt.Errorf("failed to load agent: %w", err)
	}

	// Create services
	sessionService := sessions.NewInMemorySessionService()
	// TODO: Create artifact and credential services based on URIs

	// Create session
	ctx := context.Background()
	userID := "test_user"

	session, err := sessionService.CreateSession(ctx, &core.CreateSessionRequest{
		AppName: agentFolderName,
		UserID:  userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Handle replay mode
	if replay != "" {
		return runReplayMode(ctx, replay, rootAgent, sessionService, session)
	}

	// Handle resume mode
	if resume != "" {
		return runResumeMode(ctx, resume, rootAgent, sessionService, session)
	}

	// Interactive mode
	return runInteractiveMode(ctx, rootAgent, sessionService, session, c.Bool("save-session"), c.String("session-id"))
}

func runReplayMode(ctx context.Context, replayFile string, agent core.BaseAgent, sessionService core.SessionService, session *core.Session) error {
	// TODO: Implement replay mode
	// 1. Load replay file (JSON with state and queries)
	// 2. Apply initial state to session
	// 3. Run queries automatically
	// 4. Display results

	fmt.Printf("Replay mode not yet implemented. File: %s\n", replayFile)
	return nil
}

func runResumeMode(ctx context.Context, resumeFile string, agent core.BaseAgent, sessionService core.SessionService, session *core.Session) error {
	// TODO: Implement resume mode
	// 1. Load saved session from JSON file
	// 2. Restore session state and events
	// 3. Display previous conversation
	// 4. Continue interactively

	fmt.Printf("Resume mode not yet implemented. File: %s\n", resumeFile)
	return nil
}

func runInteractiveMode(ctx context.Context, rootAgent core.BaseAgent, sessionService core.SessionService, session *core.Session, saveSession bool, sessionID string) error {
	// Create runner
	runner := runners.NewRunner(session.AppName, rootAgent, sessionService)

	fmt.Printf("Running agent %s, type 'exit' to exit.\n", rootAgent.Name())
	fmt.Print("[user]: ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		query := strings.TrimSpace(scanner.Text())

		if query == "" {
			fmt.Print("[user]: ")
			continue
		}

		if query == "exit" {
			break
		}

		// Create user message
		userMessage := &core.Content{
			Role: "user",
			Parts: []core.Part{
				{Text: &query},
			},
		}

		// Run the agent
		runReq := &core.RunRequest{
			UserID:     session.UserID,
			SessionID:  session.ID,
			NewMessage: userMessage,
		}

		eventStream, err := runner.RunAsync(ctx, runReq)
		if err != nil {
			fmt.Printf("Error running agent: %v\n", err)
			fmt.Print("[user]: ")
			continue
		}

		// Process events
		for event := range eventStream {
			if event.Content != nil && len(event.Content.Parts) > 0 {
				var textParts []string
				for _, part := range event.Content.Parts {
					if part.Text != nil {
						textParts = append(textParts, *part.Text)
					}
				}
				if len(textParts) > 0 {
					text := strings.Join(textParts, "")
					if text != "" {
						fmt.Printf("[%s]: %s\n", event.Author, text)
					}
				}
			}
		}

		fmt.Print("[user]: ")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	// Save session if requested
	if saveSession {
		if err := saveSessionToFile(sessionService, session, sessionID); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}
	}

	return nil
}

func saveSessionToFile(sessionService core.SessionService, session *core.Session, sessionID string) error {
	// TODO: Implement session saving
	// 1. Get session ID from user if not provided
	// 2. Get full session details from service
	// 3. Save to JSON file

	if sessionID == "" {
		fmt.Print("Session ID to save: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			sessionID = strings.TrimSpace(scanner.Text())
		}
	}

	if sessionID == "" {
		return fmt.Errorf("session ID is required for saving")
	}

	fmt.Printf("Session saving not yet implemented. Would save to: %s.session.json\n", sessionID)
	return nil
}
