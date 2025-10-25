package converter

import (
	"fmt"
	"strings"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/pkg/models"
)

// extractSystemText extracts system text from either string or array format
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
		// For encrypted reasoning, return a placeholder or skip
		// Some models like OpenAI o-series return [REDACTED]
		if data, ok := detail["data"].(string); ok {
			return data
		}
		return "[Reasoning redacted by model provider]"
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

	// Enable usage tracking for streaming
	// Support both OpenAI standard and OpenRouter formats
	if claudeReq.Stream != nil && *claudeReq.Stream {
		// OpenAI standard format
		openaiReq.StreamOptions = map[string]interface{}{
			"include_usage": true,
		}
		// OpenRouter format
		openaiReq.Usage = map[string]interface{}{
			"include": true,
		}
		// Enable reasoning tokens (thinking) - OpenRouter format
		// This enables extended thinking for models that support it
		openaiReq.Reasoning = map[string]interface{}{
			"enabled": true, // Enable with default medium effort
		}
	}

	// Set token limit
	if claudeReq.MaxTokens > 0 {
		// Use max_completion_tokens for newer models
		if strings.HasPrefix(openaiModel, "gpt-5") {
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

// mapModel implements pattern-based model routing
func mapModel(claudeModel string, cfg *config.Config) string {
	modelLower := strings.ToLower(claudeModel)

	// Haiku tier
	if strings.Contains(modelLower, "haiku") {
		if cfg.HaikuModel != "" {
			return cfg.HaikuModel
		}
		return "gpt-5-mini"
	}

	// Sonnet tier - version-aware routing
	if strings.Contains(modelLower, "sonnet") {
		// Check for explicit env override
		if cfg.SonnetModel != "" {
			return cfg.SonnetModel
		}

		// Pattern-based routing by version
		// Check for Sonnet 3.x first (e.g., claude-3-5-sonnet, claude-3-sonnet)
		if strings.Contains(modelLower, "claude-3") || strings.Contains(modelLower, "sonnet-3") {
			return "gpt-4o" // Sonnet 3.x → GPT-4o
		}

		// Check for Sonnet 4/5 (newer)
		if strings.Contains(modelLower, "claude-4") || strings.Contains(modelLower, "claude-5") ||
			strings.Contains(modelLower, "sonnet-4") || strings.Contains(modelLower, "sonnet-5") {
			return "gpt-5" // Newer Sonnet → GPT-5
		}

		// Default fallback for unversioned "sonnet"
		return "gpt-5"
	}

	// Opus tier
	if strings.Contains(modelLower, "opus") {
		if cfg.OpusModel != "" {
			return cfg.OpusModel
		}
		return "gpt-5"
	}

	// Pass through non-Claude models (OpenAI, OpenRouter, etc.)
	return claudeModel
}

// convertMessages converts Claude messages to OpenAI format
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

			// If there were text parts and this isn't a tool result message, add as regular message
			if len(textParts) > 0 && !hasToolResult {
				openaiMessages = append(openaiMessages, models.OpenAIMessage{
					Role:    msg.Role,
					Content: strings.Join(textParts, "\n"),
				})
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

// convertTools converts Claude tools to OpenAI format
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
						Type: "thinking",
						Text: thinkingText,
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
