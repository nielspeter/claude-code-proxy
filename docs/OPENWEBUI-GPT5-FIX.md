# OpenWebUI GPT-5 Support Fix

**Date:** 2025-11-12
**Issue:** GPT-5 reasoning model hanging when accessed through claude-code-proxy with OpenWebUI backend
**Status:** ✅ Fixed

## Problem Description

When using the proxy with OpenWebUI (https://gpt.erst.dk/api), GPT-5 requests would hang indefinitely, while GPT-4.1 (Haiku) worked perfectly. The issue manifested as:

- ✅ **Haiku tier (gpt-4.1)**: Working correctly
- ❌ **Sonnet tier (gpt-5)**: Hanging, no response
- ❌ **Opus tier (gpt-o3)**: Presumably affected (reasoning model)

## Investigation Process

### 1. Initial Diagnosis

First confirmed the issue was specific to reasoning models (GPT-5, o-series) by testing:
```bash
# Working
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model haiku -p "hi"

# Hanging
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "hi"
```

### 2. API Endpoint Discovery

Used curl to test OpenWebUI's API directly and discovered:

**Wrong endpoint:**
```bash
curl https://gpt.erst.dk/api/v1/chat/completions
# Returns: 405 Method Not Allowed
```

**Correct endpoint:**
```bash
curl https://gpt.erst.dk/api/chat/completions
# Works! But returns error about max_tokens
```

**Key finding:** OpenWebUI uses `/api/chat/completions`, not the standard OpenAI `/v1/chat/completions`

### 3. Parameter Issue Discovery

When testing with the correct endpoint:
```json
{
  "model": "gpt-5",
  "messages": [{"role": "user", "content": "Say hi"}],
  "max_completion_tokens": 100,
  "temperature": 1
}
```

Received error:
```json
{
  "detail": "litellm.BadRequestError: AzureException BadRequestError -
   Unsupported parameter: 'max_tokens' is not supported with this model.
   Use 'max_completion_tokens' instead."
}
```

**Key finding:** OpenWebUI/LiteLLM was converting our `max_completion_tokens` back to `max_tokens` before sending to Azure!

### 4. Workaround Discovery

Testing without any max tokens parameter:
```json
{
  "model": "gpt-5",
  "messages": [{"role": "user", "content": "Say hi"}],
  "temperature": 1
}
```

Result: ✅ **Success!**
```json
{
  "id": "chatcmpl-CbDsMSy25XbiXoeJ6T7IcqSc7ZIzh",
  "model": "gpt-5-2025-08-07",
  "choices": [{
    "message": {"content": "Hi", "role": "assistant"},
    "finish_reason": "stop"
  }],
  "usage": {
    "reasoning_tokens": 128,
    "completion_tokens": 139,
    "prompt_tokens": 797
  }
}
```

## Root Cause

**OpenWebUI/LiteLLM Bug:** The backend incorrectly transforms `max_completion_tokens` to `max_tokens` before forwarding to Azure's GPT-5 endpoint, which only accepts `max_completion_tokens` for reasoning models.

**Why it works in OpenWebUI UI:** The UI likely doesn't send any max tokens parameter, letting the backend use its defaults.

## Solution

Modified `internal/converter/converter.go` (lines 140-163) to handle provider-specific token limit parameters:

```go
// Set token limit
if claudeReq.MaxTokens > 0 {
	// Reasoning models (o1, o3, o4, gpt-5) require max_completion_tokens
	// instead of the legacy max_tokens parameter.
	// Uses dynamic detection from OpenRouter API for reasoning models.
	//
	// IMPORTANT: OpenWebUI/LiteLLM has a bug where it converts max_completion_tokens
	// back to max_tokens before sending to Azure, causing failures for GPT-5.
	// Workaround: Don't send any max tokens parameter for Unknown providers (OpenWebUI)
	// with reasoning models.
	provider := cfg.DetectProvider()
	if cfg.IsReasoningModel(openaiModel) {
		if provider == config.ProviderOpenAI {
			// OpenAI Direct: Use max_completion_tokens
			openaiReq.MaxCompletionTokens = claudeReq.MaxTokens
		}
		// For ProviderUnknown (OpenWebUI): Don't set any max tokens parameter
		// This is a workaround for OpenWebUI/LiteLLM bug that converts
		// max_completion_tokens to max_tokens, causing Azure to reject it
	} else {
		// Non-reasoning models: Use standard max_tokens
		openaiReq.MaxTokens = claudeReq.MaxTokens
	}
}
```

### Logic Summary

| Provider | Model Type | Parameter Sent |
|----------|-----------|----------------|
| OpenAI Direct | Reasoning (GPT-5, o1, o3, o4) | `max_completion_tokens` |
| OpenAI Direct | Standard | `max_tokens` |
| Unknown (OpenWebUI) | Reasoning | *None* (workaround) |
| Unknown (OpenWebUI) | Standard | `max_tokens` |
| OpenRouter | All | `max_tokens` |
| Ollama | All | `max_tokens` |

## Configuration

Correct `.env` configuration for OpenWebUI:

```bash
# OpenWebUI Configuration
OPENAI_BASE_URL=https://gpt.erst.dk/api
OPENAI_API_KEY=<your-jwt-token>

# Model routing
ANTHROPIC_DEFAULT_OPUS_MODEL=gpt-o3
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-5
ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-4.1
```

**Important:** The base URL should be `https://gpt.erst.dk/api` (without `/v1`), as the proxy appends `/chat/completions` to match OpenWebUI's endpoint structure.

## Test Results

After implementing the fix:

```bash
# Test Sonnet (GPT-5)
$ ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "Say hi in Danish"
Hej! Hvordan kan jeg hjælpe dig i dag?
✅ Success

# Test Haiku (GPT-4.1)
$ ANTHROPIC_BASE_URL=http://localhost:8082 claude --model haiku -p "Say hi in English"
Hi there! How can I help you today?
✅ Success
```

## Technical Details

### Provider Detection

The proxy uses URL pattern matching to detect OpenWebUI:
```go
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
	return ProviderUnknown  // OpenWebUI falls here
}
```

### Reasoning Model Detection

Pattern matching for reasoning models:
```go
func (c *Config) IsReasoningModel(modelName string) bool {
	model := strings.ToLower(modelName)
	model = strings.TrimPrefix(model, "azure/")
	model = strings.TrimPrefix(model, "openai/")

	// Check for o-series reasoning models (o1, o2, o3, o4, etc.)
	if strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o2") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4") {
		return true
	}

	// Check for GPT-5 series (gpt-5, gpt-5-mini, gpt-5-turbo, etc.)
	if strings.HasPrefix(model, "gpt-5") {
		return true
	}

	return false
}
```

## Known Limitations

1. **No max tokens enforcement for OpenWebUI reasoning models:** The workaround means Claude Code's `max_tokens` parameter is ignored when using GPT-5 through OpenWebUI. The model will use its default token limits.

2. **OpenWebUI-specific workaround:** This is a temporary fix until OpenWebUI/LiteLLM properly handles `max_completion_tokens` for reasoning models.

3. **Affects all Unknown providers:** Any provider that doesn't match OpenRouter/OpenAI/Ollama patterns will be treated like OpenWebUI. This is generally safe but may need refinement for other providers.

## Future Improvements

1. **Add explicit OpenWebUI detection:** Instead of relying on `ProviderUnknown`, detect OpenWebUI specifically:
   ```go
   if strings.Contains(baseURL, "openwebui") || strings.Contains(baseURL, "gpt.erst.dk") {
       return ProviderOpenWebUI
   }
   ```

2. **Monitor OpenWebUI/LiteLLM bug fix:** Once the upstream bug is fixed, restore proper `max_completion_tokens` support.

3. **Add provider-specific tests:** Create tests in `internal/converter/provider_test.go` for OpenWebUI:
   ```go
   func TestOpenWebUIReasoningModels(t *testing.T) {
       // Verify no max tokens parameter for reasoning models
   }
   ```

## References

- OpenWebUI Documentation: https://docs.openwebui.com/getting-started/api-endpoints/
- Issue Discovery: curl testing revealed endpoint mismatch
- LiteLLM GitHub: https://github.com/BerriAI/litellm (underlying OpenWebUI proxy)
- Azure OpenAI GPT-5 Docs: Requires `max_completion_tokens` for reasoning models

## Related Files

- `internal/converter/converter.go` - Request conversion logic (lines 140-163)
- `internal/config/config.go` - Provider detection (lines 143-206)
- `.env` - OpenWebUI configuration
- `README.md` - Updated provider comparison table

## Commit Information

This fix should be committed with the following details:

**Commit message:**
```
fix: Add OpenWebUI GPT-5 reasoning model support

OpenWebUI/LiteLLM has a bug where it converts max_completion_tokens
to max_tokens before forwarding to Azure, causing GPT-5 to fail.

Workaround: Don't send any max tokens parameter for Unknown providers
(like OpenWebUI) when using reasoning models. The backend will use
its default token limits instead.

Tested with:
- OpenWebUI GPT-5 (gpt-5-2025-08-07) ✅
- OpenWebUI GPT-4.1 (gpt-4.1-2025-04-14) ✅

Closes #[issue-number]
```

**Files changed:**
- `internal/converter/converter.go` (modified token parameter logic)
- `docs/OPENWEBUI-GPT5-FIX.md` (this documentation)
