# Claude Code Session Format Documentation
## Based on analysis of session.jsonl (796 events)

### Overview
This is a JSONL (JSON Lines) format session log from Claude Code v2.0.19 showing a complete interaction with the Anthropic API (and proxies).

**Session Stats:**
- Total Events: 796 lines
- User Messages: 243
- Assistant Messages: 486
- Models: Claude Haiku (3 times), Claude Sonnet (483 times)
- Contains: Thinking blocks, Tool usage, File operations, Caching

---

## Event Types

```
1. user          (243) - User input/messages
2. assistant     (486) - Claude responses  
3. system        (2)   - System messages
4. summary       (4)   - Session summaries
5. file-history-snapshot (61) - File state snapshots
```

---

## Event Structure

### USER MESSAGE
```json
{
  "parentUuid": null,                    // Reference to previous message
  "isSidechain": true/false,            // Is this a side conversation?
  "userType": "external",               // Type of user
  "cwd": "/path/to/directory",          // Current working directory
  "sessionId": "uuid",                  // Session identifier
  "version": "2.0.19",                  // Claude Code version
  "gitBranch": "main",                  // Git branch if applicable
  "type": "user",                       // Event type
  "message": {
    "role": "user",
    "content": "User's text or tool results array"
  },
  "uuid": "unique-event-id",
  "timestamp": "2025-10-16T13:15:27.749Z",
  "thinkingMetadata": {                 // When user enables thinking
    "level": "high",
    "disabled": false,
    "triggers": []
  }
}
```

### ASSISTANT MESSAGE
```json
{
  "parentUuid": "uuid-of-parent",
  "isSidechain": false,
  "userType": "external",
  "cwd": "/path/to/directory",
  "sessionId": "uuid",
  "version": "2.0.19",
  "gitBranch": "main",
  "type": "assistant",
  "message": {
    "model": "claude-sonnet-4-5-20250929",
    "id": "msg_01EzHtyyuKeUcyTGiNutYn1a",
    "type": "message",
    "role": "assistant",
    "content": [
      {
        "type": "thinking",
        "thinking": "Internal reasoning text..."
      },
      {
        "type": "text",
        "text": "Response text to user..."
      },
      {
        "type": "tool_use",
        "id": "toolu_017ungHk4XYtLaZJ3bE8Usfx",
        "name": "todowrite",
        "input": { ... }
      }
    ],
    "stop_reason": null,              // or "end_turn", "tool_use"
    "stop_sequence": null,
    "usage": {
      "input_tokens": 10,
      "cache_creation_input_tokens": 9064,    // New tokens cached
      "cache_read_input_tokens": 14816,       // Tokens from cache
      "cache_creation": {
        "ephemeral_5m_input_tokens": 9064,
        "ephemeral_1h_input_tokens": 0
      },
      "output_tokens": 2,
      "service_tier": "standard"
    }
  },
  "requestId": "req_011CUAwTnfNCBaZBGgvHN6Xa",
  "uuid": "event-uuid",
  "timestamp": "2025-10-16T13:16:01.452Z"
}
```

### FILE HISTORY SNAPSHOT
```json
{
  "type": "file-history-snapshot",
  "messageId": "uuid",
  "snapshot": {
    "messageId": "uuid",
    "trackedFileBackups": {},      // Files changed during this message
    "timestamp": "2025-10-16T13:15:53.586Z"
  },
  "isSnapshotUpdate": false
}
```

### SYSTEM MESSAGE
```json
{
  "type": "system",
  "message": {
    "content": "System notification or status"
  },
  "timestamp": "2025-10-16T..."
}
```

### SUMMARY
```json
{
  "type": "summary",
  "summary": "Human readable summary of conversation section",
  "leafUuid": "uuid-of-last-event"
}
```

---

## Content Block Types

### 1. Thinking Block (Extended Thinking)
```json
{
  "type": "thinking",
  "thinking": "Claude's internal reasoning process...\n\nThis is visible when extended thinking is enabled."
}
```

### 2. Text Block
```json
{
  "type": "text",
  "text": "Response text from Claude"
}
```

### 3. Tool Use Block
```json
{
  "type": "tool_use",
  "id": "toolu_017ungHk4XYtLaZJ3bE8Usfx",
  "name": "read",
  "input": {
    "file_path": "/path/to/file"
  }
}
```

### 4. Tool Result Block (in user messages)
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_017ungHk4XYtLaZJ3bE8Usfx",
  "content": "Result of the tool execution"
}
```

---

## Message Flow / Conversation Pattern

```
1. User sends message (type: "user")
   ↓
2. Claude processes (with thinking if enabled)
   ↓
3. Claude responds with one or more content blocks (type: "assistant")
   ├─ thinking block (internal reasoning)
   ├─ text block (response to user)
   └─ tool_use blocks (if needed)
   ↓
4. File history snapshot (type: "file-history-snapshot")
   ↓
5. User sends tool results back (type: "user", content with tool_result blocks)
   ↓
6. Repeat from step 2...

[Cycle repeats for 796 total events]
```

---

## Key Features Observed

### 1. Prompt Caching
```
cache_creation_input_tokens: 9064     // New content added to cache
cache_read_input_tokens: 14816        // Content retrieved from cache
```
- Anthropic's prompt caching reduces input token costs
- Later messages reuse cached content

### 2. Model Selection
- Haiku: Used for warmups or simple tasks (3 times)
- Sonnet: Used for most actual work (483 times)
- Models can change during conversation

### 3. Extended Thinking
- Sessions have `thinkingMetadata` to enable thinking
- Thinking blocks appear in assistant content
- Helps Claude reason through complex tasks

### 4. Tool Usage
Tools referenced in session:
- `read` - Read files
- `write` - Write files
- `edit` - Edit specific file content
- `glob` - Find files by pattern
- `grep` - Search file contents
- `bash` - Execute shell commands
- `todowrite` - Manage task lists
- `todoread` - Read task lists

### 5. Git Integration
- Tracks git branch (main, feature branches, etc.)
- Can read git status, logs, diffs
- Used for code organization

---

## Usage/Billing Implications

```
Total tokens = input_tokens + cache_creation_input_tokens + cache_read_input_tokens

Costs:
- input_tokens: Full cost (e.g., $3/1M for Sonnet)
- cache_creation_input_tokens: 25% of input cost (e.g., $0.75/1M)
- cache_read_input_tokens: 10% of input cost (e.g., $0.30/1M)

In this session:
- Real input tokens: ~7-10 tokens
- Created cache: ~1256 tokens (saved for reuse)
- Read from cache: ~36815 tokens (cheap!)
```

---

## Important Fields for Proxy Implementation

When implementing a proxy that needs to return Claude Code-compatible responses:

1. **Message Structure**: Must match exactly
2. **Content Blocks**: Must support thinking, text, tool_use
3. **Usage Tracking**: Must include cache metrics if caching is used
4. **RequestId**: Unique identifier for tracking
5. **Stop Reason**: Must be valid ("end_turn", "tool_use", null, etc.)
6. **Model Field**: Correct model name
7. **Tool Results**: Must support tool_result content blocks in user messages

---

## Response Expectations

Claude Code expects responses with:
- ✅ Proper message structure
- ✅ Content blocks with correct types
- ✅ Usage information (even if cached amounts are 0)
- ✅ Stop reason to know when Claude is done
- ✅ Request ID for session tracking
- ✅ Thinking blocks IF extended thinking is enabled

