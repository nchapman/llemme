# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build                    # Build binary to ./llemme
make test                     # Run all tests
make check                    # Format + vet + test (run before committing)
go test ./cmd -run TestName   # Run single test (add -v for verbose)
go test ./internal/proxy      # Test specific package
```

Linting uses golangci-lint with `errcheck` and `unused` disabled.

## Architecture Overview

**llemme** is a Go CLI for running local LLMs via llama.cpp with Hugging Face model management. Built on Charmbracelet (bubbletea, lipgloss, glamour) for TUI and Cobra for CLI.

### Multi-Model Proxy (Core Architecture)

The central design is a reverse-proxy that manages multiple llama.cpp backend instances:

```
CLI/API → Proxy (port 11313) → Routes by model name → llama-server instances (:11314+)
```

Key packages in `internal/proxy/`:
- `server.go` - HTTP routing, reverse-proxy to backends
- `manager.go` - Model lifecycle (start/stop llama-server, LRU eviction, max 3 models)
- `idle.go` - Background monitor for auto-unloading idle models
- `ports.go` - Dynamic port allocation for backends

### Package Structure

- `cmd/` - Cobra CLI commands (run, pull, list, serve, status, etc.)
- `internal/config/` - Config loading/saving, personas (saved model presets)
- `internal/hf/` - Hugging Face API client, model downloads, quantization detection
- `internal/llama/` - llama.cpp binary management
- `internal/proxy/` - Multi-model proxy server
- `internal/server/` - Backend API client (OpenAI-compatible)
- `internal/tui/` - Bubbletea TUI (chat model, components, styles)
- `internal/ui/` - CLI utilities (spinner, progress, table, logger)

### Data Storage

All data lives in `~/.llemme/`:
- `config.yaml` - User configuration
- `models/` - Downloaded GGUF files (`user/repo/quant.gguf`)
- `bin/` - llama.cpp binaries
- `logs/` - Rotating log files

## Code Patterns

### Constructors
All major types use `NewXxx() *Type` pattern.

### Imports
Standard library → external deps → internal packages (blank lines between groups).

### Error Handling
Always wrap errors with context using `fmt.Errorf("context: %w", err)`.

### Testing
Table-driven tests with subtests:
```go
tests := []struct{ name, input, expected string }{...}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {...})
}
```

### TUI
Bubbletea Model interface: `Init()`, `Update()`, `View()`. Components in `internal/tui/components/`.

### No Unnecessary Comments
Code should be self-documenting through clear naming and structure.
