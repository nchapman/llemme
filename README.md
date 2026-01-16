# lemme

A fast, simple CLI for running LLMs locally. Powered by [llama.cpp](https://github.com/ggerganov/llama.cpp) with direct [Hugging Face](https://huggingface.co) integration.

## Features

- 🚀 **Run any GGUF model** from Hugging Face with a single command
- 🔄 **Multi-model support** - load multiple models, auto-unload when idle
- 💬 **Interactive chat** or one-shot inference
- 📦 **Automatic downloads** with progress tracking
- 🔍 **Fuzzy model matching** - type `llama` instead of `TheBloke/Llama-2-7B-GGUF:Q4_K_M`
- ⚡ **OpenAI-compatible API** - works with existing tools and libraries
- 🎯 **Zero config** - auto-downloads llama.cpp, picks optimal quantization

## Install

```bash
go install github.com/nchapman/lemme@latest
```

Or build from source:

```bash
git clone https://github.com/nchapman/lemme
cd lemme
go build -o lemme .
```

## Usage

```bash
# Run a model (downloads automatically)
lemme run TheBloke/Llama-2-7B-GGUF

# One-shot prompt
lemme run llama "Explain quantum computing in one sentence"

# Search for models
lemme search mistral

# List downloaded models
lemme list

# Show running models
lemme ps
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

Config lives at `~/.lemme/config.yaml`:

```yaml
context_length: 4096
temperature: 0.7
default_quant: Q4_K_M
gpu_layers: -1

proxy:
  port: 8080
  max_models: 3
  idle_timeout_mins: 10
```

## License

MIT
