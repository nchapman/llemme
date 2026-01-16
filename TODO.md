# Gollama Implementation Plan

Building blocks from foundation to features. Each phase builds on the previous.

---

## Phase 1: Foundation

### 1.1 Project Setup
- [x] Initialize Go module (`go mod init github.com/nchapman/gollama`)
- [x] Set up Cobra CLI skeleton with root command
- [x] Add `version` command (just gollama version for now)
- [x] Set up basic project structure:
  ```
  cmd/
    root.go
    version.go
  internal/
    config/
    hf/
    llama/
    ui/
  main.go
  ```

### 1.2 Configuration & Paths
- [x] Define paths: `~/.gollama/`, `models/`, `bin/`
- [x] Create config struct and YAML parsing
- [x] Auto-create directories on first run
- [x] Load config with defaults → file → env var precedence

### 1.3 Pretty Output Foundation
- [x] Add Lip Gloss styles (errors, success, headers, tables)
- [x] Add Log for debug/verbose output
- [x] Create reusable UI components (spinner, progress bar wrappers)

**Checkpoint:** `gollama version` works with styled output

---

## Phase 2: Hugging Face Integration

### 2.1 HF API Client
- [x] HTTP client with proper User-Agent
- [x] Token discovery (env → file → config)
- [x] Rate limiting / retry logic
- [x] API methods:
  - [x] `GetModel(repo)` - fetch model metadata
  - [x] `ListFiles(repo)` - get repo file tree
  - [x] `SearchModels(query)` - search with GGUF filter

### 2.2 GGUF Detection
- [x] Parse filenames to extract quantization (Q4_K_M, Q8_0, etc.)
- [x] Handle naming variations (`model.Q4_K_M.gguf`, `model-q4_k_m.gguf`, etc.)
- [x] Rank quantizations by preference (Q4_K_M > Q4_K_S > Q5_K_M > ...)
- [x] Auto-select best quant when not specified

### 2.3 Model Downloads
- [x] Streaming download with resume support via HTTP Range headers
- [x] Atomic writes (download to `.partial`, rename on complete)

**Checkpoint:** Can fetch model info and download files from HF

---

## Phase 3: Model Management

### 3.1 Local Model Storage
- [x] Human-friendly directory structure (`models/user/repo/quant.gguf`)
- [ ] `metadata.json` schema and read/write
- [ ] Track: repo, quants downloaded, file sizes, SHA256, fetch date

### 3.2 `pull` Command
- [x] Parse model reference (`user/repo` or `user/repo:quant`)
- [x] Check if already downloaded
- [x] Show download progress
- [x] Pretty error messages (auth required, not found, no GGUF files)

### 3.3 `list` Command
- [x] Scan models directory
- [x] Display table: model, quant, size, modified date
- [x] Show total count and disk usage

### 3.4 `rm` Command
- [x] Delete model files and metadata
- [x] Support removing specific quant or entire repo
- [x] Confirmation prompt (Huh)

**Checkpoint:** `pull`, `list`, `rm` all working

---

## Phase 4: llama.cpp Binary Management

### 4.1 Binary Management
- [x] Detect platform (darwin/linux, amd64/arm64)
- [x] Map to llama.cpp release asset names
- [x] Download binary from GitHub releases
- [x] Extract binaries and dylibs from tarball
- [x] Set executable permissions
- [x] Store version in `bin/version.json`

### 4.2 `update` Command
- [x] Check GitHub API for latest release
- [x] Compare with installed version
- [x] Download and replace if newer
- [ ] Support `--version` flag to pin specific build

### 4.3 Version Display
- [x] Update `version` command to show llama.cpp version
- [x] Show backend type (Metal/CPU)
- [x] Show paths (models, binary)

**Checkpoint:** llama.cpp auto-downloads, `update` works

---

## Phase 5: Server Management (SINGLE BACKEND)

**Architecture Note:** All inference goes through `llama-server`. Server is single source of truth.

### 5.1 Server Process Management
- [ ] Start llama-server subprocess with model flag
- [ ] Store PID in `~/.gollama/server.pid`
- [ ] Store current model in `~/.gollama/server-state.json`
- [ ] Check PID before starting (avoid duplicate servers)
- [ ] Send SIGTERM for graceful shutdown
- [ ] Clean up PID file on exit
- [ ] Handle server crashes gracefully

### 5.2 Server Configuration
- [ ] Map config.yaml settings to llama-server flags:
  - `--host`, `--port`
  - `--ctx-size`, `--temp`, `--top-p`, `--top-k`
  - `--n-gpu-layers`, `--threads`
- [ ] Parse server logs for startup success/failure
- [ ] Extract server URL from config

### 5.3 Server Status Tracking
- [ ] `server-state.json` schema:
  ```json
  {
    "pid": 12345,
    "model": "TheBloke/Llama-2-7B-GGUF:Q4_K_M",
    "model_path": "/Users/.../Q4_K_M.gguf",
    "host": "127.0.0.1",
    "port": 8080,
    "started_at": "2024-01-15T12:00:00Z"
  }
  ```
- [ ] Write state on successful server start
- [ ] Read state to check current model
- [ ] Clean state on server stop

### 5.4 `serve` Command
- [ ] Start server (optionally with initial model)
- [ ] Pretty startup message with endpoints
- [ ] Show logs (or forward to file)
- [ ] Handle Ctrl+C gracefully (stop server)
- [ ] Support `--detach` flag for background mode

**Checkpoint:** Server starts/stops cleanly, state tracked

---

## Phase 6: Inference via HTTP API

### 6.1 HTTP Client for Server API
- [ ] Create client for llama-server API
- [ ] Implement request builders:
  - Chat completion (OpenAI-compatible)
  - Completion (Ollama-style)
- [ ] Handle streaming responses
- [ ] Parse generation results

### 6.2 `run` Command (Server Mode)
- [ ] Check if server is running (read PID file)
- [ ] If server not running or wrong model:
  - Stop existing server (if running)
  - Start server with requested model
  - Wait for server to be ready
- [ ] Send completion request via HTTP
- [ ] Stream tokens to terminal
- [ ] Detect TTY vs piped input
- [ ] Chat loop: read input, send to API, display response
- [ ] Handle Ctrl+C (stop generation, stay in chat)
- [ ] `/bye` or Ctrl+D to exit

### 6.3 One-Shot Mode
- [ ] Detect prompt argument or piped input
- [ ] Send completion request
- [ ] Print result
- [ ] Exit after completion (piped) or stay interactive (prompt arg)

### 6.4 Inference Parameters
- [ ] Pass flags to completion API:
  - `--ctx`, `-n`, `--temp`
  - `--top-p`, `--top-k`
  - `--repeat-penalty`
  - `--system` prompt
- [ ] Merge with config.yaml defaults

**Checkpoint:** `gollama run user/repo` works via HTTP API

---

## Phase 7: Server Operations

### 7.1 `ps` Command
- [ ] Check server state file
- [ ] If server not running, show "Server not running"
- [ ] Display current model, uptime, endpoint
- [ ] Pretty table format

### 7.2 `stop` Command
- [ ] Stop server: read PID, send SIGTERM
- [ ] Clean up state files
- [ ] Show confirmation message
- [ ] Support stopping specific model vs entire server

**Checkpoint:** `ps` and `stop` work

---

## Phase 8: Smart Matching

### 8.1 Model Matcher
- [ ] Build index of downloaded models
- [ ] Implement matching priority:
  1. Exact match
  2. Suffix match (repo name without user)
  3. Contains (case-insensitive)
  4. Fuzzy (Levenshtein distance)

### 8.2 Suggestions
- [ ] "Did you mean?" for close typos
- [ ] "Multiple matches" with list
- [ ] Apply to: `run`, `stop`, `rm`, `info`

**Checkpoint:** `gollama run llama` finds the right model

---

## Phase 9: Discovery

### 9.1 `search` Command
- [ ] Query HF API for GGUF models
- [ ] Display results table (repo, downloads, updated)
- [ ] Client-side filter for actual GGUF files (API filter unreliable)
- [ ] Pagination or limit results

### 9.2 `info` Command
- [ ] Fetch and display model details
- [ ] Show available quantizations with sizes
- [ ] Show which quants are downloaded locally
- [ ] Render model card excerpt (Glamour markdown)

**Checkpoint:** Can discover and inspect models

---

## Phase 10: Polish

### 10.1 Error Handling
- [ ] Actionable error messages for common failures
- [ ] Server crash handling (OOM, etc.)
- [ ] Network errors with retry suggestions
- [ ] Auth errors with setup instructions

### 10.2 Signal Handling
- [ ] Ctrl+C during download → clean up partial files
- [ ] Ctrl+C during inference → stop generation gracefully
- [ ] Ctrl+C in chat → exit cleanly

### 10.3 Edge Cases
- [ ] Split GGUF files (multi-part models)
- [ ] Disk space check before download
- [ ] Handle missing/corrupted metadata
- [ ] Concurrent access (multiple gollama processes)

### 10.4 Testing
- [ ] Unit tests for HF client, matcher, config
- [ ] Integration tests for commands
- [ ] Test on macOS (Intel + Apple Silicon)
- [ ] Test on Linux (x86_64 + arm64)

---

## Phase 11: Release

### 11.1 Distribution
- [ ] GoReleaser config for cross-compilation
- [ ] Homebrew formula
- [ ] GitHub releases with binaries
- [ ] Install script (`curl | sh`)

### 11.2 Documentation
- [ ] README with quick start
- [ ] Examples for common workflows
- [ ] Troubleshooting guide

---

## Dependencies

```
github.com/spf13/cobra           # CLI framework
github.com/charmbracelet/bubbletea  # TUI framework
github.com/charmbracelet/bubbles    # TUI components
github.com/charmbracelet/lipgloss   # Styling
github.com/charmbracelet/glamour    # Markdown rendering
github.com/charmbracelet/huh        # Forms/prompts
github.com/charmbracelet/log        # Logging
gopkg.in/yaml.v3                    # Config parsing
```

---

## Build Order Rationale

1. **Foundation first** — Can't do anything without config and pretty output
2. **HF before models** — Need API to know what to download
3. **Storage before run** — Need models on disk before inference
4. **llama.cpp before run** — Need binary to execute
5. **Server before inference** — All inference goes through server
6. **Basic run before smart** — Get it working, then make it clever
7. **Discovery is independent** — Can parallelize with Phase 5-8
8. **Polish last** — Know what edge cases exist from building

Each phase produces a working (if incomplete) tool. Ship early, iterate.
