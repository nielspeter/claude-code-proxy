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

**Provider Interface** (Detailed Specification):
```go
type Provider interface {
    // Configuration - static provider information
    Name() string  // "openrouter", "openai", "ollama"
    DetectFromURL(baseURL string) bool  // Does this URL match our provider?

    // Capabilities - what features does this provider support
    SupportsStreaming() bool    // Does provider support streaming responses?
    SupportsReasoning() bool    // Does provider support reasoning/thinking?
    StreamingFormat() string    // "openai" | "openrouter" | "ollama"

    // Request Handling - modify request before sending to provider
    // NOTE: Message/tool conversion happens in converter/ (generic)
    // NOTE: Model routing happens in converter/ (generic)
    // Provider only handles provider-specific request details

    RequestHeaders(cfg *config.Config) map[string]string
        // Return map of headers to add (auth, app name, etc)
        // Example: OpenRouter adds X-Title header for app tracking

    RequestParameters(cfg *config.Config) map[string]interface{}
        // Return provider-specific parameters to add to request body
        // Example: OpenRouter adds reasoning:{enabled:true}
        // Example: OpenAI adds reasoning_effort:"medium"
        // Example: Ollama adds tool_choice:"required"

    // Response Handling - extract provider-specific data from response
    // NOTE: Message/tool conversion happens in converter/ (generic)
    // NOTE: Thinking block extraction happens in converter/ (generic)
    // Provider only handles provider-specific response details

    ExtractTokens(resp *OpenAIResponse) *Usage
        // Extract token counts (providers format differently)
        // Returns InputTokens, OutputTokens

    HandleStreamingChunk(chunk []byte) (interface{}, error)
        // Parse provider-specific streaming format
        // Ollama might be different than OpenAI standard
        // Return parsed chunk or error
}
```

**Key Design Decisions**:
1. **Converter is Generic**: Message conversion, tool conversion, thinking extraction all stay in converter/ (provider-agnostic)
2. **Provider is Specific**: Only request headers, parameters, response parsing
3. **Clean Separation**: Converter doesn't know about providers. Providers don't modify messages/tools
4. **No Circular Logic**: Handlers call provider methods, NOT vice versa

**Benefits**:
- Add new provider by implementing 5-6 methods
- No changes to converter (message/tool logic)
- No changes to handlers (detection/calling logic stays same)
- Easy to test - mock provider with test_provider.go

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

### 3. Extract Middleware (New: `server/middleware/`)

**Purpose**: Extract reusable middleware from handlers into separate modules

**Structure**:
```
middleware/
├── auth.go              # API key validation middleware
├── logging.go           # Debug & simple logging middleware
└── errors.go            # Error response formatting
```

**Responsibilities**:
- `auth.go`: Validate x-api-key header against configured key
- `logging.go`: Log requests/responses in debug mode or simple log mode
- `errors.go`: Format error responses consistently

**Benefits**:
- Reusable across all endpoints
- Keeps handlers clean
- Easier to test in isolation
- Follows middleware pattern

### 4. Split Handlers into Logical Modules (Refactor: `server/handlers/`)

**Purpose**: Break down 878-line handlers.go into focused endpoints

**Structure**:
```
handlers/
├── messages.go          # Main /v1/messages endpoint (uses provider, converter, streaming)
├── tokens.go            # /v1/messages (tokens) endpoint
└── health.go            # /health endpoint
```

**Responsibilities**:
- `messages.go`: Request parsing, provider selection, converter call, response/streaming
- `tokens.go`: Token counting endpoint (parse request, call provider, return usage)
- `health.go`: Health check endpoint (no logic needed)

**Benefits**:
- Each handler is small and focused (<100 lines)
- Easier to find and modify specific endpoints
- Streaming logic extracted separately (see below)
- Middleware handles cross-cutting concerns

### 5. Extract Streaming to Separate Package (New: `server/streaming/`)

**Purpose**: Isolate complex streaming logic from request handlers

**Structure**:
```
streaming/
├── converter.go         # Orchestrates OpenAI chunks → Claude SSE events
└── sse.go              # SSE event formatting utilities
```

**Responsibilities**:
- `converter.go`: Main orchestrator that parses OpenAI SSE chunks and generates Claude SSE events
- `sse.go`: Low-level utilities for formatting/writing SSE events (message_start, content_block_delta, etc)

**Important**: Provider-specific chunk parsing stays in `providers/{openrouter,openai,ollama}.go` via the `HandleStreamingChunk()` interface method

**Benefits**:
- Streaming logic isolated from request handling
- Reusable SSE formatting utilities
- Easier to test without HTTP context
- Providers handle their own chunk parsing

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

## Architecture Rules & Constraints

### Import Rules (Enforce These to Prevent Circular Dependencies)

```
✅ ALLOWED IMPORTS:
- handlers/ → config, converter, providers, streaming, middleware
- converter/ → config only (NOT providers, NOT handlers)
- providers/ → config only (NOT converter, NOT handlers, NOT streaming)
- streaming/ → config, converter (NOT providers, NOT handlers)
- middleware/ → config only (NOT anything else)

❌ FORBIDDEN IMPORTS:
- converter imports providers (would create circular dependency)
- providers imports converter (provider methods are simple, don't convert)
- handlers imports streaming directly (use it via handlers, not external)
- middleware imports handlers (wrong direction)
- Any package imports cmd/

⚠️ CRITICAL: If you need to import something not in the allowed list, you've
probably broken separation of concerns. Refactor instead of adding imports.
```

### Responsibility Boundaries

```
converter/: Handles FORMAT conversion (Claude ↔ OpenAI, not provider logic)
providers/: Handles PROVIDER QUIRKS (headers, parameters, token extraction)
handlers/: Handles HTTP REQUESTS (parsing, routing, error handling)
streaming/: Handles SSE OUTPUT (event generation, chunk conversion)
middleware/: Handles CROSS-CUTTING CONCERNS (auth, logging)
config/:   Handles ENVIRONMENT CONFIGURATION (no logic)
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
│   │   ├── auth.go         (NEW: API key validation)
│   │   ├── auth_test.go    (NEW)
│   │   ├── logging.go      (NEW: debug/simple logging)
│   │   ├── logging_test.go (NEW)
│   │   ├── errors.go       (NEW: error response formatting)
│   │   └── errors_test.go  (NEW)
│   ├── handlers/           (NEW)
│   │   ├── messages.go     (NEW: /v1/messages handler)
│   │   ├── messages_test.go (NEW)
│   │   ├── tokens.go       (NEW: /v1/messages?tokens endpoint)
│   │   ├── tokens_test.go  (NEW)
│   │   ├── health.go       (NEW: /health endpoint)
│   │   └── health_test.go  (NEW)
│   ├── streaming/          (NEW)
│   │   ├── converter.go    (NEW: SSE conversion orchestrator)
│   │   ├── converter_test.go (NEW)
│   │   ├── sse.go          (NEW: SSE formatting utilities)
│   │   ├── sse_test.go     (NEW)
│   │   └── streaming_test.go (NEW: integration tests)
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

### Mocking Approach

**Key Testing Principle**: Don't test HTTP calls or file I/O. Mock them.

```go
// providers/test_provider.go (use in all handler tests)
type MockProvider struct {
    name              string
    supportsStreaming bool
    supportsReasoning bool
}

func (p *MockProvider) Name() string { return p.name }
func (p *MockProvider) DetectFromURL(url string) bool { return true }
func (p *MockProvider) SupportsStreaming() bool { return p.supportsStreaming }
func (p *MockProvider) SupportsReasoning() bool { return p.supportsReasoning }
func (p *MockProvider) RequestHeaders(cfg *config.Config) map[string]string { return map[string]string{} }
func (p *MockProvider) RequestParameters(cfg *config.Config) map[string]interface{} { return map[string]interface{}{} }
func (p *MockProvider) ExtractTokens(resp *OpenAIResponse) *Usage { return &Usage{} }
func (p *MockProvider) HandleStreamingChunk(chunk []byte) (interface{}, error) { return nil, nil }
```

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

**Timeline**: 4-6 hours (with thorough testing at each step)

**Detailed Tasks** (In This Order):
1. **Create Provider Interface** (30 min)
   - Create `providers/provider.go` with detailed interface
   - Add detailed comments on each method
   - Create `providers/registry.go` stub (no implementations yet)
   - Create `providers/test_provider.go` for mocking
   - Tests: `providers/provider_test.go` (interface contract tests)

2. **Extract Middleware** (45 min)
   - Create `middleware/auth.go` (API key validation)
   - Create `middleware/logging.go` (debug/simple logging)
   - Create `middleware/errors.go` (error formatting)
   - Move logic from current handlers.go
   - Tests: `middleware/*_test.go` for each module
   - **Run tests**: `go test ./internal/server/middleware`

3. **Split Converter** (60 min)
   - Create `converter/models.go` (model routing + constants)
   - Create `converter/messages.go` (message conversion)
   - Create `converter/tools.go` (tool conversion)
   - Create `converter/reasoning.go` (thinking block extraction)
   - Update `converter/converter.go` as orchestrator
   - Move tests to corresponding `*_test.go` files
   - **Run tests**: `go test ./internal/converter`
   - **Verify coverage**: Should be >= 82%

4. **Split Handlers** (60 min)
   - Create `handlers/messages.go` (main /v1/messages endpoint)
   - Create `handlers/tokens.go` (/v1/messages?tokens endpoint)
   - Create `handlers/health.go` (/health endpoint)
   - Remove original logic from handlers.go
   - Update to use middleware and provider interface
   - Tests: `handlers/*_test.go` for each handler (use MockProvider)
   - **Run tests**: `go test ./internal/server/handlers`

5. **Extract Streaming** (45 min)
   - Create `streaming/converter.go` (orchestrator)
   - Create `streaming/sse.go` (utilities)
   - Move streaming logic from handlers.go
   - Tests: `streaming/*_test.go`
   - **Run tests**: `go test ./internal/server/streaming`

6. **Final Verification** (30 min)
   - Update all imports across packages
   - Verify import rules are followed
   - **Run full test suite**: `go test ./...`
   - **Check coverage**: `go test -cover ./...`
   - **Lint**: `golangci-lint run`
   - **Build**: `go build ./...`

**Deliverables**:
- Modular architecture with single-responsibility files (all < 200 lines)
- Provider interface ready for Phase 2
- All tests passing (0 regressions)
- Coverage maintained/improved (converter: 82%+, config: 97%+, daemon: 59%+)
- Import rules followed (no circular dependencies)
- PR with detailed commit messages explaining each refactoring step

### Phase 2: Provider Abstraction (Next PR)
**Goal**: Implement provider interface for existing providers, remove provider-specific code from generic modules

**Timeline**: 2-3 hours

**Detailed Tasks**:
1. Implement `providers/openrouter.go`
   - RequestHeaders: Add X-Title (app name), x-title (app url)
   - RequestParameters: Add reasoning:{enabled:true}, usage:{include:true}
   - ExtractTokens: Extract from response
   - HandleStreamingChunk: Parse OpenRouter SSE format
   - Tests: `providers/openrouter_test.go`

2. Implement `providers/openai.go`
   - RequestHeaders: Add standard auth
   - RequestParameters: Add reasoning_effort:"medium"
   - ExtractTokens: Extract from standard format
   - HandleStreamingChunk: Parse standard OpenAI format
   - Tests: `providers/openai_test.go`

3. Implement `providers/ollama.go`
   - RequestHeaders: No auth needed (local)
   - RequestParameters: Add tool_choice:"required" when tools present
   - ExtractTokens: Handle Ollama format (may be different)
   - HandleStreamingChunk: Parse Ollama SSE format
   - Tests: `providers/ollama_test.go`

4. Update `providers/registry.go`
   - Implement detection logic (call each provider's DetectFromURL)
   - Register implementations
   - Return appropriate provider based on baseURL

5. Update handlers
   - Replace hardcoded provider checks with provider.RequestHeaders/Parameters
   - Replace response parsing with provider.ExtractTokens
   - Tests should still pass (using MockProvider)

6. Remove provider-specific code from `converter/`
   - No more openrouter-specific reasoning handling in converter
   - No more ollama-specific tool_choice in converter
   - Converter stays generic

7. Integration tests
   - Test full flow: Claude request → provider → conversion → response
   - Test all three providers with MockProvider first
   - **Run full test suite**: `go test ./...`

**Deliverables**:
- Provider-agnostic core modules (converter, handlers, middleware)
- Clean provider implementations (each is 100-150 lines)
- Easy to add new providers (just implement interface)
- Full test coverage (each provider tested separately)

### Phase 3: Provider Addition Example (Future PR - Anthropic Direct)
**Goal**: Demonstrate that new providers can be added with single file

**Task**: Add Anthropic API Direct support
- Create `providers/anthropic.go` (implement interface)
- Create `providers/anthropic_test.go`
- Update `providers/registry.go` to register
- **Result**: 0 changes to handlers, converter, middleware, streaming
- Demonstrates extensibility of architecture

**Deliverables**:
- Proven provider extensibility
- Example of adding new provider (for future contributors)

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
