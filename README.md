# llemme

A fast, simple CLI for running LLMs locally. Powered by [llama.cpp](https://github.com/ggerganov/llama.cpp) with direct [Hugging Face](https://huggingface.co) integration.

## Features

- 🚀 **Run any GGUF model** from Hugging Face with a single command
- 🔄 **Multi-model support** - load multiple models, auto-unload when idle
- 💬 **Interactive chat** or one-shot inference
- 📦 **Automatic downloads** with progress tracking
- 🔍 **Fuzzy model matching** - type `llama` instead of `bartowski/Llama-3.2-3B-Instruct-GGUF:Q4_K_M`
- ⚡ **OpenAI-compatible API** - works with existing tools and libraries
- 🎯 **Zero config** - auto-downloads llama.cpp, picks optimal quantization

## Install

```bash
go install github.com/nchapman/llemme@latest
```

Or build from source:

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
llemme list

# Show running models
llemme ps
```

## Commands

| Command | Description |
|---------|-------------|
| `run <model>` | Chat with a model (auto-downloads if needed) |
| `pull <model>` | Download a model from Hugging Face |
| `list` | List downloaded models |
| `search <query>` | Search Hugging Face for models |
| `ps` | Show proxy status and loaded models |
| `stop <model>` | Unload a model |
| `serve` | Start the proxy server |
| `info <model>` | Show model details |
| `rm <model>` | Delete a downloaded model |
| `update` | Update llama.cpp binaries |

## Multi-Model Support

Lemme runs a proxy that manages multiple llama.cpp backends. Models load on demand and unload after 10 minutes idle.

```bash
# Use the OpenAI-compatible API
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "llama", "messages": [{"role": "user", "content": "Hello!"}]}'
```

## Configuration

Config lives at `~/.llemme/config.yaml`:

```yaml
context_length: 4096
temperature: 0.7
default_quant: Q4_K_M
gpu_layers: -1      # -1 = all layers on GPU

proxy:
  host: 127.0.0.1   # bind address (0.0.0.0 for all interfaces)
  port: 8080
  max_models: 3
  idle_timeout_mins: 10

# Pass any llama-server options directly
llama_server:
  parallel: 4       # concurrent requests per model
  threads: 8        # CPU threads (-1 = auto)
  mlock: true       # lock model in RAM
```

See [llama-server docs](https://github.com/ggerganov/llama.cpp/tree/master/examples/server) for all available options.

## Logs

Logs are stored in `~/.llemme/logs/`:
- `proxy.log` - Proxy server logs
- `<model-name>.log` - Per-model backend logs (e.g., `llama-3.2-3b-instruct-q4_k_m.log`)

Logs rotate automatically (max 10MB, keeps 3 generations).

## License

MIT
