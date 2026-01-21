# lleme

Run local LLMs with [llama.cpp](https://github.com/ggerganov/llama.cpp) and [Hugging Face](https://huggingface.co).

## Features

- 🚀 **Run Any GGUF Model**: Execute any model from Hugging Face with a single command.
- 🔄 **Dynamic Multi-Model Serving**: Serves multiple models, loading them on-demand and unloading them when idle to conserve resources.
- 💬 **Interactive & One-Shot Chat**: Chat with models in an interactive TUI or get quick answers via single command-line prompts.
- 🔎 **Discover & Manage Models**: Search Hugging Face, view trending models, and manage your local model library (`list`, `pull`, `rm`).
- 🤖 **Custom Personas**: Create and switch between custom personalities and system prompts for tailored interactions.
- ⚡ **Universal API Support**: Acts as a local, drop-in replacement for both OpenAI and Anthropic APIs.
- ⚙️ **Powered by [llama.cpp](https://github.com/ggerganov/llama.cpp)**: Enjoy a zero-config start with smart defaults, or take full control with direct access to all underlying `llama.cpp` parameters.

## Install

**Homebrew (macOS/Linux):**
```bash
brew install nchapman/tap/lleme
```

**Go:**
```bash
go install github.com/nchapman/lleme@latest
```

**Build from source:**

```bash
git clone https://github.com/nchapman/lleme
cd lleme
go build -o lleme .
```

## Usage

```bash
# Run a model (downloads automatically)
lleme run unsloth/gpt-oss-20b-GGUF

# One-shot prompt
lleme run unsloth/gpt-oss-20b-GGUF "Explain quantum computing in one sentence"

# Search for models
lleme search mistral

# List downloaded models
lleme list    # or: lleme ls

# Show running models
lleme status  # or: lleme ps
```

**Note on Model Names:** `lleme` is smart about resolving downloaded model names via a case-insensitive substring search. For example, a partial query like `gpt-oss-20b` would match `unsloth/gpt-oss-20b-GGUF:Q4_K_M`. Punctuation is significant and not removed before matching. If a partial name matches uniquely, it runs. If it matches multiple quantizations of the same model, `lleme` picks the best one. If ambiguous, it will ask for more specifics.

_An animated demonstration of `lleme run` will go here._
_To record one, you can use `asciinema rec lleme-demo.cast` then convert with `svg-term --in lleme-demo.cast --out lleme-demo.svg`._

## Commands

| Category | Command | Alias | Description |
|---|---|---|---|
| Model | `run <model>` | | Chat with a model (auto-downloads if needed) |
| Model | `pull <model>` | | Download a model from Hugging Face |
| Model | `list` | `ls` | List downloaded models |
| Model | `remove [pattern]` | `rm` | Delete downloaded models by name, pattern, or filter (--older-than, --larger-than) |
| Model | `unload <model>` | | Unload a running model |
| Model | `status` | `ps` | Show server status and loaded models |
| Personas | `persona list` | | List all personas |
| Personas | `persona create <name>` | | Create a new persona |
| Personas | `persona show <name>` | | Show persona details |
| Personas | `persona edit <name>` | | Edit a persona in your editor |
| Personas | `persona rm <name>` | | Delete a persona |
| Server | `server start` | | Start the proxy server |
| Server | `server stop` | | Stop the proxy server |
| Server | `server restart` | | Restart the proxy server |
| Discovery | `search <query>` | | Search Hugging Face for models |
| Discovery | `trending` | | Show trending models |
| Discovery | `info <model>` | `show` | Show model details |
| Config | `config edit` | | Open config in your editor |
| Config | `config show` | | Print current configuration |
| Config | `config path` | | Print config file path |
| Config | `config reset` | | Reset config to defaults |
| Config | `update` | | Update llama.cpp binaries |
| Config | `version` | | Show version information |

### Advanced Model Removal

The `remove` command offers powerful filtering options to manage your downloaded models:

-   **By specific name/pattern:**
    ```bash
    lleme remove user/repo:quant       # Remove a specific model quantization
    lleme remove user/repo             # Remove all quantizations of a model
    lleme remove user/*                # Remove all models from a specific user
    lleme remove *                     # Remove all downloaded models
    ```
-   **By age:**
    ```bash
    lleme remove --older-than 30d      # Remove models not used in 30 days
    lleme remove --older-than 4w       # Remove models not used in 4 weeks
    ```
-   **By size:**
    ```bash
    lleme remove --larger-than 10GB    # Remove models larger than 10GB
    lleme remove --larger-than 500MB   # Remove models larger than 500MB
    ```
-   **Combine patterns and filters:**
    ```bash
    lleme remove user/* --older-than 7d  # Remove models from 'user' not used in 7 days
    ```
    Use the `--force` (`-f`) flag to skip the confirmation prompt.

## Multi-Model Support

Lemme runs a proxy that manages multiple llama.cpp backends. Models load on demand and unload after a configurable idle period (defaulting to 10 minutes) to conserve resources.

```bash
# Use the OpenAI-compatible API
curl http://localhost:11313/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "unsloth/gpt-oss-20b-GGUF", "messages": [{"role": "user", "content": "Hello!"}]}'
```

## Using with Claude Code

lleme supports the Anthropic Messages API, so you can use it as a backend for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

**1. Pull a model with good instruction-following and tool-use capabilities:**

```bash
lleme pull unsloth/GLM-4.7-Flash-GGUF
```

**2. Configure lleme to map Claude model names to your local model:**

```yaml
# ~/.lleme/config.yaml
server:
  claude_model: "unsloth/GLM-4.7-Flash-GGUF"
```

**3. Start lleme and run Claude Code:**

```bash
lleme server start
ANTHROPIC_BASE_URL=http://127.0.0.1:11313 claude
```

That's it! Claude Code will send requests to lleme, which routes them to your local model.

**Alternative: Configure Claude Code directly**

Instead of setting `claude_model` in lleme, you can tell Claude Code which model to request:

```bash
ANTHROPIC_BASE_URL=http://127.0.0.1:11313 \
ANTHROPIC_DEFAULT_SONNET_MODEL="unsloth/GLM-4.7-Flash-GGUF" \
claude
```

This lets you set different models per tier (Sonnet is the default). See also `ANTHROPIC_DEFAULT_OPUS_MODEL` and `ANTHROPIC_DEFAULT_HAIKU_MODEL`.

## Configuration

Config lives at `~/.lleme/config.yaml`. Edit with `lleme config edit` or view with `lleme config show`.

```yaml
huggingface:
  default_quant: Q4_K_M

server:
  host: 127.0.0.1   # bind address (0.0.0.0 for all interfaces)
  port: 11313
  max_models: 3
  idle_timeout: 10m
  # claude_model: "unsloth/GLM-4.7-Flash-GGUF"  # for Claude Code

llamacpp:
  options:
    # ctx-size: 4096
    # gpu-layers: -1  # -1 = all layers on GPU
    # threads: 8      # CPU threads
    # parallel: 4     # concurrent requests per model
```

See [llama-server docs](https://github.com/ggerganov/llama.cpp/tree/master/examples/server) for all available options.

## Logs

Logs are stored in `~/.lleme/logs/`:
- `proxy.log` - Proxy server logs
- `<model-name>.log` - Per-model backend logs (e.g., `llama-3.2-3b-instruct-q4_k_m.log`)

Logs rotate automatically (max 10MB, keeps 3 generations).

## License

MIT
