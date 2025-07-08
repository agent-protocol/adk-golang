package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

// evalCommand creates the 'eval' command
func evalCommand() *cli.Command {
	flags := append(commonServiceFlags(), []cli.Flag{
		&cli.StringFlag{
			Name:  "config-file",
			Usage: "Path to evaluation configuration file",
		},
		&cli.BoolFlag{
			Name:  "print-detailed-results",
			Usage: "Print detailed results to console",
		},
	}...)

	return &cli.Command{
		Name:      "eval",
		Usage:     "Evaluates an agent against evaluation sets",
		ArgsUsage: "AGENT_PATH EVAL_SET_FILES...",
		Flags:     flags,
		Action:    evalCommandAction,
	}
}

func evalCommandAction(c *cli.Context) error {
	args := c.Args().Slice()
	if len(args) < 2 {
		return fmt.Errorf("AGENT_PATH and at least one EVAL_SET_FILE are required")
	}

	agentPath := args[0]
	evalSetFiles := args[1:]

	// Get absolute path
	absAgentPath, err := filepath.Abs(agentPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if agent directory exists
	if _, err := os.Stat(absAgentPath); os.IsNotExist(err) {
		return fmt.Errorf("agent directory not found: %s", absAgentPath)
	}

	configFile := c.String("config-file")
	printDetailed := c.Bool("print-detailed-results")
	evalStorageURI := c.String("eval-storage-uri")

	fmt.Printf("Evaluating agent: %s\n", absAgentPath)
	fmt.Printf("Evaluation sets: %v\n", evalSetFiles)

	if configFile != "" {
		fmt.Printf("Config file: %s\n", configFile)
	}
	if evalStorageURI != "" {
		fmt.Printf("Eval storage: %s\n", evalStorageURI)
	}

	// Validate eval set files exist
	for _, evalFile := range evalSetFiles {
		if _, err := os.Stat(evalFile); os.IsNotExist(err) {
			return fmt.Errorf("eval set file not found: %s", evalFile)
		}
	}

	// TODO: Implement evaluation logic
	// This would involve:
	// 1. Loading the agent
	// 2. Loading evaluation sets and configurations
	// 3. Running the agent against each eval case
	// 4. Collecting metrics and results
	// 5. Generating reports

	fmt.Printf("Evaluation implementation not yet complete.\n")
	fmt.Printf("Configuration would be:\n")
	fmt.Printf("  Agent: %s\n", absAgentPath)
	fmt.Printf("  Eval sets: %v\n", evalSetFiles)
	fmt.Printf("  Print detailed: %v\n", printDetailed)

	return fmt.Errorf("eval command not yet implemented")
}
