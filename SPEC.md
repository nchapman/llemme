# Lemme - MVP Specification

A beautiful Go CLI wrapper around llama.cpp that brings simplicity of Ollama with direct Hugging Face integration.

## Vision

Lemme makes running local LLMs effortless. Point it at any GGUF model on Hugging Face, and it handles the rest—downloading, caching, and running inference through llama.cpp. No model conversion, no complex setup, just `llemme run username/model` and go.

## Core Principles

1. **Zero friction** - One command to run any GGUF model from Hugging Face
2. **Familiar UX** - If you know Ollama, you know Lemme
3. **Beautiful output** - Polished terminal experience using Charm libraries
4. **Transparent** - Uses llama.cpp directly, no hidden abstraction layers
5. **Lightweight** - Minimal dependencies, fast startup

## Architecture

**Single Backend Approach:** All Lemme commands interact with `llama-server` via HTTP API. The server is the single source of truth for model state, inference, and resource management.

**How it works:**
- `run` ensures server is running, then makes HTTP requests for completions
- `serve` starts the server and keeps it running
- `ps` queries server API for loaded models
- `stop` tells server to unload models or shutdown
- All inference goes through the same server process

**Why server-only:**
- Unified model loading and state management
- Easy concurrent requests later
- Simpler process management (single server process)
- Better observability (centralized logs and metrics)
- Matches Ollama's architecture

## MVP Features

### Commands

```
llemme run <user/repo>[:quant]   # Interactive chat or one-shot inference
llemme serve                       # Start llama.cpp server
llemme pull <user/repo>[:quant]    # Download a model without running
llemme list                         # List downloaded models
llemme ps                           # Show models loaded in server
llemme stop <user/repo>[:quant]    # Unload model from server
llemme rm <user/repo>[:quant]      # Remove a downloaded model
llemme search <query>               # Search Hugging Face for GGUF models
llemme info <user/repo>             # Show model details
llemme update                       # Update llama.cpp to latest release
llemme version                      # Show llemme + llama.cpp versions
```

**`run` behavior** (matches Ollama):
```
llemme run user/repo              # Interactive chat session
llemme run user/repo "prompt"     # One-shot: print response, then stay in chat
echo "prompt" | llemme run ...    # Piped: print response and exit
```

**Smart model matching** (for `run`, `stop`, `rm`, `info`):

Lemme matches partial names against downloaded models. If unique, it just works. If ambiguous, it suggests.

```
llemme run llama                  # Matches "bartowski/Llama-3.2-3B-Instruct-GGUF" if it's the only llama

llemme run mistral

  Multiple models match 'mistral':

    • bartowski/Mistral-7B-Instruct-v0.3-GGUF
    • mistralai/Mistral-7B-Instruct-v0.2-GGUF

  Be more specific, or use the full name.
```

```
llemme run lama

  No models match 'lama'. Did you mean?

    • bartowski/Llama-3.2-3B-Instruct-GGUF
    • bartowski/CodeLlama-7B-Instruct-GGUF
```

**Matching priority:**
1. Exact match (full `user/repo` or `user/repo:quant`)
2. Suffix match (`Llama-2-7B-GGUF` matches `bartowski/Llama-3.2-3B-Instruct-GGUF`)
3. Contains match (case-insensitive)
4. Fuzzy match for typo suggestions

### Model References

Models are referenced using the simple `username/repository` format:

```
llemme run bartowski/Llama-3.2-3B-Instruct-GGUF
llemme run microsoft/phi-2-gguf
llemme run mistralai/Mistral-7B-v0.1-GGUF
```

For repos with multiple GGUF files, append the quantization:

```
llemme run bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M
llemme run bartowski/Llama-3.2-3B-Instruct-GGUF:Q8_0
```

If no quantization is specified, Lemme picks the best available (preferring Q4_K_M).

## Technical Architecture

### Directory Structure

**Design goal:** Human-navigable file structure. You should be able to `ls ~/.llemme/models` and immediately understand what you have.

```
~/.llemme/
├── models/
│   └── bartowski/
│       └── Llama-2-7B-GGUF/
│           ├── Q4_K_M.gguf              # Actual model file, named by quantization
│           ├── Q8_0.gguf                # Multiple quants can coexist
│           └── metadata.json            # Repo info, available quants, etc.
├── bin/
│   ├── llama-server.bin                # llama.cpp server binary (auto-managed)
│   ├── *.dylib                       # Dynamic libraries (macOS)
│   └── version.json                  # Installed llama.cpp version
├── server.pid                       # Server process ID (when running)
└── config.yaml                      # User configuration
```

**Why this structure:**

| Ollama | Lemme | Why |
|--------|---------|-----|
| `~/.ollama/models/manifests/...` | `~/.llemme/models/bartowski/Llama-3.2-3B-Instruct-GGUF/` | Browsable with standard tools |
| `~/.ollama/blobs/sha256-abc123` | `~/.llemme/models/.../Q4_K_M.gguf` | Filename tells you the quantization |
| Requires `ollama list` to understand | `ls` or Finder works fine | No CLI required to explore |

**Example: What `ls -la` looks like:**

```bash
$ ls ~/.llemme/models/
bartowski/
microsoft/
mistralai/

$ ls ~/.llemme/models/bartowski/
CodeLlama-7B-GGUF/
Llama-2-7B-GGUF/
Mistral-7B-v0.1-GGUF/

$ ls ~/.llemme/models/bartowski/Llama-3.2-3B-Instruct-GGUF/
Q4_K_M.gguf      # 4.1 GB
Q5_K_M.gguf      # 4.8 GB
Q8_0.gguf        # 7.2 GB
metadata.json
```

### Dependencies

**Runtime:**
- llama.cpp server binary (auto-downloaded on first run)

**Go Libraries:**
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - Spinners, progress bars, text input
- `github.com/charmbracelet/lipgloss` - Styling and layout
- `github.com/charmbracelet/glamour` - Markdown rendering
- `github.com/charmbracelet/huh` - Interactive forms/prompts
- `github.com/charmbracelet/log` - Pretty logging
- `github.com/spf13/cobra` - CLI framework

### Hugging Face Integration

**API Approach:**
Use the Hugging Face Hub REST API directly—no external dependency needed. Standard `net/http` with JSON parsing is sufficient.

**API Endpoints:**
```
GET  /api/models/{user}/{repo}              # Model metadata
GET  /api/models/{user}/{repo}/tree/main    # File listing
GET  /{user}/{repo}/resolve/main/{file}     # File download
GET  /api/models?search=X&filter=gguf       # Search for GGUF models
```

Base URL: `https://huggingface.co`

**Authentication:**

Token lookup order (first match wins):
1. `HF_TOKEN` environment variable
2. `~/.cache/huggingface/token` (standard HF CLI location)
3. `hf_token` in `~/.llemme/config.yaml` (fallback)

For authenticated requests, add header:
```
Authorization: Bearer hf_xxxxxxxxxxxxx
```

**Why this order:**
- Env var is standard for CI/containers and matches HF's own priority
- Reading the HF token file means `huggingface-cli login` just works
- Config file fallback for users who don't want to use the HF CLI

**GGUF Detection:**
- Scan repo files for `.gguf` extension
- Parse quantization from filename (e.g., `*-Q4_K_M.gguf`)
- Cache available quantizations in metadata

### llama.cpp Integration

**Binary Management:**
- Auto-download prebuilt llama.cpp release on first run
- macOS: Universal binary with Metal support
- Linux: x86_64 and arm64, CPU-only (CUDA support later)
- Allow user override via `LLAMA_CPP_PATH` env var
- Track installed version in `~/.llemme/bin/version.json`
- `llemme update` fetches latest release from GitHub

**Server Mode (Single Backend):**

All inference goes through `llama-server`. Lemme manages the server lifecycle:

- Start server on-demand when needed
- Keep server running for subsequent requests
- Gracefully shutdown when idle or on explicit stop
- Server exposes OpenAI-compatible HTTP API
- Models loaded/unloaded via API calls

**Server Start:**
```bash
llama-server --host 127.0.0.1 --port 8080 --model /path/to/model.gguf
```

**Server API:**
- OpenAI-compatible `/v1/chat/completions`
- Ollama-style `/api/chat` and `/api/generate`
- `/health` endpoint for status checks

**Process Management:**
- Store PID in `~/.llemme/server.pid`
- Check PID before starting (avoid duplicate servers)
- Send SIGTERM for graceful shutdown
- Clean up PID file on exit

### Server Configuration

**Server config in `~/.llemme/config.yaml`:**
```yaml
server:
  host: "127.0.0.1"
  port: 8080
  # Context length for all requests
  ctx_len: 4096
  # Default generation parameters
  temperature: 0.7
  top_p: 0.9
  top_k: 40
  repeat_penalty: 1.1
  # GPU layers to offload (-1 = all)
  gpu_layers: -1
  # CPU threads (0 = auto)
  threads: 0
```

**CLI flags override config:**
```
llemme run user/repo --temp 0.5 --ctx 8192
```

## User Experience

### Download Progress

```
Pulling bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M

  Model info:
    • Size: 4.1 GB

  Downloading...

  ████████████████████░░░░░░░░░  68% │ 2.8 GB / 4.1 GB │ 45 MB/s │ ETA 30s

✓ Pulled bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M successfully!
```

### Interactive Mode

```
llemme run bartowski/Llama-3.2-3B-Instruct-GGUF

┌─────────────────────────────────────────────────────────────────────────┐
│  Llama 2 7B • Q4_K_M • 4096 ctx                                         │
└─────────────────────────────────────────────────────────────────────────┘

You: What is the capital of France?

AI: The capital of France is Paris. It's the largest city in France and serves
    as the country's political, economic, and cultural center...

You: █
```

### Model List

```
llemme list

Downloaded Models

  MODEL                           QUANT     SIZE      MODIFIED
  bartowski/Llama-3.2-3B-Instruct-GGUF        Q4_K_M    4.1 GB    2 days ago
  microsoft/phi-2-gguf            Q4_0      1.6 GB    1 week ago
  mistralai/Mistral-7B-GGUF       Q8_0      7.7 GB    3 weeks ago

Total: 3 models, 13.4 GB
```

### Server & Process Management

```
llemme serve

Server started on http://127.0.0.1:8080

  Endpoints:
    • OpenAI:  /v1/chat/completions
    • Ollama:  /api/chat, /api/generate

Press Ctrl+C to stop
```

```
llemme ps

Server Status

  • Running on http://127.0.0.1:8080

Loaded Models

  MODEL                           QUANT     SIZE      MEMORY    LOADED
  bartowski/Llama-3.2-3B-Instruct-GGUF        Q4_K_M    4.1 GB    3.8 GB    2 min ago

Total: 1 model, 3.8 GB memory used
```

```
llemme stop bartowski/Llama-3.2-3B-Instruct-GGUF

✓ Unloaded bartowski/Llama-3.2-3B-Instruct-GGUF from server
```

```
llemme stop

✓ Server stopped
```

### Version Info

```
llemme version

Lemme v0.1.0 (darwin/arm64)
llama.cpp b7751 (Metal)

Paths:
  Models:    ~/.llemme/models/
  Server:    ~/.llemme/bin/llama-server.bin
```

On Linux:
```
llemme version

Lemme v0.1.0 (linux/amd64)
llama.cpp b7751 (CPU)

Paths:
  Models:    ~/.llemme/models/
  Server:    ~/.llemme/bin/llama-server.bin
```

### Updating llama.cpp

```
llemme update

Checking for updates...

  Current: b3847
  Latest:  b7751

Update llama.cpp from b3847 to b7751? [y/N] y

Downloading llama.cpp b7751 for darwin/arm64...

Extracting...

✓ Updated successfully to b7751!
```

```
llemme update

llama.cpp is already up to date (b7751)
```

## Configuration

**~/.llemme/config.yaml:**

```yaml
# Server configuration
server:
  host: "127.0.0.1"
  port: 8080

# Default inference parameters
context_length: 4096
temperature: 0.7
top_p: 0.9
top_k: 40
repeat_penalty: 1.1

# Model configuration
default_quant: Q4_K_M
gpu_layers: -1
threads: 0

# Hugging Face token (fallback - prefers HF_TOKEN env var or ~/.cache/huggingface/token)
hf_token: ""
```

## CLI Flags

### Global Flags

```
--verbose, -v       Enable verbose output
--quiet, -q         Suppress non-essential output
--config <path>     Use alternate config file
```

### Run/Chat Flags

```
-c, --ctx <n>       Context length (default: from config, 4096)
-n, --predict <n>   Max tokens to generate (default: -1, infinite)
-t, --temp <f>      Temperature (default: from config, 0.7)
--top-p <f>         Top-p sampling (default: from config, 0.9)
--top-k <n>         Top-k sampling (default: from config, 40)
--repeat-penalty    Repeat penalty (default: from config, 1.1)
--gpu-layers <n>    Layers to offload to GPU (-1 = all)
--threads <n>       CPU threads to use (default: auto)
--system <prompt>   System prompt
```

### Server Flags

```
--host <addr>       Server host (default: 127.0.0.1)
--port <n>          Server port (default: 8080)
--detach            Run server in background (don't keep terminal open)
```

## Error Handling

**User-friendly errors with suggestions:**

```
Error: Model not found

  Could not find 'bartowski/Llama-3.2-3B-Instruct-GUF' on Hugging Face.

  Did you mean?
    • bartowski/Llama-3.2-3B-Instruct-GGUF
    • bartowski/Llama-3.2-3B-Instruct-GGUF

  Tips:
    • Check the spelling of the repository name
    • Use 'llemme search llama' to find models
```

```
Error: No GGUF files found

  The repository 'meta-llama/Llama-2-7b' exists but contains no GGUF files.

  Try one of these GGUF versions:
    • bartowski/Llama-3.2-3B-Instruct-GGUF
    • bartowski/Llama-3.2-3B-Instruct-GGUF
```

```
Error: Authentication required

  The repository 'meta-llama/Llama-3-8B-GGUF' requires authentication.

  To access gated models, provide a Hugging Face token:
    1. Get a token at https://huggingface.co/settings/tokens
    2. Run: huggingface-cli login
       Or set: export HF_TOKEN=hf_xxxxx
```

```
Error: Server not running

  The llama.cpp server is not running.

  Start it with: llemme serve
  Or use: llemme run <model> (will auto-start server)
```

## MVP Scope

### In Scope (v0.1)

- [x] `run` - Interactive chat / one-shot inference (via HTTP API)
- [x] `serve` - Start llama.cpp server
- [x] `pull` - Download model
- [x] `list` - List local models
- [x] `ps` - Show server status and loaded models
- [x] `stop` - Unload model or stop server
- [x] `rm` - Remove model
- [x] `search` - Search Hugging Face
- [x] `info` - Show model details
- [x] `update` - Update llama.cpp binary
- [x] `version` - Show version info
- [x] Auto-download llama.cpp binary on first run
- [x] Progress bars for downloads
- [x] Basic configuration file
- [x] Server-mode inference (single backend)
- [x] macOS support (Intel + Apple Silicon with Metal)
- [x] Linux support (x86_64 + arm64, CPU)

### Out of Scope (Future)

- [ ] Windows support
- [ ] Linux CUDA/ROCm support
- [ ] Model aliases (`llemme run llama2` → resolves to full path)
- [ ] Modelfile support (Ollama-style customization)
- [ ] Embedding generation
- [ ] Multi-model conversations
- [ ] Conversation history/persistence
- [ ] Model recommendations based on hardware
- [ ] Automatic quantization selection based on RAM

## Success Metrics

1. **Time to first inference** - Under 60 seconds from install to running first model
2. **Command parity** - Core Ollama commands work identically
3. **Pretty by default** - All output is visually polished without extra flags

## Open Questions

1. **llama.cpp version pinning** - Pin to specific release or track latest?
2. **Model naming collisions** - How to handle `user/repo` that matches multiple quantizations?
3. **Server auto-start** - Always auto-start server on `run`, or require explicit `serve`?

## References

- [llama.cpp](https://github.com/ggerganov/llama.cpp)
- [llama.cpp Server](https://github.com/ggerganov/llama.cpp/tree/master/examples/server)
- [Hugging Face Hub API](https://huggingface.co/docs/hub/api)
- [Hugging Face Environment Variables](https://huggingface.co/docs/huggingface_hub/en/package_reference/environment_variables)
- [Ollama](https://github.com/ollama/ollama)
- [Charm Libraries](https://charm.sh)
