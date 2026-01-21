// Package converter 处理 Claude 和 OpenAI API 格式之间的双向转换。
//
// 提供将 Claude API 请求转换为 OpenAI 兼容格式的函数，以及将 OpenAI 响应转换回 Claude 格式的函数。
// 包括模型映射、消息结构转换、工具调用处理，以及从推理响应中提取思考块。
package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"claude-code-proxy/internal/config"
	"claude-code-proxy/pkg/models"
)

// 默认模型映射（当环境变量未设置时使用）
// 可通过以下环境变量覆盖：
//   - ANTHROPIC_DEFAULT_OPUS_MODEL
//   - ANTHROPIC_DEFAULT_SONNET_MODEL
//   - ANTHROPIC_DEFAULT_HAIKU_MODEL
const (
	DefaultOpusModel   = "gpt-5"
	DefaultSonnetModel = "gpt-5"
	DefaultHaikuModel  = "gpt-5-mini"
)

// extractSystemText 从 Claude 的灵活系统参数中提取系统文本。
// Claude 支持字符串格式 ("system": "text") 和包含内容块的数组格式。
// 此函数将两种格式标准化为单个字符串，以兼容 OpenAI。
func extractSystemText(system interface{}) string {
	if system == nil {
		return ""
	}

	// 处理字符串格式
	if systemStr, ok := system.(string); ok {
		return systemStr
	}

	// 处理数组格式
	if systemArr, ok := system.([]interface{}); ok {
		var textParts []string
		for _, block := range systemArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						textParts = append(textParts, text)
					}
				}
			}
		}
		return strings.Join(textParts, "\n")
	}

	return ""
}

// extractReasoningText 从 OpenRouter reasoning_details 中提取文本
// 处理不同的推理详情类型：reasoning.text、reasoning.summary、reasoning.encrypted
func extractReasoningText(detail map[string]interface{}) string {
	detailType, _ := detail["type"].(string)

	switch detailType {
	case "reasoning.text":
		// 提取文本字段
		if text, ok := detail["text"].(string); ok {
			return text
		}
	case "reasoning.summary":
		// 提取摘要字段
		if summary, ok := detail["summary"].(string); ok {
			return summary
		}
	case "reasoning.encrypted":
		// 跳过加密的推理内容 - 这是 base64 编码的加密数据，不应显示
		// 像 Grok 这样的模型会在 reasoning.summary 旁边发送这个
		return ""
	}

	return ""
}

// ConvertRequest 将 Claude API 请求转换为 OpenAI 格式
func ConvertRequest(claudeReq models.ClaudeRequest, cfg *config.Config) (*models.OpenAIRequest, error) {
	// 使用基于模式的路由映射模型
	openaiModel := mapModel(claudeReq.Model, cfg)

	// 提取系统消息（可以是字符串或内容块数组）
	systemText := extractSystemText(claudeReq.System)

	// 转换消息
	openaiMessages := convertMessages(claudeReq.Messages, systemText)

	// 构建 OpenAI 请求
	openaiReq := &models.OpenAIRequest{
		Model:       openaiModel,
		Messages:    openaiMessages,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		Stream:      claudeReq.Stream,
	}

	// 启用使用量跟踪和推理 - 针对不同提供商的特定设置
	if claudeReq.Stream != nil && *claudeReq.Stream {
		provider := cfg.DetectProvider()

		switch provider {
		case config.ProviderOpenRouter:
			// OpenRouter 需要启用推理块和使用量跟踪
			// - reasoning.enabled: 在响应中启用思考块
			// - usage.include: 即使在流式模式下也跟踪令牌使用量
			openaiReq.StreamOptions = map[string]interface{}{
				"include_usage": true,
			}
			openaiReq.Usage = map[string]interface{}{
				"include": true,
			}
			openaiReq.Reasoning = map[string]interface{}{
				"enabled": true,
			}

		case config.ProviderOpenAI:
			// OpenAI GPT-5 模型支持 reasoning_effort 参数
			// 此参数控制模型在响应前花多少时间思考
			openaiReq.StreamOptions = map[string]interface{}{
				"include_usage": true,
			}
			openaiReq.ReasoningEffort = "medium" // minimal | low | medium | high

		case config.ProviderOllama:
			// Ollama needs explicit tool_choice when tools are present
			// Without this, Ollama models may not naturally choose to use tools
			if len(claudeReq.Tools) > 0 {
				openaiReq.ToolChoice = "required"
			}
		}
	}

	// Set token limit using adaptive per-model detection
	if claudeReq.MaxTokens > 0 {
		// Use capability-based detection - NO hardcoded model patterns!
		// ShouldUseMaxCompletionTokens checks cached per-model capabilities:
		// - Cache hit: Use learned value (max_completion_tokens or max_tokens)
		// - Cache miss: Try max_completion_tokens first (will auto-detect via retry)
		// This works with ANY model/provider without code changes
		if cfg.ShouldUseMaxCompletionTokens(openaiModel) {
			openaiReq.MaxCompletionTokens = claudeReq.MaxTokens
		} else {
			openaiReq.MaxTokens = claudeReq.MaxTokens
		}
	}

	// Convert stop sequences
	if len(claudeReq.StopSequences) > 0 {
		openaiReq.Stop = claudeReq.StopSequences
	}

	// Convert tools (if present)
	if len(claudeReq.Tools) > 0 {
		openaiReq.Tools = convertTools(claudeReq.Tools)
	}

	return openaiReq, nil
}

// mapModel maps Claude model names to provider-specific models using pattern matching.
// It routes haiku/sonnet/opus tiers to appropriate models (gpt-5-mini, gpt-5, etc.)
// and allows environment variable overrides for routing to alternative providers like
// Grok, Gemini, or DeepSeek. Non-Claude model names are passed through unchanged.
func mapModel(claudeModel string, cfg *config.Config) string {
	modelLower := strings.ToLower(claudeModel)

	// Haiku tier
	if strings.Contains(modelLower, "haiku") {
		if cfg.HaikuModel != "" {
			return cfg.HaikuModel
		}
		return DefaultHaikuModel
	}

	// Sonnet tier
	if strings.Contains(modelLower, "sonnet") {
		if cfg.SonnetModel != "" {
			return cfg.SonnetModel
		}
		return DefaultSonnetModel
	}

	// Opus tier
	if strings.Contains(modelLower, "opus") {
		if cfg.OpusModel != "" {
			return cfg.OpusModel
		}
		return DefaultOpusModel
	}

	// Pass through non-Claude models (OpenAI, OpenRouter, etc.)
	return claudeModel
}

// convertMessages converts Claude messages to OpenAI format.
//
// Handles three content types:
//   - String content: Simple text messages
//   - Array content with blocks: text, tool_use (mapped to tool_calls), and tool_result (mapped to role=tool)
//   - Tool results: Special handling to create OpenAI tool response messages
//
// The function maintains the conversation flow while translating Claude's content block
// structure to OpenAI's message format, ensuring tool call IDs are preserved for correlation.
func convertMessages(claudeMessages []models.ClaudeMessage, system string) []models.OpenAIMessage {
	openaiMessages := []models.OpenAIMessage{}

	// Add system message if present
	if system != "" {
		openaiMessages = append(openaiMessages, models.OpenAIMessage{
			Role:    "system",
			Content: system,
		})
	}

	// Convert each Claude message
	for _, msg := range claudeMessages {
		// Handle content (can be string or array of blocks)
		switch content := msg.Content.(type) {
		case string:
			// Simple text message
			openaiMessages = append(openaiMessages, models.OpenAIMessage{
				Role:    msg.Role,
				Content: content,
			})

		case []interface{}:
			// Handle complex content blocks
			var textParts []string
			var toolCalls []models.OpenAIToolCall
			var hasToolResult bool

			// First pass: check if this is a tool result message
			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if blockMap["type"] == "tool_result" {
						hasToolResult = true
						break
					}
				}
			}

			// Process blocks based on type
			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType := blockMap["type"]

					switch blockType {
					case "text":
						// Extract text content
						if text, ok := blockMap["text"].(string); ok {
							textParts = append(textParts, text)
						}

					case "tool_use":
						// Convert tool_use to OpenAI's tool_calls format
						toolUseID, _ := blockMap["id"].(string)
						toolName, _ := blockMap["name"].(string)
						toolInput := blockMap["input"]

						// Marshal input to JSON string
						var inputJSON string
						if toolInput != nil {
							if inputBytes, err := json.Marshal(toolInput); err == nil {
								inputJSON = string(inputBytes)
							}
						}

						toolCall := models.OpenAIToolCall{
							ID:   toolUseID,
							Type: "function",
						}
						toolCall.Function.Name = toolName
						toolCall.Function.Arguments = inputJSON
						toolCalls = append(toolCalls, toolCall)

					case "tool_result":
						// Convert tool_result to OpenAI's tool message format
						toolUseID, _ := blockMap["tool_use_id"].(string)
						toolContent := ""

						// Extract content from tool result
						if resultContent, ok := blockMap["content"].(string); ok {
							toolContent = resultContent
						} else if resultContent, ok := blockMap["content"].([]interface{}); ok {
							// Handle complex content in tool results
							var contentParts []string
							for _, item := range resultContent {
								if itemMap, ok := item.(map[string]interface{}); ok {
									if itemMap["type"] == "text" {
										if text, ok := itemMap["text"].(string); ok {
											contentParts = append(contentParts, text)
										}
									}
								}
							}
							toolContent = strings.Join(contentParts, "\n")
						}

						openaiMessages = append(openaiMessages, models.OpenAIMessage{
							Role:       "tool",
							Content:    toolContent,
							ToolCallID: toolUseID,
						})
					}
				}
			}

			// Add assistant message with text and/or tool calls
			if len(textParts) > 0 || len(toolCalls) > 0 {
				if !hasToolResult {
					textContent := strings.Join(textParts, "\n")
					openaiMessages = append(openaiMessages, models.OpenAIMessage{
						Role:      msg.Role,
						Content:   textContent,
						ToolCalls: toolCalls,
					})
				}
			}

		default:
			// Unknown content type, try to add as-is
			openaiMessages = append(openaiMessages, models.OpenAIMessage{
				Role:    msg.Role,
				Content: content,
			})
		}
	}

	return openaiMessages
}

// convertTools 将 Claude 工具定义转换为 OpenAI 函数调用格式。
// 将工具名称、描述和 input_schema 映射到 OpenAI 的函数结构。
// 同时在描述中添加参数提示，帮助模型正确使用参数。
func convertTools(claudeTools []models.Tool) []models.OpenAITool {
	openaiTools := make([]models.OpenAITool, len(claudeTools))

	for i, tool := range claudeTools {
		openaiTools[i] = models.OpenAITool{
			Type: "function",
		}
		openaiTools[i].Function.Name = tool.Name
		// 增强工具描述，添加参数使用提示
		openaiTools[i].Function.Description = enhanceToolDescription(tool.Name, tool.Description)
		openaiTools[i].Function.Parameters = tool.InputSchema
	}

	return openaiTools
}

// enhanceToolDescription 增强工具描述，添加参数使用提示
// 这有助于防止模型使用错误的参数名称（如 "query"）
func enhanceToolDescription(toolName, description string) string {
	toolNameLower := strings.ToLower(toolName)

	// 根据工具类型添加参数提示
	var paramHint string
	switch {
	case strings.Contains(toolNameLower, "edit"):
		paramHint = "\n\n【重要】必须使用参数: file_path, old_string, new_string。禁止使用 query 参数。"
	case strings.Contains(toolNameLower, "bash"):
		paramHint = "\n\n【重要】必须使用参数: command。禁止使用 query 参数。"
	case strings.Contains(toolNameLower, "read"):
		paramHint = "\n\n【重要】必须使用参数: file_path。禁止使用 query 参数。"
	case strings.Contains(toolNameLower, "write"):
		paramHint = "\n\n【重要】必须使用参数: file_path, content。禁止使用 query 参数。"
	case strings.Contains(toolNameLower, "grep"):
		paramHint = "\n\n【重要】必须使用参数: pattern。禁止使用 query 参数。"
	case strings.Contains(toolNameLower, "glob"):
		paramHint = "\n\n【重要】必须使用参数: pattern。禁止使用 query 参数。"
	}

	return description + paramHint
}

// ConvertResponse converts an OpenAI response to Claude format
func ConvertResponse(openaiResp *models.OpenAIResponse, requestedModel string) (*models.ClaudeResponse, error) {
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	choice := openaiResp.Choices[0]

	// Convert content to Claude format
	var contentBlocks []models.ContentBlock

	// Handle reasoning_details (convert to thinking blocks)
	// This must come BEFORE other content blocks
	if len(choice.Message.ReasoningDetails) > 0 {
		emptySignature := "" // 用于创建空字符串指针
		for _, reasoningDetail := range choice.Message.ReasoningDetails {
			if detailMap, ok := reasoningDetail.(map[string]interface{}); ok {
				thinkingText := extractReasoningText(detailMap)
				if thinkingText != "" {
					contentBlocks = append(contentBlocks, models.ContentBlock{
						Type:      "thinking",
						Thinking:  thinkingText,
						Signature: &emptySignature, // Required for Claude Code to hide/show thinking
					})
				}
			}
		}
	}

	// Handle text content
	if choice.Message.Content != nil {
		// Handle string content
		if contentStr, ok := choice.Message.Content.(string); ok && contentStr != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "text",
				Text: contentStr,
			})
		}

		// Handle array content (native Claude format from some providers)
		// Some OpenAI-compatible APIs may directly return Claude-style content blocks
		if contentArr, ok := choice.Message.Content.([]interface{}); ok {
			for _, block := range contentArr {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType, _ := blockMap["type"].(string)
					switch blockType {
					case "thinking":
						// Native Claude thinking block
						if thinking, ok := blockMap["thinking"].(string); ok {
							emptySignature := "" // 用于创建空字符串指针
							contentBlocks = append(contentBlocks, models.ContentBlock{
								Type:      "thinking",
								Thinking:  thinking,
								Signature: &emptySignature, // Required for Claude Code
							})
						}
					case "text":
						// Native Claude text block
						if text, ok := blockMap["text"].(string); ok {
							contentBlocks = append(contentBlocks, models.ContentBlock{
								Type: "text",
								Text: text,
							})
						}
					}
				}
			}
		}
	}

	// Handle tool calls (convert to tool_use blocks)
	for _, toolCall := range choice.Message.ToolCalls {
		contentBlocks = append(contentBlocks, models.ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: sanitizeToolInput(toolCall.Function.Name, toolCall.Function.Arguments),
		})
	}

	// Convert finish reason
	var stopReason *string
	if choice.FinishReason != nil {
		reason := convertFinishReason(*choice.FinishReason)
		stopReason = &reason
	}

	// Build Claude response
	claudeResp := &models.ClaudeResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		Model:      requestedModel, // Use original requested model
		StopReason: stopReason,
		Usage: models.Usage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	return claudeResp, nil
}

// convertFinishReason maps OpenAI finish reasons to Claude format
func convertFinishReason(openaiReason string) string {
	switch openaiReason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn" // Claude doesn't have exact equivalent
	default:
		return "end_turn"
	}
}

// sanitizeToolInput fixes common model errors where parameters are malformed.
// Specifically handles the issue where "query" parameter is hallucinated instead of
// the correct required parameters (file_path, command, pattern, etc.).
// This function ALWAYS removes the "query" parameter as it's never valid for any tool.
func sanitizeToolInput(toolName string, argsJSON string) interface{} {
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		// If it's not a JSON object, return as is (likely a string or primitive)
		return argsJSON
	}

	// Always sanitize the input regardless of query presence
	return SanitizeToolArgs(toolName, input)
}

// SanitizeToolArgs fixes tool arguments by removing invalid "query" parameter
// and mapping it to the correct required parameter based on tool type.
// This is exported so it can be used in both streaming and non-streaming handlers.
//
// This function handles several scenarios:
// 1. Model sends {"query": "..."} instead of proper parameters
// 2. Model sends {"query": "{...}"} with JSON-encoded parameters inside
// 3. Model sends correct parameters with an extra "query" field
func SanitizeToolArgs(toolName string, input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return input
	}

	// Normalize tool name to lowercase for matching
	toolNameLower := strings.ToLower(toolName)

	// Extract and remove query parameter if present (case-insensitive)
	var queryContent string
	for key, val := range input {
		if strings.ToLower(key) == "query" {
			if str, ok := val.(string); ok {
				queryContent = str
			}
			delete(input, key)
		}
	}

	// If no query was found, just return the input as-is
	if queryContent == "" {
		return input
	}

	// First, try to parse query as JSON object and merge into input
	// This handles cases where model sends {"query": "{\"file_path\":\"...\", \"old_string\":\"...\"}"}
	if strings.HasPrefix(strings.TrimSpace(queryContent), "{") {
		var parsedQuery map[string]interface{}
		if err := json.Unmarshal([]byte(queryContent), &parsedQuery); err == nil {
			// Merge parsed query into input (don't overwrite existing keys)
			for k, v := range parsedQuery {
				if _, exists := input[k]; !exists {
					input[k] = v
				}
			}
			// After merging, if we have all required params, return
			if hasRequiredParams(toolNameLower, input) {
				return input
			}
		}
	}

	// Map query content to the correct required parameter based on tool type
	// Using contains for fuzzy matching to handle variations like "mcp__xxx__Edit"
	switch {
	// Edit tool: requires file_path, old_string, new_string
	case strings.Contains(toolNameLower, "edit"):
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = queryContent
		}

	// Grep tool: requires pattern
	case strings.Contains(toolNameLower, "grep"):
		if _, ok := input["pattern"]; !ok {
			input["pattern"] = queryContent
		}
		// Also set default path if missing
		if _, ok := input["path"]; !ok {
			input["path"] = "."
		}

	// Bash tool: requires command
	case strings.Contains(toolNameLower, "bash"):
		if _, ok := input["command"]; !ok {
			input["command"] = queryContent
		}

	// Read/ReadFile tool: requires file_path
	case strings.Contains(toolNameLower, "read"):
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = queryContent
		}

	// Write/WriteFile tool: requires file_path, content
	case strings.Contains(toolNameLower, "write"):
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = queryContent
		}

	// Glob tool: requires pattern
	case strings.Contains(toolNameLower, "glob"):
		if _, ok := input["pattern"]; !ok {
			input["pattern"] = queryContent
		}

	// LSP tool: requires filePath
	case strings.Contains(toolNameLower, "lsp"):
		if _, ok := input["filePath"]; !ok {
			input["filePath"] = queryContent
		}

	// Task tool: use prompt (但不处理TodoWrite，因为它需要todos数组)
	case strings.Contains(toolNameLower, "task") && !strings.Contains(toolNameLower, "todo"):
		if _, ok := input["prompt"]; !ok {
			input["prompt"] = queryContent
		}

	// WebFetch/WebSearch: use url or query as appropriate
	case strings.Contains(toolNameLower, "webfetch") || strings.Contains(toolNameLower, "fetch"):
		if _, ok := input["url"]; !ok {
			input["url"] = queryContent
		}
	case strings.Contains(toolNameLower, "websearch") || strings.Contains(toolNameLower, "search"):
		// WebSearch actually uses "query" - but we already removed it, so restore it
		input["query"] = queryContent
	}

	return input
}

// hasRequiredParams checks if the input has the minimum required parameters for a tool
func hasRequiredParams(toolNameLower string, input map[string]interface{}) bool {
	switch {
	case strings.Contains(toolNameLower, "edit"):
		_, hasFilePath := input["file_path"]
		_, hasOldString := input["old_string"]
		_, hasNewString := input["new_string"]
		return hasFilePath && hasOldString && hasNewString
	case strings.Contains(toolNameLower, "bash"):
		_, hasCommand := input["command"]
		return hasCommand
	case strings.Contains(toolNameLower, "read"):
		_, hasFilePath := input["file_path"]
		return hasFilePath
	case strings.Contains(toolNameLower, "grep"):
		_, hasPattern := input["pattern"]
		return hasPattern
	case strings.Contains(toolNameLower, "glob"):
		_, hasPattern := input["pattern"]
		return hasPattern
	case strings.Contains(toolNameLower, "write"):
		_, hasFilePath := input["file_path"]
		_, hasContent := input["content"]
		return hasFilePath && hasContent
	default:
		return true
	}
}
