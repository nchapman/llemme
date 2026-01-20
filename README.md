# llemme

Run local LLMs with [llama.cpp](https://github.com/ggerganov/llama.cpp) and [Hugging Face](https://huggingface.co).

## Features

- 🚀 **Run any GGUF model** from Hugging Face with a single command
- 🔄 **Multi-model support** - load multiple models, auto-unload when idle
- 💬 **Interactive chat** or one-shot inference
- 📦 **Automatic downloads** with progress tracking
- 🔍 **Fuzzy model matching** - type `llama` instead of `bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M`
- ⚡ **OpenAI-compatible API** - works with existing tools and libraries
- 🤖 **Anthropic API support** - use with Claude Code and Anthropic SDKs
- 🎯 **Zero config** - auto-downloads llama.cpp, picks optimal quantization

## Install

**Homebrew (macOS/Linux):**
```bash
brew install nchapman/tap/llemme
```

**Go:**
```bash
go install github.com/nchapman/llemme@latest
```

**Build from source:**

```bash
git clone https://github.com/nchapman/llemme
cd llemme
go build -o llemme .
```

## Usage

```bash
# Run a model (downloads automatically)
llemme run bartowski/Llama-3.2-3B-Instruct-GGUF

# One-shot prompt
llemme run llama "Explain quantum computing in one sentence"

# Search for models
llemme search mistral

# List downloaded models
llemme list    # or: llemme ls

# Show running models
llemme status  # or: llemme ps
```

## Commands

**Model Commands**
| Command | Alias | Description |
|---------|-------|-------------|
| `run <model>` | | Chat with a model (auto-downloads if needed) |
| `pull <model>` | | Download a model from Hugging Face |
| `list` | `ls` | List downloaded models |
| `remove <model>` | `rm` | Delete a downloaded model |
| `unload <model>` | | Unload a running model |
| `status` | `ps` | Show server status and loaded models |

**Personas**
| Command | Description |
|---------|-------------|
| `persona list` | List all personas |
| `persona create <name>` | Create a new persona |
| `persona show <name>` | Show persona details |
| `persona edit <name>` | Edit a persona in your editor |
| `persona rm <name>` | Delete a persona |

**Server**
| Command | Description |
|---------|-------------|
| `server start` | Start the proxy server |
| `server stop` | Stop the proxy server |
| `server restart` | Restart the proxy server |

**Discovery**
| Command | Alias | Description |
|---------|-------|-------------|
| `search <query>` | | Search Hugging Face for models |
| `trending` | | Show trending models |
| `info <model>` | `show` | Show model details |

**Configuration**
| Command | Description |
|---------|-------------|
| `config edit` | Open config in your editor |
| `config show` | Print current configuration |
| `config path` | Print config file path |
| `config reset` | Reset config to defaults |
| `update` | Update llama.cpp binaries |
| `version` | Show version information |

## Multi-Model Support

Lemme runs a proxy that manages multiple llama.cpp backends. Models load on demand and unload after 10 minutes idle.

```bash
# Use the OpenAI-compatible API
curl http://localhost:11313/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "llama", "messages": [{"role": "user", "content": "Hello!"}]}'
```

## Using with Claude Code

llemme supports the Anthropic Messages API, so you can use it as a backend for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

**1. Pull a model with good instruction-following and tool-use capabilities:**

```bash
llemme pull unsloth/GLM-4.7-Flash-GGUF
```

**2. Configure llemme to map Claude model names to your local model:**

```yaml
# ~/.llemme/config.yaml
server:
  claude_model: "unsloth/GLM-4.7-Flash-GGUF"
```

**3. Start llemme and run Claude Code:**

```bash
llemme server start
ANTHROPIC_BASE_URL=http://127.0.0.1:11313 claude
```

That's it! Claude Code will send requests to llemme, which routes them to your local model.

**Alternative: Configure Claude Code directly**

Instead of setting `claude_model` in llemme, you can tell Claude Code which model to request:

```bash
ANTHROPIC_BASE_URL=http://127.0.0.1:11313 \
ANTHROPIC_DEFAULT_SONNET_MODEL="unsloth/GLM-4.7-Flash-GGUF" \
claude
```

This lets you set different models per tier (Sonnet is the default). See also `ANTHROPIC_DEFAULT_OPUS_MODEL` and `ANTHROPIC_DEFAULT_HAIKU_MODEL`.

## Configuration

Config lives at `~/.llemme/config.yaml`. Edit with `llemme config edit` or view with `llemme config show`.

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

Logs are stored in `~/.llemme/logs/`:
- `proxy.log` - Proxy server logs
- `<model-name>.log` - Per-model backend logs (e.g., `llama-3.2-3b-instruct-q4_k_m.log`)

Logs rotate automatically (max 10MB, keeps 3 generations).

## License

MIT
