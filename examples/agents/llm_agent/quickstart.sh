#!/bin/bash
# Quickstart script for DuckDuckGo Search agent example

set -e

go build -buildmode=plugin -o examples/duckduckgo_search/agent.so examples/duckduckgo_search/agent.go
go run ./cmd/adk web examples
