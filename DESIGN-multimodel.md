# Multi-Model Proxy Architecture Design

## Overview

Transform lemme from a single-model wrapper into a multi-model proxy that manages multiple llama.cpp server instances. The proxy handles request routing, automatic model loading, LRU eviction, and idle timeout cleanup.

## Architecture

```
                           User Requests
                                │
                                ▼
┌───────────────────────────────────────────────────────────────────┐
│                    lemme proxy (:8080)                          │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                     HTTP Router                             │  │
│  │  /v1/chat/completions  →  extract model  →  route/load     │  │
│  │  /v1/completions       →  extract model  →  route/load     │  │
│  │  /v1/models            →  list loaded models               │  │
│  │  /health               →  proxy health                     │  │
│  └────────────────────────────────────────────────────────────┘  │
│                              │                                    │
│  ┌───────────────────────────▼────────────────────────────────┐  │
│  │                    Model Manager                            │  │
│  │  • Tracks loaded models + their backend ports               │  │
│  │  • Starts/stops llama-server instances                      │  │
│  │  • Implements LRU eviction (default max: 3 models)          │  │
│  │  • Tracks last-activity per model                           │  │
│  └────────────────────────────────────────────────────────────┘  │
│                              │                                    │
│  ┌───────────────────────────▼────────────────────────────────┐  │
│  │                   Idle Monitor (goroutine)                  │  │
│  │  • Runs every 60 seconds                                    │  │
│  │  • Shuts down models idle > 10 minutes                      │  │
│  └────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────┘
         │                    │                    │
    ┌────▼────┐         ┌─────▼─────┐       ┌─────▼─────┐
    │ llama-  │         │ llama-    │       │ llama-    │
    │ server  │         │ server    │       │ server    │
    │ :8081   │         │ :8082     │       │ :8083     │
    │ Model A │         │ Model B   │       │ Model C   │
    └─────────┘         └───────────┘       └───────────┘
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Load behavior | Auto-start and block | Simpler for clients, no retry logic needed |
| API format | OpenAI only (`/v1/*`) | Focused scope, can add Ollama later |
| Concurrency limit | LRU eviction (default 3) | Prevents OOM while being user-friendly |
| Process management | Foreground + `--detach` flag | Explicit control, auto-start on `run` |
| Idle timeout | 10 minutes | Balance between responsiveness and resources |

## Components

### 1. Proxy Server (`internal/proxy/`)

The main HTTP server that receives all client requests.

```go
// proxy/server.go
type ProxyServer struct {
    httpServer    *http.Server
    modelManager  *ModelManager
    idleMonitor   *IdleMonitor
    config        *ProxyConfig
}

type ProxyConfig struct {
    Host           string        // "127.0.0.1"
    Port           int           // 8080
    MaxModels      int           // 3 (0 = unlimited)
    IdleTimeout    time.Duration // 10 * time.Minute
    BackendPortMin int           // 8081
    BackendPortMax int           // 8100
}
```

**Responsibilities:**
- Listen on configured port (default 8080)
- Parse incoming requests to extract model name
- Delegate to ModelManager for backend resolution
- Reverse-proxy requests to appropriate backend
- Update last-activity timestamps

### 2. Model Manager (`internal/proxy/manager.go`)

Manages the lifecycle of llama-server backend instances.

```go
type ModelManager struct {
    mu           sync.RWMutex
    backends     map[string]*Backend  // model name -> backend
    lruOrder     []string             // for eviction ordering
    portAllocator *PortAllocator
    config       *ProxyConfig
}

type Backend struct {
    ModelName    string           // "TheBloke/Llama-2-7B-GGUF:Q4_K_M"
    ModelPath    string           // "/Users/x/.lemme/models/.../Q4_K_M.gguf"
    Port         int              // 8081
    Process      *os.Process      // llama-server process
    LastActivity time.Time        // for idle detection
    Status       BackendStatus    // starting, ready, stopping, stopped
    ReadyChan    chan struct{}    // closed when backend is ready
}

type BackendStatus int
const (
    BackendStarting BackendStatus = iota
    BackendReady
    BackendStopping
    BackendStopped
)
```

**Responsibilities:**
- Start llama-server processes on available ports
- Wait for backend readiness (poll `/health`)
- Track LRU order for eviction
- Stop backends (graceful SIGTERM, then SIGKILL after timeout)
- Handle concurrent requests for same model (coalesce startup)

### 3. Idle Monitor (`internal/proxy/idle.go`)

Background goroutine that cleans up unused models.

```go
type IdleMonitor struct {
    manager     *ModelManager
    idleTimeout time.Duration
    checkInterval time.Duration  // 60 seconds
    stopChan    chan struct{}
}
```

**Responsibilities:**
- Run periodic checks (every 60s)
- Identify models with LastActivity > IdleTimeout
- Request graceful shutdown via ModelManager
- Log shutdown events

### 4. Port Allocator (`internal/proxy/ports.go`)

Manages backend port assignment.

```go
type PortAllocator struct {
    mu       sync.Mutex
    minPort  int
    maxPort  int
    inUse    map[int]bool
}

func (p *PortAllocator) Allocate() (int, error)
func (p *PortAllocator) Release(port int)
```

## Request Flow

### Normal Request (Model Already Loaded)

```
1. Client → POST /v1/chat/completions {"model": "TheBloke/Llama-2-7B-GGUF:Q4_K_M", ...}
2. Proxy extracts model name from request body
3. ModelManager.GetBackend(modelName) → returns Backend{Port: 8081}
4. Update Backend.LastActivity = now
5. Update LRU order (move to front)
6. Reverse proxy request to localhost:8081
7. Stream response back to client
```

### Request for Unloaded Model

```
1. Client → POST /v1/chat/completions {"model": "mistral/Mistral-7B:Q4_K_M", ...}
2. Proxy extracts model name
3. ModelManager.GetOrLoadBackend(modelName):
   a. Check if already loading (wait on ReadyChan if so)
   b. Check if at MaxModels limit → evict LRU model
   c. Resolve model name to file path (using existing model resolver)
   d. Allocate port (e.g., 8082)
   e. Start llama-server process
   f. Poll /health until ready (with timeout)
   g. Mark status = Ready, close ReadyChan
4. Reverse proxy request to localhost:8082
5. Stream response back to client
```

### Model Eviction (LRU)

```
1. ModelManager needs to load new model but len(backends) >= MaxModels
2. Find least-recently-used model (end of lruOrder)
3. Call StopBackend(lruModelName):
   a. Set status = Stopping
   b. Send SIGTERM to process
   c. Wait up to 5s for graceful exit
   d. Send SIGKILL if still running
   e. Release port
   f. Remove from backends map and lruOrder
4. Proceed with loading new model
```

## API Endpoints

### Proxied to Backends

| Endpoint | Method | Behavior |
|----------|--------|----------|
| `/v1/chat/completions` | POST | Extract `model`, route to backend |
| `/v1/completions` | POST | Extract `model`, route to backend |
| `/v1/embeddings` | POST | Extract `model`, route to backend |

### Handled by Proxy

| Endpoint | Method | Behavior |
|----------|--------|----------|
| `/v1/models` | GET | List all loaded models with status |
| `/health` | GET | Proxy health (always 200 if running) |
| `/api/status` | GET | Detailed status: loaded models, memory, ports |

### `/v1/models` Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "TheBloke/Llama-2-7B-GGUF:Q4_K_M",
      "object": "model",
      "created": 1699900000,
      "owned_by": "local",
      "lemme": {
        "status": "ready",
        "port": 8081,
        "last_activity": "2024-01-15T10:30:00Z",
        "loaded_at": "2024-01-15T10:00:00Z"
      }
    }
  ]
}
```

### `/api/status` Response (lemme-specific)

```json
{
  "proxy": {
    "version": "0.2.0",
    "uptime_seconds": 3600,
    "host": "127.0.0.1",
    "port": 8080
  },
  "backends": {
    "max_models": 3,
    "loaded_count": 2,
    "idle_timeout_minutes": 10
  },
  "models": [
    {
      "name": "TheBloke/Llama-2-7B-GGUF:Q4_K_M",
      "status": "ready",
      "port": 8081,
      "pid": 12345,
      "loaded_at": "2024-01-15T10:00:00Z",
      "last_activity": "2024-01-15T10:30:00Z",
      "idle_minutes": 5
    }
  ]
}
```

## CLI Command Changes

### `lemme serve`

```bash
# Start proxy in foreground (logs to stdout)
lemme serve

# Start proxy in background (daemonize)
lemme serve --detach

# With custom settings
lemme serve --port 9000 --max-models 5 --idle-timeout 30m
```

**Behavior:**
- Starts proxy server
- Writes PID to `~/.lemme/proxy.pid`
- Foreground: blocks, logs to stdout, Ctrl+C stops
- Detached: backgrounds, logs to `~/.lemme/proxy.log`

### `lemme run`

```bash
lemme run TheBloke/Llama-2-7B-GGUF:Q4_K_M
```

**Behavior:**
1. Check if proxy is running (read proxy.pid, check process)
2. If not running, auto-start proxy in background
3. Send request to proxy to ensure model is loaded
4. Start interactive chat session

### `lemme ps`

```bash
$ lemme ps

Proxy Status
  • Running on http://127.0.0.1:8080 (PID 12345)
  • Uptime: 2h 15m
  • Max models: 3

Loaded Models

  MODEL                           QUANT     PORT    STATUS    IDLE      MEMORY
  TheBloke/Llama-2-7B-GGUF        Q4_K_M    8081    ready     2m ago    3.8 GB
  mistralai/Mistral-7B-GGUF       Q4_K_M    8082    ready     8m ago    4.1 GB

Total: 2 models loaded, 7.9 GB memory
```

### `lemme stop`

```bash
# Stop specific model (unload from memory)
lemme stop TheBloke/Llama-2-7B-GGUF:Q4_K_M
# → "✓ Unloaded TheBloke/Llama-2-7B-GGUF:Q4_K_M"

# Stop all models but keep proxy running
lemme stop --all
# → "✓ Unloaded 2 models"

# Stop proxy entirely (and all models)
lemme stop --proxy
# → "✓ Stopped proxy and unloaded 2 models"
```

## File Structure Changes

```
~/.lemme/
├── models/              # unchanged
├── bin/                 # unchanged
├── proxy.pid            # NEW: proxy process ID
├── proxy.log            # NEW: proxy logs (when detached)
├── proxy.sock           # FUTURE: unix socket for local CLI communication
└── config.yaml          # updated with proxy settings
```

## Configuration Changes

```yaml
# ~/.lemme/config.yaml

# Proxy configuration (NEW)
proxy:
  host: "127.0.0.1"
  port: 8080
  max_models: 3              # 0 = unlimited
  idle_timeout: "10m"        # duration string
  backend_port_range: "8081-8100"

# Server configuration (existing, now per-backend defaults)
server:
  ctx_len: 4096
  gpu_layers: -1
  threads: 0

# Inference defaults (unchanged)
temperature: 0.7
top_p: 0.9
# ...
```

## State Persistence

The proxy maintains state in memory only. On restart:
- All backend processes are orphaned (they'll exit naturally or on next cleanup)
- Proxy starts fresh with no models loaded
- First request for each model triggers a new load

**Future consideration:** State file for crash recovery (not in initial implementation).

## Error Handling

### Backend Startup Failure

```
1. llama-server fails to start (bad model, OOM, etc.)
2. ModelManager detects via process exit or health check timeout
3. Clean up: release port, remove from backends
4. Return 500 to client: {"error": {"message": "Failed to load model: <reason>", "type": "server_error"}}
```

### Backend Crash During Request

```
1. Reverse proxy gets connection error
2. Mark backend as stopped, clean up
3. Return 502 to client: {"error": {"message": "Backend server crashed", "type": "server_error"}}
4. Next request will trigger fresh load
```

### Startup Timeout

```
1. llama-server started but /health not responding after 60s
2. Kill process, release port
3. Return 504 to client: {"error": {"message": "Model load timeout", "type": "server_error"}}
```

## Implementation Phases

### Phase 1: Core Proxy (MVP)
- [ ] ProxyServer with basic routing
- [ ] ModelManager with start/stop
- [ ] Port allocator
- [ ] LRU eviction
- [ ] Update `serve` command
- [ ] Update `run` to auto-start proxy

### Phase 2: Monitoring & Polish
- [ ] IdleMonitor goroutine
- [ ] Update `ps` command for new output
- [ ] Update `stop` command with new flags
- [ ] `/api/status` endpoint
- [ ] Logging improvements

### Phase 3: Robustness
- [ ] Health check recovery (restart crashed backends)
- [ ] Graceful proxy shutdown (drain connections)
- [ ] Request queuing during model load
- [ ] Metrics/observability

## Model Name Resolution

The proxy uses the **same fuzzy matching as the CLI** for model names in API requests. This means:

```
POST /v1/chat/completions
{"model": "llama", "messages": [...]}
```

Will match `TheBloke/Llama-2-7B-GGUF:Q4_K_M` if it's the only downloaded model containing "llama".

**Matching priority** (same as CLI):
1. Exact match (`TheBloke/Llama-2-7B-GGUF:Q4_K_M`)
2. Suffix match (`Llama-2-7B-GGUF`)
3. Contains match, case-insensitive (`llama`)
4. Fuzzy match for typos

**Ambiguous matches return 400:**
```json
{
  "error": {
    "message": "Ambiguous model name 'mistral'. Matches: TheBloke/Mistral-7B-GGUF:Q4_K_M, mistralai/Mistral-7B-Instruct-GGUF:Q4_K_M",
    "type": "invalid_request_error"
  }
}
```

**No matches return 404:**
```json
{
  "error": {
    "message": "No downloaded model matches 'lama'. Did you mean: TheBloke/Llama-2-7B-GGUF?",
    "type": "not_found"
  }
}
```

## Open Questions

1. **Streaming:** llama-server uses SSE for streaming. Ensure reverse proxy handles this correctly (chunked transfer, no buffering).

2. **Request body parsing:** Need to parse JSON body to extract `model` field, then re-serialize for proxying. Consider buffering implications for large requests.

3. **Concurrent loads:** If 10 requests come in simultaneously for an unloaded model, ensure only one backend starts (coalesce via ReadyChan pattern).

## Testing Strategy

### Unit Tests
- PortAllocator: allocation, release, exhaustion
- LRU ordering: updates, eviction selection
- Model name parsing from request bodies

### Integration Tests
- Start proxy, load model, make request
- LRU eviction when at limit
- Idle timeout triggers shutdown
- Concurrent requests for same unloaded model
- Backend crash recovery

### Manual Testing
- `run` auto-starts proxy
- `ps` shows correct state
- `stop` variations work
- Memory doesn't leak over time
