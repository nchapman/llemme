# Gollama Implementation Plan

Building blocks from foundation to features. Each phase builds on the previous.

---

## Phase 1: Foundation

### 1.1 Project Setup
- [ ] Initialize Go module (`go mod init github.com/nchapman/gollama`)
- [ ] Set up Cobra CLI skeleton with root command
- [ ] Add `version` command (just gollama version for now)
- [ ] Set up basic project structure:
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
- [ ] Define paths: `~/.gollama/`, `models/`, `bin/`
- [ ] Create config struct and YAML parsing
- [ ] Auto-create directories on first run
- [ ] Load config with defaults → file → env var precedence

### 1.3 Pretty Output Foundation
- [ ] Add Lip Gloss styles (errors, success, headers, tables)
- [ ] Add Log for debug/verbose output
- [ ] Create reusable UI components (spinner, progress bar wrappers)

**Checkpoint:** `gollama version` works with styled output

---

## Phase 2: Hugging Face Integration

### 2.1 HF API Client
- [ ] HTTP client with proper User-Agent
- [ ] Token discovery (env → file → config)
- [ ] Rate limiting / retry logic
- [ ] API methods:
  - [ ] `GetModel(repo)` - fetch model metadata
  - [ ] `ListFiles(repo)` - get repo file tree
  - [ ] `SearchModels(query)` - search with GGUF filter

### 2.2 GGUF Detection
- [ ] Parse filenames to extract quantization (Q4_K_M, Q8_0, etc.)
- [ ] Handle naming variations (`model.Q4_K_M.gguf`, `model-q4_k_m.gguf`, etc.)
- [ ] Rank quantizations by preference (Q4_K_M > Q4_K_S > Q5_K_M > ...)
- [ ] Auto-select best quant when not specified

### 2.3 Model Downloads
- [ ] Streaming download with progress callback
- [ ] Resume support via HTTP Range headers
- [ ] SHA256 verification (HF provides hashes)
- [ ] Atomic writes (download to `.partial`, rename on complete)
- [ ] Clean up partial files on interrupt

**Checkpoint:** Can fetch model info and download files from HF

---

## Phase 3: Model Management

### 3.1 Local Model Storage
- [ ] Human-friendly directory structure (`models/user/repo/quant.gguf`)
- [ ] `metadata.json` schema and read/write
- [ ] Track: repo, quants downloaded, file sizes, SHA256, fetch date

### 3.2 `pull` Command
- [ ] Parse model reference (`user/repo` or `user/repo:quant`)
- [ ] Check if already downloaded
- [ ] Show progress with Bubbles progress bar
- [ ] Write metadata on success
- [ ] Pretty error messages (auth required, not found, no GGUF files)

### 3.3 `list` Command
- [ ] Scan models directory
- [ ] Display table: model, quant, size, modified date
- [ ] Show total count and disk usage

### 3.4 `rm` Command
- [ ] Delete model files and metadata
- [ ] Support removing specific quant or entire repo
- [ ] Confirmation prompt (Huh)

**Checkpoint:** `pull`, `list`, `rm` all working

---

## Phase 4: llama.cpp Integration

### 4.1 Binary Management
- [ ] Detect platform (darwin/linux, amd64/arm64)
- [ ] Map to llama.cpp release asset names
- [ ] Download binary from GitHub releases
- [ ] Extract and set executable permissions
- [ ] Store version in `bin/version.json`

### 4.2 `update` Command
- [ ] Check GitHub API for latest release
- [ ] Compare with installed version
- [ ] Download and replace if newer
- [ ] Support `--version` flag to pin specific build

### 4.3 Version Display
- [ ] Update `version` command to show llama.cpp version
- [ ] Show backend type (Metal/CPU)
- [ ] Show paths (models, binary)

**Checkpoint:** llama.cpp auto-downloads, `update` works

---

## Phase 5: Inference

### 5.1 Basic `run` Command
- [ ] Resolve model reference to GGUF path
- [ ] Spawn `llama-cli` subprocess
- [ ] Pass through flags (-c, -n, --temp, etc.)
- [ ] Stream stdout to terminal
- [ ] Handle process exit codes

### 5.2 Interactive Mode
- [ ] Detect TTY vs piped input
- [ ] Chat loop with Bubbles text input
- [ ] Display model info header
- [ ] Stream tokens as they arrive
- [ ] Handle Ctrl+C gracefully (stop generation, don't exit)
- [ ] `/bye` or Ctrl+D to exit

### 5.3 One-Shot Mode
- [ ] Detect prompt argument or piped input
- [ ] Run inference, print result
- [ ] Exit after completion (piped) or stay interactive (prompt arg)

**Checkpoint:** `gollama run user/repo` works interactively

---

## Phase 6: Smart Matching

### 6.1 Model Matcher
- [ ] Build index of downloaded models
- [ ] Implement matching priority:
  1. Exact match
  2. Suffix match (repo name without user)
  3. Contains (case-insensitive)
  4. Fuzzy (Levenshtein distance)

### 6.2 Suggestions
- [ ] "Did you mean?" for close typos
- [ ] "Multiple matches" with list
- [ ] Apply to: `run`, `stop`, `rm`, `info`

**Checkpoint:** `gollama run llama` finds the right model

---

## Phase 7: Discovery

### 7.1 `search` Command
- [ ] Query HF API for GGUF models
- [ ] Display results table (repo, downloads, updated)
- [ ] Client-side filter for actual GGUF files (API filter unreliable)
- [ ] Pagination or limit results

### 7.2 `info` Command
- [ ] Fetch and display model details
- [ ] Show available quantizations with sizes
- [ ] Show which quants are downloaded locally
- [ ] Render model card excerpt (Glamour markdown)

**Checkpoint:** Can discover and inspect models

---

## Phase 8: Server Mode

### 8.1 `serve` Command
- [ ] Spawn `llama-server` subprocess
- [ ] Configure host/port from flags or config
- [ ] Pretty startup message with endpoints
- [ ] Forward logs to file and optionally terminal
- [ ] Graceful shutdown on Ctrl+C

### 8.2 Model Loading
- [ ] Load model on first request (lazy)
- [ ] Track loaded models and memory usage
- [ ] Unload models on timeout or memory pressure

### 8.3 `ps` Command
- [ ] Query server for loaded models
- [ ] Display table: model, quant, VRAM, load time

### 8.4 `stop` Command
- [ ] Send unload request to server
- [ ] Support smart matching for model name
- [ ] Confirm unload success

**Checkpoint:** Full server mode with `serve`, `ps`, `stop`

---

## Phase 9: Polish

### 9.1 Error Handling
- [ ] Actionable error messages for common failures
- [ ] llama.cpp crash handling (OOM, etc.)
- [ ] Network errors with retry suggestions
- [ ] Auth errors with setup instructions

### 9.2 Signal Handling
- [ ] Ctrl+C during download → clean up partial files
- [ ] Ctrl+C during inference → stop generation gracefully
- [ ] Ctrl+C in chat → exit cleanly with message

### 9.3 Edge Cases
- [ ] Split GGUF files (multi-part models)
- [ ] Disk space check before download
- [ ] Handle missing/corrupted metadata
- [ ] Concurrent access (multiple gollama processes)

### 9.4 Testing
- [ ] Unit tests for HF client, matcher, config
- [ ] Integration tests for commands
- [ ] Test on macOS (Intel + Apple Silicon)
- [ ] Test on Linux (x86_64 + arm64)

---

## Phase 10: Release

### 10.1 Distribution
- [ ] GoReleaser config for cross-compilation
- [ ] Homebrew formula
- [ ] GitHub releases with binaries
- [ ] Install script (`curl | sh`)

### 10.2 Documentation
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
4. **llama.cpp before run** — Need the binary to execute
5. **Basic run before smart** — Get it working, then make it clever
6. **Discovery is independent** — Can parallelize with Phase 5-6
7. **Server after run** — Similar patterns, more complexity
8. **Polish last** — Know what edge cases exist from building

Each phase produces a working (if incomplete) tool. Ship early, iterate.
