# Lemme

A beautiful Go CLI wrapper around [llama.cpp](https://github.com/ggerganov/llama.cpp) that brings the simplicity of [Ollama](https://ollama.com) with direct Hugging Face integration.

## Features

- 🚀 **Run any GGUF model** from Hugging Face with a single command
- 🤖 **Interactive chat mode** or one-shot inference
- 📦 **Automatic model downloads** with progress bars
- 🔧 **Auto-managed llama.cpp** - no manual installation needed
- 🎨 **Beautiful terminal output** powered by Charm libraries
- ⚡ **Server-first architecture** - OpenAI-compatible HTTP API
- 📝 **Configuration via YAML** or command-line flags

## Quick Start

### Install

```bash
# Clone and build
git clone https://github.com/nchapman/lemme
cd lemme
make build
sudo cp build/lemme /usr/local/bin/
```

### First Run

Lemme automatically downloads llama.cpp on first use:

```bash
lemme run TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF:Q6_K "Hello, world!"
```

### Interactive Chat

```bash
lemme run TheBloke/Llama-2-7B-GGUF

# Start chatting
You: What is the capital of France?
AI: The capital of France is Paris.
You: /bye
```

### One-Shot Inference

```bash
# Pass prompt as argument
lemme run model:quant "Write a haiku about code"

# Or pipe input
echo "Summarize this text" | lemme run model:quant
```

## Commands

```bash
lemme run <user/repo>[:quant]    # Interactive chat or one-shot inference
lemme serve                        # Start llama.cpp server
lemme pull <user/repo>[:quant]    # Download a model
lemme list                         # List downloaded models
lemme ps                           # Show server status
lemme stop                          # Stop server
lemme rm <user/repo>[:quant]    # Remove a model
lemme version                       # Show versions
lemme update                       # Update llama.cpp
```

## Model Management

### Pull a Model

```bash
# Pull specific quantization
lemme pull TheBloke/Llama-2-7B-GGUF:Q4_K_M

# Pull and auto-select best quant
lemme pull TheBloke/Llama-2-7B-GGUF
```

### List Models

```bash
lemme list

Downloaded Models

  MODEL                               QUANT     SIZE      MODIFIED
  TheBloke/Llama-2-7B-GGUF         Q4_K_M    4.1 GB    2 hours ago
  TheBloke/TinyLlama-1.1B-Chat      Q6_K      862 MB    1 day ago

Total: 2 models, 4.9 GB
```

### Remove a Model

```bash
lemme rm TheBloke/Llama-2-7B-GGUF:Q4_K_M
```

## Server Mode

Lemme runs inference through `llama-server`, exposing an OpenAI-compatible HTTP API.

### Start Server

```bash
lemme serve --model TheBloke/Llama-2-7B-GGUF:Q4_K_M --detach
```

### Server Endpoints

```bash
# OpenAI-compatible
curl -X POST http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "model",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'

# Ollama-compatible
curl -X POST http://127.0.0.1:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "model": "model",
    "message": "Hello"
  }'
```

## Configuration

Create `~/.lemme/config.yaml`:

```yaml
server:
  host: "127.0.0.1"
  port: 8080

# Default inference parameters
context_length: 4096
temperature: 0.7
top_p: 0.9
top_k: 40
repeat_penalty: 1.1
gpu_layers: -1
threads: 0

# Default quantization preference
default_quant: Q4_K_M

# Hugging Face token (optional)
hf_token: ""
```

## CLI Flags

### Run/Chat Flags

```bash
-c, --ctx <n>           Context length (default: from config)
-n, --predict <n>       Max tokens to generate
-t, --temp <f>          Temperature
--top-p <f>             Top-p sampling
--top-k <n>             Top-k sampling
--repeat-penalty        Repeat penalty
--gpu-layers <n>        GPU layers to offload
--threads <n>           CPU threads
--system <prompt>        System prompt
```

### Server Flags

```bash
--host <addr>           Server host (default: 127.0.0.1)
--port <n>              Server port (default: 8080)
--detach                Run server in background
```

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Check (format, vet, test)

```bash
make check
```

### Coverage

```bash
make test-coverage
open build/coverage.html
```

## Architecture

Lemme uses a **server-first architecture** where all inference goes through `llama-server`. This provides:

- Unified model loading and state management
- Easy concurrent request handling
- Simple process management
- Better observability (centralized logs)
- OpenAI-compatible API out of the box

```
lemme run → llama-server → HTTP API → Inference
```

## License

MIT

## Acknowledgments

- [llama.cpp](https://github.com/ggerganov/llama.cpp) - LLM inference backend
- [Ollama](https://github.com/ollama/ollama) - CLI design inspiration
- [Charm](https://charm.sh) - Beautiful terminal components
- [Hugging Face](https://huggingface.co) - Model hosting
