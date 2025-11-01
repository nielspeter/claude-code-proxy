# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive unit tests for dynamic reasoning model detection
- Test coverage for `IsReasoningModel()` and `FetchReasoningModels()` functions
- Mock HTTP server tests for OpenRouter API integration

### Changed
- Updated reasoning model detection tests to use `cfg.IsReasoningModel()` method

## [1.1.0] - 2025-10-31

### Added
- **Dynamic reasoning model detection** from OpenRouter API (#5)
  - Automatically fetches list of reasoning-capable models on startup
  - Caches 130+ reasoning models (DeepSeek-R1, Gemini, GPT-5, o-series, etc.)
  - Falls back to hardcoded pattern matching for OpenAI Direct and Ollama
- Robust `max_completion_tokens` parameter detection for reasoning models
- Provider-specific model detection (OpenRouter uses API cache, others use patterns)

### Changed
- Moved reasoning model detection from standalone function to `Config` method
- Improved model detection to support dynamic discovery of new reasoning models
- Enhanced `IsReasoningModel()` to check provider type before using cache

### Technical Details
- Uses OpenRouter's `supported_parameters=reasoning` endpoint (no auth required)
- Asynchronous model fetching to avoid blocking startup
- Global `ReasoningModelCache` with populated flag for fallback behavior

## [1.0.0] - 2025-10-26

### Added
- **Initial release** of Claude Code Proxy
- Bidirectional API format conversion (Claude ↔ OpenAI)
- **Multi-provider support**:
  - OpenRouter (200+ models including Grok, Gemini, DeepSeek)
  - OpenAI Direct (GPT-4, GPT-5, o1, o3)
  - Ollama (local models)
- **Full Claude Code feature compatibility**:
  - Tool calling (function calling)
  - Extended thinking blocks (from reasoning models)
  - Streaming responses with SSE
  - Token usage tracking
- **Pattern-based model routing**:
  - Haiku → lightweight models (gpt-5-mini, gemini-flash)
  - Sonnet → flagship models (gpt-5, grok)
  - Opus → premium models (gpt-5, o3)
- **Daemon mode** with background process management
- **`ccp` wrapper script** for seamless Claude Code integration
- **Simple log mode** (`-s` flag) with one-line request summaries and throughput metrics
- **Debug mode** (`-d` flag) for full request/response logging
- Environment variable configuration via `.env` files
- Provider-specific parameter injection:
  - OpenRouter: `reasoning: {enabled: true}`, `usage: {include: true}`
  - OpenAI Direct: `reasoning_effort: "medium"` for GPT-5
  - Ollama: `tool_choice: "required"` when tools present
- Comprehensive unit tests for converter and tool calling

### Fixed
- Thinking blocks now use correct `thinking` field (not `text`)
- Streaming token usage continues past `finish_reason`
- Tool calling format conversion between Claude and OpenAI
- Encrypted reasoning blocks from models like Grok (skipped, not shown)
- HTTP logging now respects simple log mode setting
- Golangci-lint errors for CI/CD pipeline

### Changed
- Replaced hardcoded model names with constants
- Removed unused configuration options (`MAX_TOKENS_LIMIT`, `REQUEST_TIMEOUT`)
- Simplified Sonnet pattern to match all versions (sonnet-3, sonnet-4, sonnet-5)
- Updated documentation to remove o1/o3 references from defaults

### Documentation
- Complete CLI command and flag documentation
- Environment variable override documentation
- CLAUDE.md for AI-assisted development
- Beta software disclaimer
- MIT License

### Infrastructure
- GitHub Actions workflows for CI/CD
- golangci-lint integration
- Automated testing pipeline
- Claude Code Review workflow
- Claude PR Assistant workflow

## [0.1.0] - Initial Development

### Added
- Manual Anthropic-to-OpenAI proxy implementation (proof of concept)

---

## Release Notes

### How to Use This Changelog

- **Unreleased**: Changes in `main` branch not yet released
- **[X.Y.Z]**: Released versions with dates
- **Categories**:
  - `Added`: New features
  - `Changed`: Changes to existing functionality
  - `Deprecated`: Soon-to-be removed features
  - `Removed`: Removed features
  - `Fixed`: Bug fixes
  - `Security`: Security fixes

### Upgrade Guide

#### From v1.0.0 to v1.1.0
- No breaking changes
- Dynamic reasoning model detection happens automatically
- Existing `.env` configurations remain compatible
- Hardcoded fallback ensures compatibility if OpenRouter API fetch fails

---

**Links:**
- [v1.1.0 Release](https://github.com/nielspeter/claude-code-proxy/releases/tag/v1.1.0)
- [v1.0.0 Release](https://github.com/nielspeter/claude-code-proxy/releases/tag/v1.0.0)
