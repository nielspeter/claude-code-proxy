# Refactoring Plan: Provider-Extensible Architecture

## Status
- **Branch**: `refactor/split-large-files`
- **Phase**: 1 (Core Refactoring)
- **Goal**: Make system provider-agnostic and easily extensible

## Problem Statement

### Current Issues
- **handlers.go (878 lines)**: Mixing streaming, SSE, API calls, request handling
- **converter.go (418 lines)**: Generic + provider-specific logic intertwined
- **Provider logic scattered**: OpenRouter/OpenAI/Ollama handling spread across files
- **Hard to add providers**: Adding new provider requires modifying multiple large files
- **Difficult to maintain**: Single responsibility principle violated

### Current File Sizes
```
878 lines - internal/server/handlers.go
418 lines - internal/converter/converter.go
155 lines - internal/config/config.go
149 lines - internal/server/server.go
```

## Proposed Architecture: Plugin-Based Provider System

### Design Principles
1. **Single Responsibility**: Each file handles one concern
2. **Open/Closed Principle**: Open for extension (new providers), closed for modification
3. **Provider Isolation**: Each provider's quirks stay in its own file
4. **Easy Testing**: Provider interface enables mocking
5. **Future-Proof**: Room for new providers (Claude Direct, local models, custom)

## Architecture Layers

### 1. Provider Abstraction Layer (New: `providers/` package)

**Purpose**: Abstract provider-specific behavior into pluggable implementations

**Structure**:
```
providers/
├── provider.go          # Provider interface definition
├── registry.go          # Provider factory and registration
├── openrouter.go        # OpenRouter implementation
├── openai.go            # OpenAI Direct implementation
└── ollama.go            # Ollama implementation
```

**Provider Interface**:
```go
type Provider interface {
    Name() string
    DetectFromURL(baseURL string) bool
    PrepareRequest(req *OpenAIRequest) error
    ParseResponse(resp *OpenAIResponse) error
    SupportsStreaming() bool
    SupportsReasoning() bool
    StreamingFormat() string  // "openai", "openrouter", etc
}
```

**Benefits**:
- Add new provider by creating single file implementing interface
- No changes to main handlers needed
- Each provider can have custom request/response logic
- Reasoning, tool_choice, stream options all provider-specific

### 2. Split Converter into Logical Modules (Refactor: `converter/`)

**Purpose**: Break down 418-line converter into focused, testable modules

**Structure**:
```
converter/
├── converter.go         # Main orchestrator (~50 lines)
├── models.go            # Model mapping and routing
├── messages.go          # Claude ↔ OpenAI message conversion
├── tools.go             # Tool/function conversion
├── reasoning.go         # Thinking block extraction and formatting
└── finish_reason.go     # Finish reason mapping
```

**Responsibilities**:
- `models.go`: Pattern-based routing, constants (opus/sonnet/haiku defaults)
- `messages.go`: Convert Claude messages to OpenAI format and vice versa
- `tools.go`: Convert tool definitions between formats
- `reasoning.go`: Extract reasoning_details → thinking blocks with signature
- `finish_reason.go`: Map OpenAI stop reasons to Claude stop reasons

**Benefits**:
- Each converter handles one concern
- Easy to override per-provider if needed
- Models centralized for routing logic
- Easier to test each converter independently
- Better code organization and readability

### 3. Split Handlers into Logical Modules (Refactor: `server/handlers/`)

**Purpose**: Break down 878-line handlers.go into focused endpoints

**Structure**:
```
handlers/
├── messages.go          # Main /v1/messages endpoint
├── tokens.go            # /v1/messages (tokens) endpoint
├── health.go            # /health endpoint
└── middleware.go        # API key validation, debug logging
```

**Responsibilities**:
- `messages.go`: Request parsing, validation, provider selection, response handling
- `tokens.go`: Token counting endpoint (separate concern)
- `health.go`: Health check endpoint
- `middleware.go`: API key validation, request/response logging

**Benefits**:
- Each handler is small and focused
- Easier to find and modify specific endpoints
- Streaming logic extracted separately (see below)

### 4. Extract Streaming to Separate Package (New: `server/streaming/`)

**Purpose**: Isolate complex streaming logic from request handlers

**Structure**:
```
streaming/
├── adapter.go           # Generic streaming converter
├── sse_writer.go        # SSE event formatting
└── parser.go            # Parse provider-specific streaming formats
```

**Responsibilities**:
- `adapter.go`: Orchestrates SSE event generation from provider chunks
- `sse_writer.go`: Format Claude SSE events (message_start, content_block_start, etc)
- `parser.go`: Parse OpenAI SSE format → Claude format

**Benefits**:
- Streaming logic isolated from request handling
- Reusable SSE utilities
- Easier to test streaming without full HTTP context
- Provider-specific streaming formats handled in provider implementations

### 5. Request/Response Pipeline (Existing, no changes needed)

Current flow will still work:
```
Claude Request
    ↓
handlers/messages.go (validates, logs)
    ↓
Provider (selected based on config)
    ↓
converter/{messages,tools,reasoning}.go (format conversion)
    ↓
HTTP call to provider API
    ↓
converter/{messages,tools,reasoning}.go (response conversion)
    ↓
streaming/ or handlers/ (format output)
    ↓
Claude Response
```

## File Organization After Refactoring

### Before (Current State)
```
internal/
├── config/
│   ├── config.go
│   └── config_test.go
├── converter/
│   ├── converter.go        (418 lines - monolithic)
│   ├── converter_test.go
│   └── provider_test.go
├── server/
│   ├── server.go
│   ├── handlers.go         (878 lines - monolithic)
│   └── handlers_test.go
├── daemon/
│   ├── daemon.go
│   └── daemon_test.go
└── cmd/
```

### After (Refactored)
```
internal/
├── config/
│   ├── config.go           (no change)
│   └── config_test.go      (no change)
├── converter/              (REFACTORED)
│   ├── converter.go        (orchestrator, ~50 lines)
│   ├── models.go           (NEW: model routing)
│   ├── messages.go         (NEW: message conversion)
│   ├── tools.go            (NEW: tool conversion)
│   ├── reasoning.go        (NEW: thinking blocks)
│   ├── finish_reason.go    (NEW: reason mapping)
│   ├── converter_test.go   (updated)
│   ├── models_test.go      (NEW)
│   ├── messages_test.go    (NEW)
│   ├── tools_test.go       (NEW)
│   ├── reasoning_test.go   (NEW)
│   └── provider_test.go    (updated)
├── providers/              (NEW PACKAGE)
│   ├── provider.go         (interface definition)
│   ├── registry.go         (factory/registration)
│   ├── openrouter.go       (future: provider impl)
│   ├── openai.go           (future: provider impl)
│   ├── ollama.go           (future: provider impl)
│   └── provider_test.go    (NEW)
├── server/                 (REFACTORED)
│   ├── server.go           (no change)
│   ├── middleware/         (NEW)
│   │   ├── auth.go         (API key validation)
│   │   └── logging.go      (debug/simple logging)
│   ├── handlers/           (NEW)
│   │   ├── messages.go     (NEW: /v1/messages handler)
│   │   ├── tokens.go       (NEW: /v1/messages?tokens endpoint)
│   │   ├── health.go       (NEW: /health endpoint)
│   │   └── handlers_test.go (updated)
│   ├── streaming/          (NEW)
│   │   ├── adapter.go      (NEW: SSE conversion)
│   │   ├── sse_writer.go   (NEW: SSE formatting)
│   │   ├── parser.go       (NEW: response parsing)
│   │   └── streaming_test.go (NEW)
│   └── server_test.go      (updated)
├── daemon/                 (no change)
│   ├── daemon.go
│   └── daemon_test.go
└── cmd/
```

## Adding a New Provider: Before vs After

### Before Refactoring
To add a new provider (e.g., Anthropic Direct), you would need to:
1. Modify `converter.go` - add provider-specific request/response logic
2. Modify `handlers.go` - handle provider-specific streaming format
3. Modify `config.go` - add detection logic for new provider
4. Modify `server.go` - register new routes/handlers
5. Add tests in multiple `_test.go` files

**Problem**: Changes scattered across multiple large files, easy to miss edge cases.

### After Refactoring
To add a new provider, just create ONE file:

```go
// providers/anthropic.go
package providers

type AnthropicProvider struct{}

func (p *AnthropicProvider) Name() string {
    return "anthropic"
}

func (p *AnthropicProvider) DetectFromURL(baseURL string) bool {
    return strings.Contains(baseURL, "api.anthropic.com")
}

func (p *AnthropicProvider) PrepareRequest(req *OpenAIRequest) error {
    // Add Anthropic-specific headers, parameters, etc
    return nil
}

func (p *AnthropicProvider) ParseResponse(resp *OpenAIResponse) error {
    // Handle Anthropic-specific response fields
    return nil
}

func (p *AnthropicProvider) SupportsStreaming() bool {
    return true
}

func (p *AnthropicProvider) SupportsReasoning() bool {
    return false
}

func (p *AnthropicProvider) StreamingFormat() string {
    return "anthropic"
}
```

Then register in `providers/registry.go`:
```go
registry.Register("anthropic", &AnthropicProvider{})
```

**Done!** No other files need changes. The handler automatically picks up the new provider.

## Testing Strategy

### Test Files Before
```
converter_test.go         (all converter tests)
converter/provider_test.go (provider-specific tests)
handlers_test.go          (all handler tests)
daemon_test.go
config_test.go
```

### Test Files After
```
converter/
├── converter_test.go     (orchestrator tests)
├── models_test.go        (model routing tests)
├── messages_test.go      (message conversion tests)
├── tools_test.go         (tool conversion tests)
├── reasoning_test.go     (thinking block tests)
└── provider_test.go      (provider interface contract)

handlers/
├── handlers_test.go      (endpoint handler tests)
├── messages_test.go      (messages endpoint tests)
├── tokens_test.go        (token endpoint tests)
└── health_test.go        (health endpoint tests)

streaming/
├── streaming_test.go     (overall streaming)
├── adapter_test.go       (SSE conversion)
├── sse_writer_test.go    (SSE formatting)
└── parser_test.go        (response parsing)

providers/
└── provider_test.go      (interface contract tests)
```

### Test Verification Points
Each refactoring step will verify:
- ✅ Unit tests pass: `go test ./...`
- ✅ No test regressions
- ✅ Coverage maintained/improved
- ✅ Integration tests still pass

### Target Coverage
| Package | Target | Current |
|---------|--------|---------|
| converter | >= 82% | 82.4% |
| config | >= 97% | 97.0% |
| daemon | >= 59% | 59.2% |
| server | >= 50% | 0% (handler tests) |
| providers | >= 80% | TBD |

## Implementation Phases

### Phase 1: Core Refactoring (This PR)
**Goal**: Split large files into focused modules, establish provider interface

Tasks:
1. ✅ Split `converter.go` into `{models, messages, tools, reasoning}.go`
2. ✅ Create provider interface stub in `providers/provider.go`
3. ✅ Split `handlers.go` into `handlers/{messages, tokens, health}.go`
4. ✅ Create `streaming/` package with SSE utilities
5. ✅ Create `middleware/` package for auth/logging
6. ✅ Update all imports
7. ✅ Run full test suite - verify coverage maintained
8. ✅ Commit with message documenting refactoring

**Deliverables**:
- Modular architecture with single-responsibility files
- All tests passing
- Coverage maintained at current levels
- PR with detailed commit messages

### Phase 2: Provider Abstraction (Next PR)
**Goal**: Implement provider interface for existing providers, remove provider-specific code from generic modules

Tasks:
1. Implement `providers/openrouter.go` with OpenRouter-specific logic
2. Implement `providers/openai.go` with OpenAI-specific logic
3. Implement `providers/ollama.go` with Ollama-specific logic
4. Create `providers/registry.go` to detect and register providers
5. Update handlers to use provider registry instead of direct provider checks
6. Move provider-specific request/response logic into provider implementations
7. Remove provider-specific code from `converter/`
8. Update tests for provider implementations
9. Integration tests with all three providers

**Deliverables**:
- Provider-agnostic core modules
- Clean provider implementations
- Easy to add new providers
- Full test coverage for each provider

### Phase 3: Middleware & Utilities (Future PR)
**Goal**: Extract middleware into separate modules

Tasks:
1. Create `middleware/auth.go` for API key validation
2. Create `middleware/logging.go` for debug/simple logging
3. Create `middleware/error_handler.go` for error responses
4. Update tests

**Deliverables**:
- Reusable middleware components
- Cleaner request handler code

## Key Design Decisions

### 1. Provider Detection
- **Before**: Hardcoded if/else in converter
- **After**: Provider interface with `DetectFromURL()` method
- **Benefit**: Easy to add detection logic per provider

### 2. Request/Response Conversion
- **Before**: Generic conversion + provider-specific cases
- **After**: Generic conversion + provider-specific overrides via interface
- **Benefit**: Generic path works for most, overrides for exceptions

### 3. Streaming Format
- **Before**: Provider checks in `streamOpenAIToClaude()`
- **After**: Each provider declares format, `streaming/parser.go` handles conversion
- **Benefit**: Centralized streaming logic, provider-specific parsing

### 4. File Organization
- **Before**: By concern (handlers, converter, etc) with monolithic files
- **After**: By concern AND responsibility (handlers/messages, handlers/tokens, converter/models, etc)
- **Benefit**: Find code faster, easier to navigate

## Migration Timeline

### Commit 1: Extract Converter Modules
```
- Split converter.go into {models, messages, tools, reasoning}.go
- Move tests to corresponding _test.go files
- Update imports in all files
- Verify coverage: 82%+
```

### Commit 2: Extract Handler Modules
```
- Split handlers.go into handlers/{messages, tokens, health}.go
- Extract middleware into middleware/{auth, logging}.go
- Move tests to corresponding _test.go files
- Verify coverage: 50%+
```

### Commit 3: Extract Streaming
```
- Create streaming/{adapter, sse_writer, parser}.go
- Move streaming tests
- Verify coverage: 70%+
```

### Commit 4: Provider Interface
```
- Create providers/provider.go interface
- Create providers/registry.go (stub, no implementations)
- Verify coverage: 80%+
```

### Commit 5: Update Imports & Verify
```
- Final import fixes
- Run full test suite: go test ./...
- Verify no regressions
- Check overall coverage
```

## Rollback Plan

If any step breaks tests or causes regressions:
1. Identify which commit caused issue: `git bisect`
2. Reset to previous working state: `git reset --soft HEAD~1`
3. Fix the issue
4. Recommit with corrections

All work is on `refactor/split-large-files` branch, so main remains stable.

## Success Criteria

- ✅ All tests pass: `go test ./...`
- ✅ No test regressions
- ✅ Coverage maintained at current levels or improved
- ✅ Code compiles: `go build ./...`
- ✅ Lints clean: `golangci-lint run`
- ✅ Each file <= 250 lines (readability threshold)
- ✅ Clear commit history with descriptive messages
- ✅ Easy to understand file organization
- ✅ Provider interface ready for phase 2

## Future Work

### Phase 2+ Opportunities
- Claude API Direct support (when Claude APIs add streaming)
- LiteLLM proxy support (additional 200+ models)
- vLLM local inference
- LocalAI support
- Custom provider via plugins/WASM
- Provider-specific rate limiting
- Provider health checks and failover
- Provider cost tracking (API calls × price per model)

All of these become single-file additions once architecture is in place!

## Questions & Notes

### Q: What about backward compatibility?
**A**: Full backward compatibility. Configuration and APIs remain identical. Internal refactoring only.

### Q: Will this slow down the proxy?
**A**: No. Refactoring is purely structural. Same operations, same order, same performance.

### Q: What if tests fail during refactoring?
**A**: We run tests after each major step. If tests fail, we fix before continuing.

### Q: How long will this take?
**A**: Phase 1 (refactoring): 2-4 hours with proper testing
       Phase 2 (provider abstraction): 2-3 hours

### Q: Can I still use the proxy while refactoring?
**A**: Changes are on `refactor/split-large-files` branch. `main` remains stable.
