# Gollama - MVP Specification

A beautiful Go CLI wrapper around llama.cpp that brings the simplicity of Ollama with direct Hugging Face integration.

## Vision

Gollama makes running local LLMs effortless. Point it at any GGUF model on Hugging Face, and it handles the rest—downloading, caching, and running inference through llama.cpp. No model conversion, no complex setup, just `gollama run username/model` and go.

## Core Principles

1. **Zero friction** - One command to run any GGUF model from Hugging Face
2. **Familiar UX** - If you know Ollama, you know Gollama
3. **Beautiful output** - Polished terminal experience using Charm libraries
4. **Transparent** - Uses llama.cpp directly, no hidden abstraction layers
5. **Lightweight** - Minimal dependencies, fast startup

## MVP Features

### Commands

```
gollama run <user/repo>          # Interactive chat (or one-shot with prompt)
gollama serve                    # Start the API server
gollama pull <user/repo>         # Download a model without running
gollama list                     # List downloaded models
gollama ps                       # Show models loaded in server
gollama stop <user/repo>         # Unload a model from server
gollama rm <user/repo>           # Remove a downloaded model
gollama update                   # Update llama.cpp to latest release
gollama version                  # Show gollama + llama.cpp versions
```

**`run` behavior** (matches Ollama):
```
gollama run user/repo              # Interactive chat session
gollama run user/repo "prompt"     # One-shot: print response, then interactive
echo "prompt" | gollama run ...    # Piped: print response and exit
```

**Smart model matching** (for `run`, `stop`, `rm`, `info`):

Gollama matches partial names against downloaded models. If unique, it just works. If ambiguous, it suggests.

```
gollama run llama                  # Matches "TheBloke/Llama-2-7B-GGUF" if it's the only llama

gollama run mistral

  Multiple models match 'mistral':

    • TheBloke/Mistral-7B-v0.1-GGUF
    • mistralai/Mistral-7B-Instruct-v0.2-GGUF

  Be more specific, or use the full name.
```

```
gollama run lama

  No models match 'lama'. Did you mean?

    • TheBloke/Llama-2-7B-GGUF
    • TheBloke/CodeLlama-7B-GGUF
```

**Matching priority:**
1. Exact match (full `user/repo` or `user/repo:quant`)
2. Suffix match (`Llama-2-7B-GGUF` matches `TheBloke/Llama-2-7B-GGUF`)
3. Contains match (case-insensitive)
4. Fuzzy match for typo suggestions

### Model References

Models are referenced using the simple `username/repository` format:

```
gollama run TheBloke/Llama-2-7B-GGUF
gollama run microsoft/phi-2-gguf
gollama run mistralai/Mistral-7B-v0.1-GGUF
```

For repos with multiple GGUF files, append the quantization:

```
gollama run TheBloke/Llama-2-7B-GGUF:Q4_K_M
gollama run TheBloke/Llama-2-7B-GGUF:Q8_0
```

If no quantization is specified, Gollama picks the best available (preferring Q4_K_M).

### Model Discovery

```
gollama search llama           # Search Hugging Face for GGUF models
gollama info <user/repo>       # Show model details and available quantizations
```

## Technical Architecture

### Directory Structure

**Design goal:** Human-navigable file structure. You should be able to `ls ~/.gollama/models` and immediately understand what you have.

```
~/.gollama/
├── models/
│   └── TheBloke/
│       └── Llama-2-7B-GGUF/
│           ├── Q4_K_M.gguf              # Actual model file, named by quantization
│           ├── Q8_0.gguf                # Multiple quants can coexist
│           └── metadata.json            # Repo info, available quants, etc.
├── blobs/
│   └── sha256-a1b2c3d4...               # Content-addressed storage for deduplication
├── bin/
│   └── llama-cli                        # llama.cpp binary (auto-managed)
└── config.yaml                          # User configuration
```

**Why this structure:**

| Ollama | Gollama | Why |
|--------|---------|-----|
| `~/.ollama/models/manifests/...` | `~/.gollama/models/TheBloke/Llama-2-7B-GGUF/` | Browsable with standard tools |
| `~/.ollama/blobs/sha256-abc123` | `~/.gollama/models/.../Q4_K_M.gguf` | Filename tells you the quantization |
| Requires `ollama list` to understand | `ls` or Finder works fine | No CLI required to explore |

**Blob storage (optional deduplication):**

The `blobs/` directory is for advanced cases where the same GGUF file appears in multiple repos (rare but possible). The model files in `models/` are either:
- Direct files (default, simple case)
- Symlinks to `blobs/sha256-xxx` (when deduplication is detected)

This keeps the common case simple while still allowing space savings for power users with many models.

**Example: What `ls -la` looks like:**

```bash
$ ls ~/.gollama/models/
TheBloke/
microsoft/
mistralai/

$ ls ~/.gollama/models/TheBloke/
CodeLlama-7B-GGUF/
Llama-2-7B-GGUF/
Mistral-7B-v0.1-GGUF/

$ ls ~/.gollama/models/TheBloke/Llama-2-7B-GGUF/
Q4_K_M.gguf      # 4.1 GB
Q5_K_M.gguf      # 4.8 GB
Q8_0.gguf        # 7.2 GB
metadata.json
```

### Dependencies

**Runtime:**
- llama.cpp CLI binary (auto-downloaded on first run)

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
3. `hf_token` in `~/.gollama/config.yaml` (fallback)

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
- Track installed version in `~/.gollama/bin/version.json`
- `gollama update` fetches latest release from GitHub

**Version Strategy:**
- Pin to known-good releases by default (tested compatibility)
- `gollama update` moves to latest stable release
- `gollama update --version b4567` pins to specific build
- Store version info for reproducibility

**Execution:**
- Spawn `llama-cli` as subprocess for `run` command
- Stream stdout/stderr to terminal with pretty formatting
- Pass through common flags: `-c`, `-n`, `--temp`, etc.

### Server Mode

Gollama wraps `llama-server` (included in llama.cpp releases) to provide an OpenAI-compatible API.

**Starting the server:**
```
gollama serve                           # Start on default port 8080
gollama serve --port 11434              # Custom port (Ollama-compatible)
gollama serve --host 0.0.0.0            # Listen on all interfaces
```

**API Compatibility:**
- Exposes OpenAI-compatible `/v1/chat/completions` endpoint
- Also exposes `/api/generate` and `/api/chat` (Ollama-style)
- Models loaded on-demand when first requested

**Model management with running server:**
```
gollama ps                              # List loaded models
gollama stop TheBloke/Llama-2-7B-GGUF   # Unload model from memory
```

**Server config in `~/.gollama/config.yaml`:**
```yaml
server:
  host: "127.0.0.1"
  port: 8080
  # Models to preload on server start (optional)
  preload: []
```

## User Experience

### Download Progress

```
Pulling TheBloke/Llama-2-7B-GGUF:Q4_K_M

  ████████████████████░░░░░░░░░░  68% │ 2.8 GB / 4.1 GB │ 45 MB/s │ ETA 30s

Model info:
  • Parameters: 7B
  • Quantization: Q4_K_M (4.1 GB)
  • License: META LLAMA 2
```

### Interactive Mode

```
gollama run TheBloke/Llama-2-7B-GGUF

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
gollama list

Downloaded Models

  MODEL                           QUANT     SIZE      MODIFIED
  TheBloke/Llama-2-7B-GGUF        Q4_K_M    4.1 GB    2 days ago
  microsoft/phi-2-gguf            Q4_0      1.6 GB    1 week ago
  mistralai/Mistral-7B-GGUF       Q8_0      7.7 GB    3 weeks ago

Total: 3 models, 13.4 GB
```

### Search Results

```
gollama search codellama

Found 24 GGUF models for "codellama"

  REPOSITORY                              DOWNLOADS    UPDATED
  TheBloke/CodeLlama-7B-GGUF              125.4k       Oct 2024
  TheBloke/CodeLlama-13B-GGUF             89.2k        Oct 2024
  TheBloke/CodeLlama-34B-GGUF             45.1k        Oct 2024
  ...

Use 'gollama info <repo>' for details
```

### Server & Process Management

```
gollama serve

Server started on http://127.0.0.1:8080

  Endpoints:
    • OpenAI:  /v1/chat/completions
    • Ollama:  /api/chat, /api/generate

  Logs: ~/.gollama/server.log

Press Ctrl+C to stop
```

```
gollama ps

Loaded Models

  MODEL                           QUANT     SIZE      VRAM      LOADED
  TheBloke/Llama-2-7B-GGUF        Q4_K_M    4.1 GB    3.8 GB    2 min ago

Total: 1 model, 3.8 GB VRAM used
```

```
gollama stop TheBloke/Llama-2-7B-GGUF

Unloaded TheBloke/Llama-2-7B-GGUF
```

### Version Info

```
gollama version

Gollama v0.1.0 (darwin/arm64)
llama.cpp b3847 (Metal)

Paths:
  Models:    ~/.gollama/models/
  llama.cpp: ~/.gollama/bin/llama-cli
```

On Linux:
```
gollama version

Gollama v0.1.0 (linux/amd64)
llama.cpp b3847 (CPU)

Paths:
  Models:    ~/.gollama/models/
  llama.cpp: ~/.gollama/bin/llama-cli
```

### Updating llama.cpp

```
gollama update

Checking for updates...

  Current: b3847
  Latest:  b3912

Downloading llama.cpp b3912 for darwin/arm64...

  ████████████████████████████████  100% │ 48 MB │ Done

Updated successfully!
```

```
gollama update

llama.cpp is already up to date (b3912)
```

## Configuration

**~/.gollama/config.yaml:**

```yaml
# Default context length
context_length: 4096

# Default temperature
temperature: 0.7

# Preferred quantization (when not specified)
default_quant: Q4_K_M

# GPU layers to offload (-1 = all)
gpu_layers: -1

# Custom llama.cpp path (optional)
llama_path: ""

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
-c, --ctx <n>       Context length (default: 4096)
-n, --predict <n>   Max tokens to generate (default: -1, infinite)
-t, --temp <f>      Temperature (default: 0.7)
--top-p <f>         Top-p sampling (default: 0.9)
--top-k <n>         Top-k sampling (default: 40)
--repeat-penalty    Repeat penalty (default: 1.1)
--gpu-layers <n>    Layers to offload to GPU (-1 = all)
--threads <n>       CPU threads to use
--system <prompt>   System prompt
```

## Error Handling

**User-friendly errors with suggestions:**

```
Error: Model not found

  Could not find 'TheBloke/Llama-2-7B-GUF' on Hugging Face.

  Did you mean?
    • TheBloke/Llama-2-7B-GGUF
    • TheBloke/Llama-2-7B-Chat-GGUF

  Tips:
    • Check the spelling of the repository name
    • Use 'gollama search llama' to find models
```

```
Error: No GGUF files found

  The repository 'meta-llama/Llama-2-7b' exists but contains no GGUF files.

  Try one of these GGUF versions:
    • TheBloke/Llama-2-7B-GGUF
    • TheBloke/Llama-2-7B-Chat-GGUF
```

```
Error: Authentication required

  The repository 'meta-llama/Llama-3-8B-GGUF' requires authentication.

  To access gated models, provide a Hugging Face token:
    1. Get a token at https://huggingface.co/settings/tokens
    2. Run: huggingface-cli login
       Or set: export HF_TOKEN=hf_xxxxx
```

## MVP Scope

### In Scope (v0.1)

- [x] `run` - Interactive chat / one-shot inference
- [x] `serve` - Start API server (wraps llama-server)
- [x] `pull` - Download model
- [x] `list` - List local models
- [x] `ps` - Show models loaded in server
- [x] `stop` - Unload model from server
- [x] `rm` - Remove model
- [x] `search` - Search Hugging Face
- [x] `info` - Show model details
- [x] `update` - Update llama.cpp binary
- [x] `version` - Show version info
- [x] Auto-download llama.cpp binary on first run
- [x] Progress bars for downloads
- [x] Basic configuration file
- [x] macOS support (Intel + Apple Silicon with Metal)
- [x] Linux support (x86_64 + arm64, CPU)

### Out of Scope (Future)

- [ ] Windows support
- [ ] Linux CUDA/ROCm support
- [ ] Model aliases (`gollama run llama2` → resolves to full path)
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
3. **Streaming** - Real-time token streaming vs buffered output?

## References

- [llama.cpp](https://github.com/ggerganov/llama.cpp)
- [Hugging Face Hub API](https://huggingface.co/docs/hub/api)
- [Hugging Face Environment Variables](https://huggingface.co/docs/huggingface_hub/en/package_reference/environment_variables)
- [Ollama](https://github.com/ollama/ollama)
- [Charm Libraries](https://charm.sh)