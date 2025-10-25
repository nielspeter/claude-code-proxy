# Claude Code Proxy (Go)

A lightweight HTTP proxy that enables Claude Code to work with OpenAI-compatible API providers including OpenRouter (200+ models), OpenAI Direct (GPT-5 reasoning), and Ollama (free local inference).

> **⚠️ Early Stage / Beta Software**
>
> This project is in active development and has limited production testing. While core functionality works, edge cases and some provider-specific features may have issues. Use with caution in production environments.
>
> **Feedback welcome!** Please report issues at https://github.com/nielspeter/claude-code-proxy/issues

## Features

- ✅ **Full Claude Code Compatibility** - Complete support for all Claude Code features
  - Tool calling (read, write, edit, glob, grep, bash, etc.)
  - Extended thinking blocks with proper hiding/showing
  - Streaming responses with real-time token tracking
  - Proper SSE event formatting
- ✅ **Multiple Provider Support** - OpenRouter, OpenAI Direct, and Ollama
  - **OpenRouter**: 200+ models (GPT, Grok, Gemini, etc.) through single API
  - **OpenAI Direct**: Native GPT-5 reasoning model support
  - **Ollama**: Free local inference with DeepSeek-R1, Llama3, Qwen, etc.
- ✅ **Pattern-based routing** - Auto-detects Claude models and routes to appropriate backend models
- ✅ **Zero dependencies** - Single ~10MB binary, no runtime needed
- ✅ **Daemon mode** - Runs in background, serves multiple Claude Code sessions
- ✅ **Fast startup** - < 10ms cold start
- ✅ **Config flexibility** - Loads from `~/.claude/proxy.env` or `.env`
- ✅ **Passthrough mode** - Optional direct proxying to Anthropic API for debugging

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

### Install

**Option 1: System-wide installation (recommended)**

```bash
# Install binary and ccp wrapper to /usr/local/bin
make install

# This installs:
#   - claude-code-proxy (main binary)
#   - ccp (wrapper script for easy usage)
```

**Option 2: Manual installation**

```bash
# Copy binary to PATH
sudo cp claude-code-proxy /usr/local/bin/

# Copy wrapper script (optional but recommended)
sudo cp scripts/ccp /usr/local/bin/
sudo chmod +x /usr/local/bin/ccp
```

After installation, `claude-code-proxy` and `ccp` will be available system-wide.

### Configuration

The proxy supports three provider types. Choose the one that fits your needs:

**Option 1: OpenRouter (Recommended)**
```bash
mkdir -p ~/.claude
cat > ~/.claude/proxy.env << 'EOF'
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=sk-or-v1-your-openrouter-key

# Model routing
ANTHROPIC_DEFAULT_SONNET_MODEL=x-ai/grok-code-fast-1
ANTHROPIC_DEFAULT_HAIKU_MODEL=google/gemini-2.5-flash

# Optional: Better rate limits
OPENROUTER_APP_NAME=Claude-Code-Proxy
OPENROUTER_APP_URL=https://github.com/yourname/repo
EOF
```

**Option 2: OpenAI Direct**
```bash
mkdir -p ~/.claude
cat > ~/.claude/proxy.env << 'EOF'
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=sk-proj-your-openai-key

# Model routing
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-5
ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-5-mini
ANTHROPIC_DEFAULT_OPUS_MODEL=gpt-5  # Reasoning model
EOF
```

**Option 3: Ollama (Local)**
```bash
mkdir -p ~/.claude
cat > ~/.claude/proxy.env << 'EOF'
OPENAI_BASE_URL=http://localhost:11434/v1
# No API key needed!

# Model routing
ANTHROPIC_DEFAULT_SONNET_MODEL=deepseek-r1:70b
ANTHROPIC_DEFAULT_HAIKU_MODEL=llama3.1:8b
EOF
```

## Provider Comparison

| Feature | OpenRouter | OpenAI Direct | Ollama |
|---------|-----------|---------------|--------|
| **Cost** | Pay-per-use | Pay-per-use | Free |
| **Setup** | Easy | Easy | Requires local install |
| **Models** | 200+ | OpenAI only | Open source only |
| **Reasoning** | Yes (via GPT/Grok/etc) | Yes (GPT-5) | Yes (DeepSeek-R1) |
| **Tool Calling** | Yes | Yes | Model dependent |
| **Privacy** | Cloud | Cloud | 100% local |
| **Speed** | Fast | Fast | Very fast (local) |
| **API Key** | Required | Required | Not needed |

### Run

**Commands:**

```bash
./claude-code-proxy              # Start daemon
./claude-code-proxy status       # Check if running
./claude-code-proxy stop         # Stop daemon
./claude-code-proxy version      # Show version
./claude-code-proxy help         # Show help
```

**Flags:**

```bash
-d, --debug     # Enable debug mode (full request/response logging)
-s, --simple    # Enable simple log mode (one-line summaries)
```

**Examples:**

```bash
# Start with debug logging
./claude-code-proxy -d

# Start with simple one-line summaries
./claude-code-proxy -s

# Combine flags
./claude-code-proxy -d -s
```

**Option 1: Use ccp wrapper (recommended)**

If you installed via `make install`, the `ccp` wrapper is already available:

```bash
# Use ccp instead of claude
ccp chat
ccp --version
ccp code /path/to/project
```

The `ccp` wrapper automatically:
- Starts the proxy daemon (if not running)
- Sets `ANTHROPIC_BASE_URL`
- Execs `claude` with your arguments

**No installation needed** - `ccp` is installed system-wide with `make install`.

**Option 2: Use with Claude Code directly**

```bash
# Start the proxy
./claude-code-proxy

# Configure Claude Code to use the proxy
export ANTHROPIC_BASE_URL=http://localhost:8082
claude chat
```

## Pattern-Based Routing

The proxy auto-detects Claude model names:

| Claude Model Pattern | Default OpenAI Model |
|---------------------|---------------------|
| `*opus*` | `gpt-5` |
| `*sonnet*` | `gpt-5` |
| `*haiku*` | `gpt-5-mini` |

Override with env vars:
```bash
ANTHROPIC_DEFAULT_OPUS_MODEL=gpt-5
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-5
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
- `OPENAI_API_KEY` - Your API key (not needed for Ollama/localhost)

**Optional - API Configuration:**
- `OPENAI_BASE_URL` - API base URL (default: `https://api.openai.com/v1`)
  - For OpenRouter: `https://openrouter.ai/api/v1`
  - For Ollama: `http://localhost:11434/v1`
  - For other providers: Use their OpenAI-compatible endpoint

**Optional - Model Routing:**
- `ANTHROPIC_DEFAULT_OPUS_MODEL` - Override opus routing (default: `gpt-5`)
- `ANTHROPIC_DEFAULT_SONNET_MODEL` - Override sonnet routing (default: `gpt-5`)
- `ANTHROPIC_DEFAULT_HAIKU_MODEL` - Override haiku routing (default: `gpt-5-mini`)

Examples with OpenRouter:
```bash
ANTHROPIC_DEFAULT_SONNET_MODEL=x-ai/grok-code-fast-1
ANTHROPIC_DEFAULT_HAIKU_MODEL=google/gemini-2.5-flash
ANTHROPIC_DEFAULT_OPUS_MODEL=openai/gpt-5
```

**Optional - OpenRouter Specific:**
- `OPENROUTER_APP_NAME` - App name for OpenRouter dashboard tracking
- `OPENROUTER_APP_URL` - App URL for better rate limits (higher quotas)

**Optional - Security:**
- `ANTHROPIC_API_KEY` - Client API key validation (optional)
  - If set, clients must provide this exact key
  - Leave unset to disable validation

**Optional - Server Settings:**
- `HOST` - Server host (default: `0.0.0.0`)
- `PORT` - Server port (default: `8082`)
- `PASSTHROUGH_MODE` - Direct proxy to Anthropic API (default: `false`)

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

## Supported Claude Code Features

The proxy fully supports all Claude Code features:

- **Tool Calling** - Complete support for all Claude Code tools
  - File operations: `read`, `write`, `edit`
  - Search operations: `glob`, `grep`
  - Shell execution: `bash`
  - Task management: `todowrite`, `todoread`
  - And all other Claude Code tools

- **Extended Thinking** - Proper thinking block support
  - Thinking blocks are properly formatted and hidden in Claude Code UI
  - Shows "Thought for Xs" indicator instead of full content
  - Can be revealed with Ctrl+O in Claude Code
  - Supports signature_delta events for authentication

- **Streaming** - Real-time streaming responses
  - Proper SSE (Server-Sent Events) formatting
  - Accurate token usage tracking
  - Low latency streaming from backend models

- **Token Tracking** - Full usage metrics
  - Input tokens counted accurately
  - Output tokens tracked in real-time
  - Cache metrics supported (when using Anthropic backend)

## Development

```bash
# Run in dev mode
go run cmd/claude-code-proxy/main.go

# Run tests
go test ./...
# Or with verbose output
go test -v ./internal/converter

# Run specific test
go test -v ./internal/converter -run TestConvertMessagesWithComplexContent

# Format code
go fmt ./...

# Lint (requires golangci-lint)
golangci-lint run
```

## Testing

The project includes comprehensive unit tests:

```bash
# Run all tests
go test ./...

# Run converter tests (includes tool calling tests)
go test -v ./internal/converter

# Run with coverage
go test -cover ./...
```

## How It Works

1. **Request Flow**:
   - Claude Code sends Claude API format request to proxy
   - Proxy converts Claude format → OpenAI format
   - Proxy routes to OpenRouter/OpenAI/other provider
   - Provider returns OpenAI format response
   - Proxy converts back to Claude format
   - Claude Code receives properly formatted response

2. **Format Conversion**:
   - Claude's `tool_use` blocks → OpenAI's `tool_calls` format
   - OpenAI's `reasoning_details` → Claude's `thinking` blocks
   - Maintains proper tool_use ↔ tool_result correspondence
   - Preserves all metadata and signatures

3. **Streaming**:
   - Converts OpenAI SSE chunks to Claude SSE events
   - Generates proper event sequence (message_start, content_block_start, deltas, etc.)
   - Tracks content block indices for proper Claude Code rendering

## License

MIT
