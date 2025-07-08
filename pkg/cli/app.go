package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// Version information - will be set during build
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// NewApp creates and configures the CLI application
func NewApp() *cli.App {
	app := &cli.App{
		Name:    "adk",
		Usage:   "Agent Development Kit CLI tools",
		Version: Version,
		Commands: []*cli.Command{
			createCommand(),
			runCommand(),
			webCommand(),
			apiServerCommand(),
			evalCommand(),
			deployCommand(),
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose logging",
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("verbose") {
				// Set verbose logging
				os.Setenv("ADK_LOG_LEVEL", "DEBUG")
			}
			return nil
		},
	}

	// Custom help template
	cli.AppHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
   {{if len .Authors}}
AUTHOR:
   {{range .Authors}}{{ . }}{{end}}
   {{end}}{{if .Commands}}
COMMANDS:
{{range .Commands}}{{if not .HideHelp}}   {{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}{{if .Copyright }}
COPYRIGHT:
   {{.Copyright}}
   {{end}}{{if .Version}}
VERSION:
   {{.Version}}
   {{end}}
`

	return app
}

// Common flags that are shared across commands
func commonServiceFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "session-service-uri",
			Usage: "URI of the session service (e.g., 'sqlite://path/to/db.sqlite', 'agentengine://resource_id')",
		},
		&cli.StringFlag{
			Name:  "artifact-service-uri",
			Usage: "URI of the artifact service (e.g., 'gs://bucket-name')",
		},
		&cli.StringFlag{
			Name:  "memory-service-uri",
			Usage: "URI of the memory service (e.g., 'rag://corpus_id', 'agentengine://resource_id')",
		},
		&cli.StringFlag{
			Name:  "eval-storage-uri",
			Usage: "URI for evaluation storage (e.g., 'gs://bucket-name')",
		},
	}
}

// Common web server flags
func webServerFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Value: "127.0.0.1",
			Usage: "Host to bind the server to",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: 8000,
			Usage: "Port to bind the server to",
		},
		&cli.StringSliceFlag{
			Name:  "allow-origins",
			Usage: "Additional origins to allow for CORS",
		},
		&cli.StringFlag{
			Name:  "log-level",
			Value: "INFO",
			Usage: "Logging level (DEBUG, INFO, WARNING, ERROR, CRITICAL)",
		},
		&cli.BoolFlag{
			Name:  "trace-to-cloud",
			Usage: "Enable cloud trace for telemetry",
		},
		&cli.BoolFlag{
			Name:  "reload",
			Value: true,
			Usage: "Enable auto reload for server",
		},
		&cli.BoolFlag{
			Name:  "a2a",
			Usage: "Enable A2A endpoint",
		},
	}
}

// Helper function to validate required flags
func validateRequiredFlags(c *cli.Context, flags ...string) error {
	for _, flag := range flags {
		if c.String(flag) == "" {
			return fmt.Errorf("required flag --%s is missing", flag)
		}
	}
	return nil
}
