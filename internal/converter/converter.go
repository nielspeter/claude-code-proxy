// Package converter handles bidirectional conversion between Claude and OpenAI API formats.
//
// It provides functions to convert Claude API requests to OpenAI-compatible format and
// OpenAI responses back to Claude format. This includes mapping models, converting message
// structures, handling tool calls, and extracting thinking blocks from reasoning responses.
package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/pkg/models"
)

// Default model mappings when env overrides are not set
// These can be overridden using:
//   - ANTHROPIC_DEFAULT_OPUS_MODEL
//   - ANTHROPIC_DEFAULT_SONNET_MODEL
//   - ANTHROPIC_DEFAULT_HAIKU_MODEL
const (
	DefaultOpusModel   = "gpt-5"
	DefaultSonnetModel = "gpt-5"
	DefaultHaikuModel  = "gpt-5-mini"
)

// isReasoningModel detects if a model uses reasoning/extended thinking capabilities.
// Reasoning models require max_completion_tokens instead of max_tokens.
// This includes:
//   - OpenAI o-series: o1, o3, o4 (reasoning models)
//   - OpenAI GPT-5 series: gpt-5, gpt-5-mini, etc.
//   - Azure variants: azure/o1, azure/gpt-5, etc.
func isReasoningModel(modelName string) bool {
	model := strings.ToLower(modelName)

	// Remove provider prefixes for pattern matching
	model = strings.TrimPrefix(model, "azure/")
	model = strings.TrimPrefix(model, "openai/")

	// Check for o-series reasoning models (o1, o3, o4, etc.)
	// Matches: o1, o1-preview, o3, o3-mini, o4, etc.
	if strings.HasPrefix(model, "o1") ||
	   strings.HasPrefix(model, "o3") ||
	   strings.HasPrefix(model, "o4") {
		return true
	}

	// Check for GPT-5 series (gpt-5, gpt-5-mini, gpt-5-turbo, etc.)
	if strings.HasPrefix(model, "gpt-5") || strings.Contains(model, "/gpt-5") {
		return true
	}

	return false
}

// extractSystemText extracts system text from Claude's flexible system parameter.
// Claude supports both string format ("system": "text") and array format with content blocks.
// This function normalizes both formats to a single string for OpenAI compatibility.
func extractSystemText(system interface{}) string {
	if system == nil {
		return ""
	}

	// Handle string format
	if systemStr, ok := system.(string); ok {
		return systemStr
	}

	// Handle array format
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

// extractReasoningText extracts text from OpenRouter reasoning_details
// Handles different reasoning detail types: reasoning.text, reasoning.summary, reasoning.encrypted
func extractReasoningText(detail map[string]interface{}) string {
	detailType, _ := detail["type"].(string)

	switch detailType {
	case "reasoning.text":
		// Extract text field
		if text, ok := detail["text"].(string); ok {
			return text
		}
	case "reasoning.summary":
		// Extract summary field
		if summary, ok := detail["summary"].(string); ok {
			return summary
		}
	case "reasoning.encrypted":
		// Skip encrypted reasoning - it's base64 encrypted data not meant to be shown
		// Models like Grok send this alongside reasoning.summary
		return ""
	}

	return ""
}

// ConvertRequest converts a Claude API request to OpenAI format
func ConvertRequest(claudeReq models.ClaudeRequest, cfg *config.Config) (*models.OpenAIRequest, error) {
	// Map model using pattern-based routing
	openaiModel := mapModel(claudeReq.Model, cfg)

	// Extract system message (can be string or array of content blocks)
	systemText := extractSystemText(claudeReq.System)

	// Convert messages
	openaiMessages := convertMessages(claudeReq.Messages, systemText)

	// Build OpenAI request
	openaiReq := &models.OpenAIRequest{
		Model:       openaiModel,
		Messages:    openaiMessages,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		Stream:      claudeReq.Stream,
	}

	// Enable usage tracking and reasoning - provider-specific
	if claudeReq.Stream != nil && *claudeReq.Stream {
		provider := cfg.DetectProvider()

		switch provider {
		case config.ProviderOpenRouter:
			// OpenRouter needs reasoning blocks and usage tracking enabled
			// - reasoning.enabled: Enables thinking blocks in response
			// - usage.include: Tracks token usage even in streaming mode
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
			// OpenAI GPT-5 models support reasoning_effort parameter
			// This controls how much time the model spends thinking before responding
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

	// Set token limit
	if claudeReq.MaxTokens > 0 {
		// Reasoning models (o1, o3, o4, gpt-5) require max_completion_tokens
		// instead of the legacy max_tokens parameter
		if isReasoningModel(openaiModel) {
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

// convertTools converts Claude tool definitions to OpenAI function calling format.
// Maps tool name, description, and input_schema to OpenAI's function structure.
func convertTools(claudeTools []models.Tool) []models.OpenAITool {
	openaiTools := make([]models.OpenAITool, len(claudeTools))

	for i, tool := range claudeTools {
		openaiTools[i] = models.OpenAITool{
			Type: "function",
		}
		openaiTools[i].Function.Name = tool.Name
		openaiTools[i].Function.Description = tool.Description
		openaiTools[i].Function.Parameters = tool.InputSchema
	}

	return openaiTools
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
		for _, reasoningDetail := range choice.Message.ReasoningDetails {
			if detailMap, ok := reasoningDetail.(map[string]interface{}); ok {
				thinkingText := extractReasoningText(detailMap)
				if thinkingText != "" {
					contentBlocks = append(contentBlocks, models.ContentBlock{
						Type:     "thinking",
						Thinking: thinkingText, // Use Thinking field, not Text
					})
				}
			}
		}
	}

	// Handle text content
	if choice.Message.Content != nil {
		if contentStr, ok := choice.Message.Content.(string); ok && contentStr != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "text",
				Text: contentStr,
			})
		}
	}

	// Handle tool calls (convert to tool_use blocks)
	for _, toolCall := range choice.Message.ToolCalls {
		contentBlocks = append(contentBlocks, models.ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: toolCall.Function.Arguments, // OpenAI sends as JSON string
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
