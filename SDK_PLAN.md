# Claude Code Proxy: Multi-Backend SDK Implementation Plan

## Mission Statement
**Enable Claude Code to work perfectly with ANY AI backend** by building a proxy that uses official SDKs to guarantee 100% Anthropic API compatibility.

## Why This Matters
- **Claude Code** is an excellent IDE/editor for AI-assisted development
- Users should be able to use Claude Code with GPT-4, local models, or any AI provider
- Perfect compatibility means ALL Claude Code features work: thinking blocks, tools, streaming, caching

---

## Architecture Overview

```
┌─────────────┐
│ Claude Code │
└─────┬───────┘
      │ Anthropic API Format
      ▼
┌─────────────────────────────────────────┐
│         claude-code-proxy SDK Proxy            │
├─────────────────────────────────────────┤
│ 1. Anthropic SDK (Parse Request)        │
│ 2. Route to Backend                     │
│ 3. Backend SDK (Make Request)           │
│ 4. Parse Response                       │
│ 5. Anthropic SDK (Format Response)      │
└─────────────────────────────────────────┘
      │         │         │
      ▼         ▼         ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ OpenAI   │ │OpenRouter│ │  Ollama  │
└──────────┘ └──────────┘ └──────────┘
```

---

## SDKs to Use

1. **Input/Output Format**: `github.com/anthropics/anthropic-sdk-go`
   - Parse incoming Claude API requests
   - Generate properly formatted Claude API responses
   - Handle thinking blocks, tool use, and caching correctly

2. **Backend Clients**:
   - `github.com/openai/openai-go` - OpenAI API
   - `github.com/reVrost/go-openrouter` - OpenRouter API
   - `github.com/rozoomcool/go-ollama-sdk` - Ollama local models

---

## Implementation Plan

### Phase 1: Core Infrastructure
- [ ] Set up Go module with all SDK dependencies
- [ ] Create configuration system for API keys and endpoints
- [ ] Build request router to select backend based on model name
- [ ] Implement health check endpoints

### Phase 2: Anthropic SDK Integration
- [ ] Use Anthropic SDK to parse incoming requests
- [ ] Extract model, messages, parameters
- [ ] Use SDK's streaming response writer for output
- [ ] Ensure thinking blocks render correctly in Claude Code

### Phase 3: Backend Integrations

#### OpenRouter Backend
- [ ] Initialize OpenRouter client with API key
- [ ] Convert Anthropic messages to OpenRouter format
- [ ] Handle reasoning/thinking field mapping
- [ ] Stream responses back through Anthropic SDK

#### OpenAI Backend
- [ ] Initialize OpenAI client
- [ ] Map Claude models to GPT models
- [ ] Convert message formats
- [ ] Handle system prompts correctly

#### Ollama Backend
- [ ] Initialize Ollama client for local models
- [ ] Map Claude models to local model names
- [ ] Handle streaming responses
- [ ] Support local model configuration

### Phase 4: Advanced Features
- [ ] Tool use translation between formats
- [ ] Prompt caching support
- [ ] Token counting across backends
- [ ] Error handling and retries
- [ ] Request/response logging

### Phase 5: Claude Code Testing & Validation
- [ ] Test all Claude Code features:
  - [ ] Thinking blocks render in collapsible UI
  - [ ] File operations (read/write/edit)
  - [ ] Bash command execution
  - [ ] Todo list management
  - [ ] Multi-file operations
  - [ ] Streaming responses feel smooth
  - [ ] Error messages display correctly
- [ ] Test with different prompts:
  - [ ] "lets understand this project - ultrathink"
  - [ ] Complex multi-step tasks
  - [ ] Long-running operations
- [ ] Verify model switching works
- [ ] Performance benchmarking
- [ ] Memory optimization

---

## Model Mapping Strategy

### Environment Variables
```bash
# Claude model mappings
ANTHROPIC_DEFAULT_OPUS_MODEL=openrouter:anthropic/claude-opus-4-1-20250805
ANTHROPIC_DEFAULT_SONNET_MODEL=openai:gpt-4o
ANTHROPIC_DEFAULT_HAIKU_MODEL=ollama:llama3.2

# Backend configurations
OPENROUTER_API_KEY=sk-or-...
OPENAI_API_KEY=sk-...
OLLAMA_HOST=http://localhost:11434
```

### Model Router Logic
```go
func routeModel(model string) (backend, actualModel string) {
    // Check env var overrides first
    if override := os.Getenv("ANTHROPIC_DEFAULT_" + strings.ToUpper(extractModelType(model)) + "_MODEL"); override != "" {
        parts := strings.SplitN(override, ":", 2)
        return parts[0], parts[1]
    }

    // Default routing based on prefix
    switch {
    case strings.HasPrefix(model, "gpt-"):
        return "openai", model
    case strings.HasPrefix(model, "claude-"):
        return "openrouter", model
    default:
        return "ollama", model
    }
}
```

---

## Key Benefits

1. **Perfect API Compatibility**
   - Official SDKs ensure exact format compliance
   - Thinking blocks display correctly in Claude Code
   - All features work as expected

2. **Multiple Backend Support**
   - Use OpenAI's GPT models through Claude API
   - Access OpenRouter's model selection
   - Run local models with Ollama
   - Mix and match based on cost/performance

3. **Maintainability**
   - SDK updates handle API changes
   - Clean separation of concerns
   - Easier to debug and extend

4. **Extended Thinking Support**
   - Proper thinking block formatting
   - Correct streaming behavior
   - Claude Code UI integration

---

## File Structure

```
proxy/
├── cmd/
│   └── proxy/
│       └── main.go           # Entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── router/
│   │   └── router.go         # Model routing logic
│   ├── backends/
│   │   ├── openrouter.go    # OpenRouter backend
│   │   ├── openai.go        # OpenAI backend
│   │   └── ollama.go        # Ollama backend
│   ├── converter/
│   │   └── converter.go     # Format conversion utilities
│   └── server/
│       └── server.go         # HTTP server with Anthropic SDK
├── go.mod
├── go.sum
└── .env.example
```

---

## Example Usage

### 1. Claude Code with GPT-4
```bash
export ANTHROPIC_DEFAULT_SONNET_MODEL=openai:gpt-4o
export OPENAI_API_KEY=sk-...
./ultrathink
# Claude Code will use GPT-4 when requesting Sonnet
```

### 2. Local Ollama Models
```bash
export ANTHROPIC_DEFAULT_HAIKU_MODEL=ollama:llama3.2
export OLLAMA_HOST=http://localhost:11434
./ultrathink
# Claude Code will use local Llama when requesting Haiku
```

### 3. Mixed Configuration
```bash
export ANTHROPIC_DEFAULT_OPUS_MODEL=openrouter:anthropic/claude-opus
export ANTHROPIC_DEFAULT_SONNET_MODEL=openai:gpt-4o
export ANTHROPIC_DEFAULT_HAIKU_MODEL=ollama:mistral
./ultrathink
# Different backends for different model tiers
```

---

## Development Timeline

- **Week 1**: Core infrastructure + Anthropic SDK integration
- **Week 2**: OpenRouter backend implementation
- **Week 3**: OpenAI and Ollama backends
- **Week 4**: Testing, optimization, and documentation

---

## Success Metrics

### Claude Code Compatibility (PRIMARY GOAL)
1. ✅ **Thinking blocks** display in collapsible UI format
2. ✅ **Tools** (read, write, bash, etc.) work correctly
3. ✅ **Streaming** feels real-time with proper progress indication
4. ✅ **Token counting** shows accurate usage
5. ✅ **Error handling** displays properly in Claude Code UI
6. ✅ **Model switching** (haiku/sonnet/opus) works seamlessly
7. ✅ **Context management** and auto-compacting function

### Performance Metrics
- **Streaming latency**: < 100ms additional overhead
- **First token time**: < 500ms from request
- **Error rate**: < 0.1% failed requests

---

## Next Steps

1. Review and approve this plan
2. Set up development environment
3. Create GitHub repository structure
4. Begin Phase 1 implementation
5. Set up CI/CD pipeline

---

## Notes

- Focus on streaming performance for real-time feel
- Ensure graceful degradation when backends are unavailable
- Consider adding metrics/monitoring endpoints
- Plan for horizontal scaling if needed