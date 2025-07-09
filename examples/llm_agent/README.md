Build as plugin:
```
go build -buildmode=plugin -o examples/duckduckgo_search/agent.so examples/duckduckgo_search/agent.go
```
Then from project root:
```
go run ./cmd/adk web examples
```
To run with all agents under examples