# Claude Code Proxy Requirements
## Based on session.jsonl Analysis

### What Claude Code Expects

When Claude Code makes API calls through our proxy, it expects responses in the exact format shown in session.jsonl.

---

## Critical Response Structure Requirements

### 1. **Non-Streaming Response (for /v1/messages endpoint)**

Claude Code sends:
```json
{
  "model": "claude-sonnet-4-5-20250929",
  "messages": [...],
  "stream": false,
  "thinking": {
    "type": "enabled",
    "budget_tokens": 10000
  }
}
```

Claude Code expects back:
```json
{
  "id": "msg_01EzHtyyuKeUcyTGiNutYn1a",
  "type": "message",
  "role": "assistant",
  "model": "claude-sonnet-4-5-20250929",
  "content": [
    {
      "type": "thinking",
      "thinking": "Internal reasoning..."
    },
    {
      "type": "text",
      "text": "Response text..."
    }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "cache_creation": {
      "ephemeral_5m_input_tokens": 0,
      "ephemeral_1h_input_tokens": 0
    }
  }
}
```

### 2. **Streaming Response (/v1/messages endpoint with stream: true)**

Claude Code sends:
```json
{
  "model": "claude-haiku-4-5-20251001",
  "messages": [...],
  "stream": true,
  "thinking": {
    "type": "enabled",
    "budget_tokens": 10000
  }
}
```

Claude Code expects back (Server-Sent Events format):
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01...","type":"message","role":"assistant","model":"...","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0,...}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","text":"..."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"..."}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":0,"output_tokens":0,...}}

event: message_stop
data: {"type":"message_stop"}
```

---

## Key Differences from OpenAI Format

### Content Structure

**OpenAI Format:**
```json
{
  "choices": [{
    "delta": {
      "content": "text content",
      "reasoning": "thinking content"
    }
  }]
}
```

**Claude Format (what proxy must return):**
```json
{
  "content": [
    {"type": "thinking", "thinking": "..."},
    {"type": "text", "text": "..."}
  ]
}
```

### Content Blocks

Our proxy must convert:
- OpenAI's `delta.content` → Claude's `content_block_delta` with `type: "text_delta"`
- OpenAI's `delta.reasoning` → Claude's `content_block_delta` with `type: "thinking_delta"`

### Streaming Events

Our proxy must generate:
- `message_start` - Once at the beginning
- `content_block_start` - Before each content block (thinking, text)
- `content_block_delta` - For each chunk of content
- `content_block_stop` - After each content block
- `message_delta` - With final stop_reason
- `message_stop` - Final marker

### Content Block Ordering

**CRITICAL**: Must follow this order:
1. Thinking block (index 0) - if thinking is enabled
2. Text block (index 1)
3. Tool use blocks (index 2, 3, 4, ...) - if applicable

**WRONG** (both at index 0):
```json
[
  {"type": "thinking", ...},  // index 0
  {"type": "text", ...}       // ALSO index 0 - CRASH!
]
```

**CORRECT**:
```json
[
  {"type": "thinking", ...},  // index 0
  {"type": "text", ...}       // index 1
]
```

---

## Usage Field Requirements

Must always include (even if zeros):
```json
{
  "input_tokens": 0,
  "output_tokens": 0,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 0,
  "cache_creation": {
    "ephemeral_5m_input_tokens": 0,
    "ephemeral_1h_input_tokens": 0
  }
}
```

This allows Claude Code to:
- ✅ Track token consumption
- ✅ Show usage in UI
- ✅ Apply auto-compacting based on context window
- ✅ Implement prompt caching if available

---

## Stop Reason Values

Valid values:
- `"end_turn"` - Normal completion (default)
- `"tool_use"` - Claude wants to use a tool
- `"max_tokens"` - Reached token limit
- null - Stream not finished (in streaming chunks)

---

## Tools & Tool Results

Claude Code uses tools like:
- `read`, `write`, `edit` - File operations
- `glob`, `grep` - File search
- `bash` - Shell execution
- `todowrite`, `todoread` - Task management

When Claude uses a tool, it includes:
```json
{
  "type": "tool_use",
  "id": "toolu_...",
  "name": "read",
  "input": {"file_path": "/path/to/file"}
}
```

Claude Code then executes the tool and sends back:
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_...",
  "content": "Tool output here"
}
```

**Our proxy doesn't handle tools** - they're Claude Code specific.

---

## What Our Go Proxy Currently Does ✅

1. ✅ Converts Claude request to OpenAI format
2. ✅ Sends to OpenRouter/OpenAI-compatible providers
3. ✅ Receives SSE stream with reasoning blocks
4. ✅ Converts back to Claude format with proper events
5. ✅ Sends correct content block structure with indices
6. ✅ Includes usage information
7. ✅ Handles thinking blocks properly (with signature support)
8. ✅ Removes malformed debug output
9. ✅ Supports tool calling (tool_use ↔ tool_calls conversion)
10. ✅ Accurate streaming token tracking
11. ✅ Multiple backend provider support (OpenRouter, OpenAI, etc.)
12. ✅ Pattern-based model routing

---

## Feature Status

All major features are now implemented and tested:
- ✅ Tool use support (Claude Code tool_use blocks properly converted)
- ✅ Multiple content blocks in single response
- ✅ Stop reason handling (end_turn, tool_use, max_tokens)
- ✅ Error handling for malformed requests
- ✅ Model validation and routing (pattern-based + env overrides)
- ✅ Token limit enforcement (configurable via MAX_TOKENS_LIMIT)
- ✅ Comprehensive unit test coverage

---

## Session Patterns We Observed

1. **Warmup Message** → Haiku response (quick)
2. **Main Work** → Sonnet responses with thinking
3. **Tool Usage** → Read/Glob/Grep for files
4. **Cache Building** → Early messages create cache
5. **Cache Reuse** → Later messages use cached tokens at 10% cost

---

## Success Metrics

The proxy works correctly when:
- ✅ Claude Code doesn't crash with "Cannot read properties of undefined"
- ✅ Thinking blocks appear in Claude Code UI
- ✅ Responses are returned without malformed events
- ✅ Content blocks have unique, sequential indices
- ✅ Usage tokens are tracked (even if 0 from OpenRouter)
- ✅ Stop reason is correct

