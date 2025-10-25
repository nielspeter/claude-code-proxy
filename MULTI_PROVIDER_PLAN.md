# Multi-Provider Support Plan
## OpenRouter, OpenAI Direct, and Ollama

**Status**: Planning Phase
**Goal**: Support three major provider types with minimal code changes
**Philosophy**: Single codebase, smart auto-detection, provider-specific optimizations

---

## Current State Analysis

### What We Have ✅

**Core Architecture** (Provider-Agnostic):
- Claude API → OpenAI API format conversion (`internal/converter`)
- OpenAI API → Claude API format streaming (`internal/server/handlers.go`)
- Pattern-based model routing (opus/sonnet/haiku → backend models)
- Configuration system via environment variables

**OpenRouter Support** (Primary, Production-Ready):
- ✅ Works perfectly with `OPENAI_BASE_URL=https://openrouter.ai/api/v1`
- ✅ Handles `reasoning_details` array format correctly
- ✅ Supports 200+ models through single endpoint
- ✅ Tool calling works
- ✅ Extended thinking works
- ⚠️ Missing: Optional headers for better rate limits

**OpenAI Direct** (Untested, Should Work):
- ✅ Code supports standard OpenAI format
- ❓ Unknown: Does o1/o3 reasoning format match our parser?
- ❓ Unknown: Does `reasoning_content` (OpenAI) vs `reasoning_details` (OpenRouter) work?
- ❌ No documentation or examples

**Ollama** (Blocked, Needs Changes):
- ❌ Requires `OPENAI_API_KEY` but Ollama has no auth
- ❌ No Ollama-specific model routing examples
- ❓ Unknown: Does DeepSeek-R1 reasoning work locally?
- ❌ No documentation

---

## Technical Requirements by Provider

### 1. OpenRouter (Enhancement)

**What Works**:
```bash
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=sk-or-v1-xxx
ANTHROPIC_DEFAULT_SONNET_MODEL=x-ai/grok-code-fast-1
```

**What's Missing**:
- Optional headers for better rate limits and dashboard visibility

**Implementation**:

File: `internal/config/config.go`
```go
type Config struct {
    // ... existing fields ...

    // OpenRouter-specific (optional)
    OpenRouterAppName string
    OpenRouterAppURL  string
}

// In Load():
OpenRouterAppName: os.Getenv("OPENROUTER_APP_NAME"),
OpenRouterAppURL:  os.Getenv("OPENROUTER_APP_URL"),
```

File: `internal/server/handlers.go:184-185` (streaming) and `handlers.go:713-714` (non-streaming)
```go
// Set headers
httpReq.Header.Set("Content-Type", "application/json")
httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)

// OpenRouter-specific headers (optional, improves rate limits)
if strings.Contains(cfg.OpenAIBaseURL, "openrouter.ai") {
    if cfg.OpenRouterAppURL != "" {
        httpReq.Header.Set("HTTP-Referer", cfg.OpenRouterAppURL)
    }
    if cfg.OpenRouterAppName != "" {
        httpReq.Header.Set("X-Title", cfg.OpenRouterAppName)
    }
}
```

**Configuration Example**:
```bash
# OpenRouter with optional headers
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=sk-or-v1-xxx
OPENROUTER_APP_NAME=Claude-Code-Proxy
OPENROUTER_APP_URL=https://github.com/yourname/claude-code-proxy
```

**Benefits**:
- Higher rate limits from OpenRouter
- Better tracking in OpenRouter dashboard
- Backwards compatible (optional fields)

---

### 2. OpenAI Direct (Testing + Documentation)

**What Should Work**:
```bash
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=sk-proj-xxx
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-4o
ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-4o-mini
ANTHROPIC_DEFAULT_OPUS_MODEL=o1  # Reasoning model
```

**Unknown Compatibility Issues**:

**Issue 1: Reasoning Format Differences**
- OpenRouter uses: `delta.reasoning_details[]` array format
- OpenAI o1/o3 uses: `delta.reasoning_content` string format

Current code (`handlers.go:368`):
```go
// OpenRouter sends reasoning_details in delta.reasoning_details array
if reasoningDetails, ok := delta["reasoning_details"].([]interface{}); ok {
    // ... handles array format ...
}
```

**Needs**: Support both formats:
```go
// Support both OpenRouter and OpenAI reasoning formats
var thinkingText string

// OpenRouter format: reasoning_details array
if reasoningDetails, ok := delta["reasoning_details"].([]interface{}); ok {
    thinkingText = extractReasoningFromArray(reasoningDetails)
}

// OpenAI o1/o3 format: reasoning_content string
if reasoningContent, ok := delta["reasoning_content"].(string); ok && reasoningContent != "" {
    thinkingText = reasoningContent
}
```

**Issue 2: Model Availability**
- o1 models have different token limits and capabilities
- Need to verify tool calling support on o1 models
- May need model-specific configuration

**Testing Required**:
1. Test with `gpt-4o` (standard model, no reasoning)
2. Test with `o1-preview` (reasoning model)
3. Test with `o1-mini` (cheaper reasoning model)
4. Verify tool calling works
5. Verify thinking blocks render correctly

**Documentation Needed**:
- Setup guide for OpenAI direct
- Model compatibility matrix
- Known limitations (if any)

---

### 3. Ollama (Code Changes Required)

**Target Configuration**:
```bash
OPENAI_BASE_URL=http://localhost:11434/v1
OPENAI_API_KEY=ollama  # Dummy value (Ollama doesn't need auth)
ANTHROPIC_DEFAULT_SONNET_MODEL=deepseek-r1:70b
ANTHROPIC_DEFAULT_HAIKU_MODEL=llama3.1:8b
```

**Blockers**:

**Blocker 1: API Key Required**

Current code (`internal/config/config.go:84-86`):
```go
// Validate required fields
if cfg.OpenAIAPIKey == "" {
    return nil, fmt.Errorf("OPENAI_API_KEY is required")
}
```

**Solution**: Make API key optional for localhost:
```go
// Validate required fields
// Allow missing API key for Ollama (localhost endpoints)
if cfg.OpenAIAPIKey == "" {
    if !strings.Contains(cfg.OpenAIBaseURL, "localhost") &&
       !strings.Contains(cfg.OpenAIBaseURL, "127.0.0.1") {
        return nil, fmt.Errorf("OPENAI_API_KEY is required (unless using localhost)")
    }
    // Set dummy key for Ollama
    cfg.OpenAIAPIKey = "ollama"
}
```

**Blocker 2: Authorization Header**

Current code sends `Authorization: Bearer {key}` always.

**Solution**: Skip auth header for localhost:
```go
// Set headers
httpReq.Header.Set("Content-Type", "application/json")

// Skip auth for Ollama (localhost)
if !strings.Contains(cfg.OpenAIBaseURL, "localhost") &&
   !strings.Contains(cfg.OpenAIBaseURL, "127.0.0.1") {
    httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
}
```

**Enhancement 1: Ollama-Specific Timeouts**

Local models respond faster than cloud APIs. Optimize:
```go
// Create HTTP client with provider-aware timeout
timeout := time.Duration(cfg.RequestTimeout) * time.Second

// Ollama runs locally - use shorter timeout
if strings.Contains(cfg.OpenAIBaseURL, "localhost") ||
   strings.Contains(cfg.OpenAIBaseURL, "127.0.0.1") {
    timeout = 30 * time.Second
}

client := &http.Client{
    Timeout: timeout,
}
```

**Enhancement 2: Provider Detection**

Add helper to detect provider type:
```go
// In internal/config/config.go
type ProviderType string

const (
    ProviderOpenRouter ProviderType = "openrouter"
    ProviderOpenAI     ProviderType = "openai"
    ProviderOllama     ProviderType = "ollama"
    ProviderUnknown    ProviderType = "unknown"
)

func (c *Config) DetectProvider() ProviderType {
    baseURL := strings.ToLower(c.OpenAIBaseURL)

    if strings.Contains(baseURL, "openrouter.ai") {
        return ProviderOpenRouter
    }
    if strings.Contains(baseURL, "api.openai.com") {
        return ProviderOpenAI
    }
    if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
        return ProviderOllama
    }
    return ProviderUnknown
}
```

**Testing Required**:
1. Test with `llama3.1:8b` (no reasoning)
2. Test with `deepseek-r1:70b` (with reasoning)
3. Test with `qwen2.5-coder:32b` (coding model)
4. Verify streaming works
5. Verify thinking blocks work with DeepSeek-R1
6. Performance testing (local should be faster)

---

## Implementation Roadmap

### Phase 1: Quick Wins (Can Do Now) 🎯

**Goal**: Make Ollama work with minimal changes

**Tasks**:
1. Make `OPENAI_API_KEY` optional for localhost URLs
2. Skip `Authorization` header for localhost
3. Add provider detection helper
4. Update README with Ollama examples

**Files to Change**:
- `internal/config/config.go` - Optional API key validation
- `internal/server/handlers.go` - Conditional auth header (2 places)
- `README.md` - Add Ollama section

**Estimated Effort**: 30 minutes coding, 15 minutes testing

**Success Criteria**:
- Can run proxy with Ollama without API key errors
- Ollama models respond correctly
- No regression with OpenRouter

---

### Phase 2: OpenAI Compatibility (Should Do) 🔧

**Goal**: Support OpenAI direct with reasoning models

**Tasks**:
1. Add support for `reasoning_content` format (o1/o3 models)
2. Test with actual OpenAI API
3. Document OpenAI direct setup
4. Add OpenAI examples to README

**Files to Change**:
- `internal/server/handlers.go` - Dual reasoning format support
- `internal/converter/converter.go` - May need converter updates
- `README.md` - Add OpenAI section

**Testing Required**:
- Test with `gpt-4o`
- Test with `o1-preview` or `o1-mini`
- Verify thinking blocks render
- Verify tool calling works

**Estimated Effort**: 1 hour coding, 1 hour testing (requires OpenAI API access)

**Success Criteria**:
- OpenAI models work correctly
- o1 reasoning blocks display in Claude Code
- Tool calling works with compatible models

---

### Phase 3: Provider Optimizations (Nice to Have) ✨

**Goal**: Provider-specific enhancements

**Tasks**:
1. Add OpenRouter headers (`HTTP-Referer`, `X-Title`)
2. Add provider auto-detection
3. Provider-specific timeouts
4. Enhanced error messages per provider

**Files to Change**:
- `internal/config/config.go` - Add OpenRouter fields, provider detection
- `internal/server/handlers.go` - Provider-specific headers and timeouts
- `README.md` - Document optional features

**Estimated Effort**: 1 hour coding, 30 minutes testing

**Success Criteria**:
- Better rate limits from OpenRouter
- Faster responses from Ollama (shorter timeouts)
- Better error messages

---

## Testing Strategy

### Manual Testing Checklist

**For Each Provider**:
- [ ] Basic chat works (no tools, no thinking)
- [ ] Streaming works (real-time display)
- [ ] Thinking blocks work (if model supports reasoning)
- [ ] Tool calling works (if model supports tools)
- [ ] Token counting is accurate
- [ ] Error handling works (invalid API key, network errors)
- [ ] Model routing works (opus/sonnet/haiku patterns)

**OpenRouter Testing**:
- [ ] Test with Grok model
- [ ] Test with Gemini model
- [ ] Test with GPT model via OpenRouter
- [ ] Test with optional headers

**OpenAI Testing**:
- [ ] Test with `gpt-4o`
- [ ] Test with `gpt-4o-mini`
- [ ] Test with `o1-preview` (reasoning)
- [ ] Test with `o1-mini` (reasoning)

**Ollama Testing**:
- [ ] Test with `llama3.1:8b`
- [ ] Test with `deepseek-r1:70b` (reasoning)
- [ ] Test with `qwen2.5-coder:32b`
- [ ] Test without API key
- [ ] Test performance vs cloud providers

### Automated Testing

**Unit Tests to Add**:
```go
// internal/config/config_test.go
func TestProviderDetection(t *testing.T) {
    tests := []struct {
        baseURL  string
        expected ProviderType
    }{
        {"https://openrouter.ai/api/v1", ProviderOpenRouter},
        {"https://api.openai.com/v1", ProviderOpenAI},
        {"http://localhost:11434/v1", ProviderOllama},
        {"https://custom.ai/v1", ProviderUnknown},
    }
    // ... test implementation
}

func TestOptionalAPIKey(t *testing.T) {
    // Test that localhost allows missing API key
    // Test that remote requires API key
}
```

```go
// internal/server/handlers_test.go
func TestReasoningFormatCompatibility(t *testing.T) {
    // Test OpenRouter reasoning_details format
    // Test OpenAI reasoning_content format
    // Ensure both render correctly
}
```

---

## Documentation Updates

### README.md Structure

**Update Configuration Section**:
```markdown
## Configuration

### Provider Options

You can use this proxy with three types of providers:

#### 1. OpenRouter (Recommended)
Access 200+ models through a single API.

**Setup**:
```bash
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=sk-or-v1-your-key
ANTHROPIC_DEFAULT_SONNET_MODEL=x-ai/grok-code-fast-1
ANTHROPIC_DEFAULT_HAIKU_MODEL=google/gemini-2.5-flash

# Optional: Better rate limits
OPENROUTER_APP_NAME=My-Claude-Proxy
OPENROUTER_APP_URL=https://github.com/yourname/repo
```

**Benefits**:
- 200+ models (GPT, Grok, Gemini, Claude, etc.)
- Pay-as-you-go pricing
- Single API key for all models
- Higher rate limits with app headers

#### 2. OpenAI Direct
Use OpenAI models directly, including reasoning models.

**Setup**:
```bash
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=sk-proj-your-key
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-4o
ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-4o-mini
ANTHROPIC_DEFAULT_OPUS_MODEL=o1-preview  # Reasoning model
```

**Benefits**:
- Direct access to latest OpenAI models
- o1/o3 reasoning models supported
- Better rate limits (if you have tier 5)

#### 3. Ollama (Local)
Run models locally for free and privacy.

**Setup**:
```bash
OPENAI_BASE_URL=http://localhost:11434/v1
# No API key needed for Ollama!
ANTHROPIC_DEFAULT_SONNET_MODEL=deepseek-r1:70b
ANTHROPIC_DEFAULT_HAIKU_MODEL=llama3.1:8b
```

**Benefits**:
- 100% free (after downloading models)
- Complete privacy (no data leaves your machine)
- Works offline
- Fast responses (local inference)

**Recommended Models**:
- `deepseek-r1:70b` - Reasoning model (similar to o1)
- `qwen2.5-coder:32b` - Best for coding
- `llama3.1:8b` - Fast, good quality
```

### Add Provider Comparison Table

```markdown
## Provider Comparison

| Feature | OpenRouter | OpenAI Direct | Ollama |
|---------|-----------|---------------|---------|
| **Cost** | Pay-per-use | Pay-per-use | Free |
| **Setup** | Easy | Easy | Requires local install |
| **Models** | 200+ | OpenAI only | Open source only |
| **Reasoning** | Yes (via GPT/Grok/etc) | Yes (o1/o3) | Yes (DeepSeek-R1) |
| **Tool Calling** | Yes | Yes | Model dependent |
| **Privacy** | Cloud | Cloud | 100% local |
| **Speed** | Fast | Fast | Very fast (local) |
| **API Key** | Required | Required | Not needed |
```

---

## Risk Analysis

### Potential Issues

**Risk 1: OpenAI Reasoning Format Incompatibility**
- **Probability**: Medium
- **Impact**: High (breaks thinking blocks)
- **Mitigation**: Test thoroughly with o1 models before release
- **Fallback**: Document as unsupported until tested

**Risk 2: Ollama API Differences**
- **Probability**: Low (Ollama claims OpenAI compatibility)
- **Impact**: Medium (some features may not work)
- **Mitigation**: Test with multiple Ollama models
- **Fallback**: Document known limitations

**Risk 3: Breaking Changes for Existing Users**
- **Probability**: Low (mostly additive changes)
- **Impact**: High (would break production setups)
- **Mitigation**: Extensive testing, backwards compatibility focus
- **Fallback**: Version bump, clear migration guide

**Risk 4: Performance Degradation**
- **Probability**: Low
- **Impact**: Medium
- **Mitigation**: Provider detection is simple string matching (fast)
- **Fallback**: Add performance benchmarks

---

## Success Metrics

### Definition of Done

**Phase 1 (Ollama)**:
- [ ] Can start proxy with Ollama without API key errors
- [ ] Chat works with llama3.1:8b
- [ ] Reasoning works with deepseek-r1:70b
- [ ] README has Ollama setup guide
- [ ] No regression with OpenRouter

**Phase 2 (OpenAI)**:
- [ ] Chat works with gpt-4o
- [ ] Reasoning works with o1-preview
- [ ] README has OpenAI setup guide
- [ ] No regression with OpenRouter or Ollama

**Phase 3 (Optimizations)**:
- [ ] OpenRouter headers improve rate limits (measurable)
- [ ] Ollama has faster response times than cloud
- [ ] Provider-specific error messages are helpful
- [ ] Documentation covers all provider-specific features

### User Success Criteria

**What Users Should Experience**:
1. "I can use any provider with a 5-line config change"
2. "Ollama works locally without any API keys"
3. "OpenAI o1 thinking blocks display correctly"
4. "The proxy auto-optimizes for each provider"

---

## Open Questions

1. **OpenAI Reasoning Format**:
   - Does `reasoning_content` work with our current parser?
   - Do we need to handle both `reasoning_details` AND `reasoning_content`?

2. **Ollama Tool Calling**:
   - Which Ollama models support tool calling?
   - Is the format exactly OpenAI-compatible?

3. **Performance**:
   - Should we add request/response caching?
   - Should we add connection pooling per provider?

4. **Model Routing**:
   - Should we auto-detect Ollama models and route differently?
   - Should we have provider-specific routing config?

5. **Error Handling**:
   - Should we retry with backoff per provider?
   - Should we have provider-specific error messages?

---

## Next Steps

**Immediate Actions**:
1. Review this plan with stakeholders
2. Decide on priority: Phase 1 only vs all phases
3. Set up testing environment for each provider
4. Get OpenAI API access for testing (if doing Phase 2)
5. Install Ollama locally for testing (if doing Phase 1)

**Before Starting**:
- [ ] Ensure we have OpenAI API key for testing
- [ ] Install Ollama and download test models
- [ ] Create test scripts for each provider
- [ ] Back up current working state (git branch)

**Recommended Approach**:
Start with Phase 1 (Ollama) as it has clear business value (free local inference) and is lowest risk. Test thoroughly. If successful, proceed to Phase 2 (OpenAI) and Phase 3 (Optimizations).
