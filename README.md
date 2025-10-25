# Claude Code Proxy (Go)

A lightweight, standalone HTTP proxy that enables Claude Code to work with OpenAI-compatible API providers.

## Features

- ✅ **Pattern-based routing** - Auto-detects Claude models and routes to appropriate OpenAI models
- ✅ **Zero dependencies** - Single ~10MB binary, no runtime needed
- ✅ **Daemon mode** - Runs in background, serves multiple Claude Code sessions
- ✅ **Fast startup** - < 10ms cold start
- ✅ **Config flexibility** - Loads from `~/.claude/proxy.env`

## Quick Start

### Build

```bash
# Install dependencies
go mod download

# Build binary
go build -o claude-code-proxy cmd/claude-code-proxy/main.go

# Or use make
make build
```

### Configuration

Create config file:

```bash
mkdir -p ~/.claude
cat > ~/.claude/proxy.env << 'EOF'
OPENAI_API_KEY=sk-your-key-here
EOF
```

### Run

**Option 1: Start daemon manually**

```bash
./claude-code-proxy           # Start daemon
./claude-code-proxy status    # Check status
./claude-code-proxy stop      # Stop daemon
```

**Option 2: Use ccp wrapper (recommended)**

```bash
# Copy wrapper script
cp scripts/ccp ~/.local/bin/ccp
chmod +x ~/.local/bin/ccp

# Use ccp instead of claude
ccp chat
ccp --version
```

The `ccp` wrapper automatically:
- Starts the proxy daemon (if not running)
- Sets `ANTHROPIC_BASE_URL`
- Execs `claude` with your arguments

## Pattern-Based Routing

The proxy auto-detects Claude model names:

| Claude Model Pattern | Default OpenAI Model |
|---------------------|---------------------|
| `*opus*` | `gpt-5` |
| `*sonnet-4*`, `*sonnet-5*` | `gpt-5` |
| `*sonnet-3*` | `gpt-4o` |
| `*haiku*` | `gpt-5-mini` |

Override with env vars:
```bash
ANTHROPIC_DEFAULT_OPUS_MODEL=gpt-5
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-4o
ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-5-mini
```

## Build for Distribution

```bash
# Build for all platforms
make build-all

# Output:
# dist/claude-code-proxy-darwin-amd64
# dist/claude-code-proxy-darwin-arm64
# dist/claude-code-proxy-linux-amd64
# dist/claude-code-proxy-linux-arm64
```

## Configuration Reference

**Required:**
- `OPENAI_API_KEY` - Your OpenAI API key

**Optional:**
- `OPENAI_BASE_URL` - API base URL (default: `https://api.openai.com/v1`)
- `ANTHROPIC_API_KEY` - Client API key validation (optional)
- `ANTHROPIC_DEFAULT_OPUS_MODEL` - Override opus routing
- `ANTHROPIC_DEFAULT_SONNET_MODEL` - Override sonnet routing
- `ANTHROPIC_DEFAULT_HAIKU_MODEL` - Override haiku routing
- `HOST` - Server host (default: `0.0.0.0`)
- `PORT` - Server port (default: `8082`)

## Project Structure

```
proxy/
├── cmd/
│   └── claude-code-proxy/
│       └── main.go           # Entry point
├── internal/
│   ├── config/               # Config loading
│   ├── daemon/               # Process management
│   ├── server/               # HTTP server (Fiber)
│   └── converter/            # Claude ↔ OpenAI conversion
├── pkg/
│   └── models/               # Shared types
├── scripts/
│   └── ccp                   # Shell wrapper
└── Makefile                  # Build automation
```

## Development

```bash
# Run in dev mode
go run cmd/claude-code-proxy/main.go

# Run tests
make test

# Format code
make fmt

# Lint
make lint
```

## License

MIT
