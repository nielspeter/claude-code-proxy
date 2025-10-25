# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Code Proxy is an HTTP proxy that translates Claude API requests to OpenAI-compatible format, enabling Claude Code to work with 200+ alternative models through OpenRouter, OpenAI Direct (o1/o3), and Ollama (local). The proxy runs as a daemon, performs bidirectional API format conversion, and maintains full Claude Code feature compatibility including tool calling, extended thinking blocks, and streaming.

## Build Commands

```bash
# Build the binary
go build -o claude-code-proxy cmd/claude-code-proxy/main.go
# Or use make
make build

# Build for all platforms (creates dist/ folder)
make build-all

# Run tests
go test ./...

# Run specific test file
go test -v ./internal/converter

# Run single test
go test -v ./internal/converter -run TestConvertMessagesWithComplexContent

# Run tests with coverage
make test-coverage

# Format code
go fmt ./...

# Compile and start proxy in simple log mode
go build -o claude-code-proxy cmd/claude-code-proxy/main.go && ./claude-code-proxy -s
```

## Architecture

### Core Request Flow

1. **Claude Code** → sends Claude API format request to `localhost:8082`
2. **handlers.go** → receives `/v1/messages` POST request
3. **converter.go** → transforms Claude format → OpenAI format
   - Detects provider type (OpenRouter/OpenAI/Ollama) via `cfg.DetectProvider()`
   - Applies provider-specific parameters (reasoning format, tool_choice)
   - Maps Claude model name to target provider model using pattern-based routing
4. **handlers.go** → forwards OpenAI request to configured provider
5. **Provider** → returns OpenAI-format response (streaming or non-streaming)
6. **converter.go** → transforms OpenAI format → Claude format
7. **handlers.go** → returns Claude-format response to Claude Code

### Provider-Specific Behavior

The proxy applies different request parameters based on `OPENAI_BASE_URL`:

**OpenRouter** (`https://openrouter.ai/api/v1`):
- Adds `reasoning: {enabled: true}` for thinking support
- Uses `usage: {include: true}` for token tracking
- Extracts `reasoning_details` array → converts to Claude `thinking` blocks

**OpenAI Direct** (`https://api.openai.com/v1`):
- Adds `reasoning_effort: "medium"` for GPT-5 reasoning models
- Uses standard `stream_options: {include_usage: true}`

**Ollama** (`http://localhost:*`):
- Sets `tool_choice: "required"` when tools are present (forces tool usage)
- No API key validation (localhost endpoints skip auth)

### Format Conversion Details

**Tool Calling** (`convertMessages` in converter.go):
- Claude `tool_use` content blocks → OpenAI `tool_calls` array
- OpenAI `tool_calls` → Claude `tool_use` blocks
- Maintains `tool_use.id` ↔ `tool_result.tool_use_id` correspondence
- Preserves JSON arguments as strings during conversion

**Thinking Blocks** (`ConvertResponse` in converter.go):
- OpenRouter `reasoning_details` → Claude `thinking` block with `signature` field
- `signature` field is REQUIRED for Claude Code to hide/show thinking properly
- Without signature, thinking appears as regular text in chat

**Streaming** (`streamOpenAIToClaude` in handlers.go):
- Converts OpenAI SSE chunks (`data: {...}`) → Claude SSE events
- Generates proper event sequence: `message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop`
- Tracks content block indices to maintain proper ordering
- Handles tool call deltas by accumulating function arguments across chunks

### Pattern-Based Model Routing

The `mapModel()` function in converter.go implements intelligent routing:

```go
// Haiku tier → lightweight models
"*haiku*" → gpt-5-mini (or ANTHROPIC_DEFAULT_HAIKU_MODEL)

// Sonnet tier → version-aware
"*sonnet-4*" or "*sonnet-5*" → gpt-5
"*sonnet-3*" → gpt-4o
(or ANTHROPIC_DEFAULT_SONNET_MODEL)

// Opus tier → flagship models
"*opus*" → gpt-5 (or ANTHROPIC_DEFAULT_OPUS_MODEL)
```

Override via environment variables to route to alternative models (Grok, Gemini, DeepSeek-R1, etc.).

## Configuration System

Config loading priority (see `internal/config/config.go`):
1. `./.env` (local project override)
2. `~/.claude/proxy.env` (recommended location)
3. `~/.claude-code-proxy` (legacy location)

Uses `godotenv.Overload()` to allow later files to override earlier ones.

Provider detection via URL pattern matching in `DetectProvider()`:
- Contains `openrouter.ai` → ProviderOpenRouter
- Contains `api.openai.com` → ProviderOpenAI
- Contains `localhost` or `127.0.0.1` → ProviderOllama
- Otherwise → ProviderUnknown

## Testing Strategy

The test suite has two main categories:

**Provider Tests** (`internal/converter/provider_test.go`):
- Verify provider-specific request parameters are correct
- Ensure OpenRouter gets `reasoning: {enabled: true}` not `reasoning_effort`
- Ensure OpenAI Direct gets `reasoning_effort` not `reasoning` object
- Ensure Ollama gets `tool_choice: "required"` when tools present
- Test provider isolation (no cross-contamination of parameters)

**Conversion Tests** (`internal/converter/converter_test.go`):
- Test Claude → OpenAI message conversion
- Test tool calling format conversion
- Test thinking block extraction from reasoning_details
- Test streaming chunk aggregation

When adding new provider support, create tests in `provider_test.go` following the existing pattern.

## Simple Log Mode

The `-s` or `--simple` flag enables one-line request summaries:

```
[REQ] <base_url> model=<provider_model> in=<tokens> out=<tokens> tok/s=<rate>
```

Implementation:
- Track `startTime := time.Now()` at request start
- Extract token counts from response usage data
- Calculate throughput: `tokensPerSec = float64(outputTokens) / duration`
- Output in both streaming (`streamOpenAIToClaude`) and non-streaming handlers

Token extraction requires `float64 → int` conversion because JSON unmarshals numbers as float64.

## Common Pitfalls

1. **Tool arguments must be strings**: OpenAI expects `arguments: "{\"key\":\"value\"}"` not `arguments: {key: "value"}`

2. **Thinking blocks need signature field**: Without `signature: "..."` field, Claude Code shows thinking as plain text instead of hiding it

3. **Provider parameter isolation**: Never mix OpenRouter `reasoning` object with OpenAI `reasoning_effort` parameter - detection logic in `ConvertRequest()` ensures this

4. **Streaming index tracking**: Content blocks must maintain consistent indices across SSE events - use state struct to track current index

5. **Token count type conversion**: Always convert JSON number types to int when extracting from maps: `int(val.(float64))`

## Daemon Process

The proxy runs as a background daemon (see `internal/daemon/daemon.go`):
- Creates PID file at `/tmp/claude-code-proxy.pid`
- Redirects stdout/stderr to `/tmp/claude-code-proxy.log`
- `./claude-code-proxy status` checks if process is running
- `./claude-code-proxy stop` kills the daemon via PID file

When testing locally, use `-d` flag for debug logging to see full requests/responses.

## Package Structure

- `cmd/claude-code-proxy/main.go` - Entry point, CLI arg parsing
- `internal/config/` - Environment variable loading, provider detection
- `internal/converter/` - Claude ↔ OpenAI format conversion logic
- `internal/server/` - HTTP server (Fiber), request handlers, streaming
- `internal/daemon/` - Process management, PID file handling
- `pkg/models/` - Shared type definitions for Claude and OpenAI formats
- `scripts/ccp` - Wrapper script that starts daemon and execs Claude Code

## Key Files

- `internal/converter/converter.go:ConvertRequest()` - Claude → OpenAI request conversion with provider-specific parameters
- `internal/converter/converter.go:ConvertResponse()` - OpenAI → Claude response conversion, thinking block extraction
- `internal/server/handlers.go:streamOpenAIToClaude()` - SSE chunk conversion, event generation
- `internal/config/config.go:DetectProvider()` - URL-based provider detection
- `pkg/models/types.go` - All request/response type definitions
