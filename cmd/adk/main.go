package main

import (
	"log"
	"os"

	"github.com/agent-protocol/adk-golang/pkg/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
