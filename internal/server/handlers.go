package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-code-proxy/internal/config"
	"claude-code-proxy/internal/converter"
	"claude-code-proxy/pkg/models"
	"github.com/gofiber/fiber/v2"
)

// addOpenRouterHeaders 添加 OpenRouter 特定的 HTTP 头以获得更好的速率限制。
// 配置后设置 HTTP-Referer 和 X-Title 头，有助于 OpenRouter 的速率限制和使用量跟踪。
func addOpenRouterHeaders(req *http.Request, cfg *config.Config) {
	if cfg.OpenRouterAppURL != "" {
		req.Header.Set("HTTP-Referer", cfg.OpenRouterAppURL)
	}
	if cfg.OpenRouterAppName != "" {
		req.Header.Set("X-Title", cfg.OpenRouterAppName)
	}
}

// handleMessages 是 /v1/messages 端点的主处理器。
// 解析 Claude 请求，转换为 OpenAI 格式，并根据请求的 stream 参数路由到流式或非流式处理器。
func handleMessages(c *fiber.Ctx, cfg *config.Config) error {
	// 调试：记录原始请求
	if cfg.Debug {
		fmt.Printf("\n=== Claude 请求 ===\n%s\n===================\n", string(c.Body()))
	}

	// 解析 Claude 请求
	var claudeReq models.ClaudeRequest
	if err := c.BodyParser(&claudeReq); err != nil {
		// 记录错误和原始请求体以便调试
		fmt.Printf("[错误] 解析请求体失败: %v\n", err)
		fmt.Printf("[错误] 原始请求体: %s\n", string(c.Body()))
		return c.Status(400).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "invalid_request_error",
				"message": fmt.Sprintf("Invalid request body: %v", err),
			},
		})
	}

	// 验证 API 密钥（如果已配置）
	if cfg.AnthropicAPIKey != "" {
		apiKey := c.Get("x-api-key")
		if apiKey != cfg.AnthropicAPIKey {
			return c.Status(401).JSON(fiber.Map{
				"type": "error",
				"error": fiber.Map{
					"type":    "authentication_error",
					"message": "API 密钥无效",
				},
			})
		}
	}

	// 将 Claude 请求转换为 OpenAI 格式
	openaiReq, err := converter.ConvertRequest(claudeReq, cfg)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "invalid_request_error",
				"message": err.Error(),
			},
		})
	}

	// 注入指令以防止模型使用无效的 "query" 参数
	// 这对于通过 OpenAI 兼容 API 访问的 Claude 模型至关重要
	if len(openaiReq.Messages) > 0 {
		instruction := `

【关键工具参数要求】
使用工具时，必须使用每个工具 schema 中定义的确切参数名称。任何工具都不存在 "query" 参数。

各工具必需参数：
- Edit: file_path, old_string, new_string（三个都必需）
- Read: file_path（必需）
- Write: file_path, content（两个都必需）
- Bash: command（必需）
- Grep: pattern（必需）
- Glob: pattern（必需）
- LSP: operation, filePath, line, character（全部必需）
- Task: description, prompt, subagent_type（全部必需）
- WebFetch: url, prompt（两个都必需）

绝对不要使用 "query" 作为参数名称 - 这会导致工具执行失败。`

		// 如果第一条消息是系统消息，则追加到其中
		if openaiReq.Messages[0].Role == "system" {
			// 将 interface{} 类型断言为 string
			contentStr, _ := openaiReq.Messages[0].Content.(string)
			openaiReq.Messages[0].Content = contentStr + instruction
		} else {
			// 否则在前面添加新的系统消息
			openaiReq.Messages = append([]models.OpenAIMessage{
				{Role: "system", Content: instruction},
			}, openaiReq.Messages...)
		}
	}

	// 调试：记录转换后的 OpenAI 请求
	if cfg.Debug {
		openaiReqJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
		fmt.Printf("\n=== OpenAI 请求 ===\n%s\n===================\n", string(openaiReqJSON))
		if len(claudeReq.Tools) > 0 {
			fmt.Printf("[调试] 请求包含 %d 个工具\n", len(claudeReq.Tools))
			for i, tool := range openaiReq.Tools {
				fmt.Printf("[调试] 工具 %d: %s\n", i, tool.Function.Name)
			}
		}
	}

	// 调试：检查 Stream 字段
	if cfg.Debug {
		if openaiReq.Stream == nil {
			fmt.Printf("[调试] Stream 字段为 nil\n")
		} else {
			fmt.Printf("[调试] Stream 字段 = %v\n", *openaiReq.Stream)
		}
	}

	// 处理流式与非流式请求
	if openaiReq.Stream != nil && *openaiReq.Stream {
		return handleStreamingMessages(c, openaiReq, cfg)
	}

	// 记录计时用于简单日志
	startTime := time.Now()

	// 非流式响应
	openaiResp, err := callOpenAI(openaiReq, cfg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "api_error",
				"message": fmt.Sprintf("OpenAI API 错误: %v", err),
			},
		})
	}

	// 调试：记录 OpenAI 响应
	if cfg.Debug {
		openaiRespJSON, _ := json.MarshalIndent(openaiResp, "", "  ")
		fmt.Printf("\n=== OpenAI 响应 ===\n%s\n====================\n", string(openaiRespJSON))
		if len(openaiResp.Choices) > 0 {
			choice := openaiResp.Choices[0]
			fmt.Printf("[调试] OpenAI 响应包含 %d 个工具调用\n", len(choice.Message.ToolCalls))
			for i, tc := range choice.Message.ToolCalls {
				fmt.Printf("[调试] 工具调用 %d: ID=%s, 名称=%s\n", i, tc.ID, tc.Function.Name)
			}
		}
	}

	// 将 OpenAI 响应转换为 Claude 格式
	claudeResp, err := converter.ConvertResponse(openaiResp, claudeReq.Model)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "api_error",
				"message": fmt.Sprintf("响应转换错误: %v", err),
			},
		})
	}

	// 调试：记录 Claude 响应
	if cfg.Debug {
		claudeRespJSON, _ := json.MarshalIndent(claudeResp, "", "  ")
		fmt.Printf("\n=== Claude 响应 ===\n%s\n====================\n\n", string(claudeRespJSON))
		fmt.Printf("[调试] Claude 响应包含 %d 个内容块\n", len(claudeResp.Content))
		for i, block := range claudeResp.Content {
			fmt.Printf("[调试] 块 %d: 类型=%s", i, block.Type)
			if block.Type == "tool_use" {
				fmt.Printf(", 名称=%s, ID=%s", block.Name, block.ID)
			}
			fmt.Printf("\n")
		}
	}

	// 简单日志：单行摘要
	if cfg.SimpleLog {
		duration := time.Since(startTime).Seconds()
		tokensPerSec := 0.0
		if duration > 0 && claudeResp.Usage.OutputTokens > 0 {
			tokensPerSec = float64(claudeResp.Usage.OutputTokens) / duration
		}
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] [请求] %s 模型=%s 输入=%d 输出=%d 令牌/秒=%.1f\n",
			timestamp,
			cfg.OpenAIBaseURL,
			openaiReq.Model,
			claudeResp.Usage.InputTokens,
			claudeResp.Usage.OutputTokens,
			tokensPerSec)
	}

	// 在非流式模式下清理工具参数（移除无效的 query 参数）
	for i := range claudeResp.Content {
		if claudeResp.Content[i].Type == "tool_use" {
			// 将 block.Input 类型断言为 map 以进行清理
			if args, ok := claudeResp.Content[i].Input.(map[string]interface{}); ok {
				claudeResp.Content[i].Input = converter.SanitizeToolArgs(claudeResp.Content[i].Name, args)
			}
		}
	}

	return c.JSON(claudeResp)
}

// handleStreamingMessages 处理来自提供商的流式 SSE 响应。
// 转发 OpenAI 请求，接收流式数据块，并使用 streamOpenAIToClaude 实时转换为 Claude 的 SSE 事件格式。
func handleStreamingMessages(c *fiber.Ctx, openaiReq *models.OpenAIRequest, cfg *config.Config) error {
	// 记录计时用于简单日志
	startTime := time.Now()

	// 设置 SSE 头
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		if cfg.Debug {
			fmt.Printf("[调试] 流写入器：开始\n")
		}

		if cfg.Debug {
			fmt.Printf("[调试] 流写入器：正在向 %s 发送流式请求\n", cfg.OpenAIBaseURL+"/chat/completions")
		}

		// 使用自动重试逻辑发送流式请求
		resp, err := callOpenAIStream(openaiReq, cfg)
		if err != nil {
			if cfg.Debug {
				fmt.Printf("[调试] 流写入器：请求失败: %v\n", err)
			}
			writeSSEError(w, fmt.Sprintf("流式请求失败: %v", err))
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if cfg.Debug {
			fmt.Printf("[调试] 流写入器：收到响应，开始 streamOpenAIToClaude 转换\n")
		}

		// 流式转换
		streamOpenAIToClaude(w, resp.Body, openaiReq.Model, cfg, startTime)

		if cfg.Debug {
			fmt.Printf("[调试] 流写入器：完成\n")
		}
	})

	return nil
}

// ToolCallState 跟踪流式传输期间工具调用的状态
type ToolCallState struct {
	ID          string // 来自 OpenAI 的工具调用 ID
	Name        string // 函数名称
	ArgsBuffer  string // 累积的 JSON 参数
	JSONSent    bool   // 是否已发送 JSON delta 的标志
	ClaudeIndex int    // Claude 的内容块索引
	Started     bool   // 是否已发送 content_block_start 的标志
}

// streamOpenAIToClaude 将 OpenAI 流式响应转换为 Claude 的 SSE 事件格式。
func streamOpenAIToClaude(w *bufio.Writer, reader io.Reader, providerModel string, cfg *config.Config, startTime time.Time) {
	if cfg.Debug {
		fmt.Printf("[调试] streamOpenAIToClaude：开始转换\n")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 增加缓冲区大小

	// 状态变量
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	// 使用动态自增索引防止索引不连续问题
	nextIndex := 0
	textBlockIndex := -1     // -1 表示尚未分配
	thinkingBlockIndex := -1 // -1 表示尚未分配

	currentToolCalls := make(map[int]*ToolCallState)
	finalStopReason := "end_turn"
	usageData := map[string]interface{}{
		"input_tokens":                0,
		"output_tokens":               0,
		"cache_creation_input_tokens": 0,
		"cache_read_input_tokens":     0,
		"cache_creation": map[string]interface{}{
			"ephemeral_5m_input_tokens": 0,
			"ephemeral_1h_input_tokens": 0,
		},
	}

	// 思考块跟踪（用于在 Claude Code 中显示思考指示器）
	thinkingBlockStarted := false
	thinkingBlockHasContent := false
	textBlockStarted := false // 跟踪是否已发送文本块的 block_start

	// 发送初始 SSE 事件
	writeSSEEvent(w, "message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         providerModel,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":                0,
				"output_tokens":               0,
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens":     0,
				"cache_creation": map[string]interface{}{
					"ephemeral_5m_input_tokens": 0,
					"ephemeral_1h_input_tokens": 0,
				},
			},
		},
	})

	writeSSEEvent(w, "ping", map[string]interface{}{
		"type": "ping",
	})

	_ = w.Flush()

	// 处理流式数据块
	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 检查 [DONE] 标记
		if strings.Contains(line, "[DONE]") {
			break
		}

		// 解析数据行
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		dataJSON := strings.TrimPrefix(line, "data: ")

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataJSON), &chunk); err != nil {
			continue
		}

		// 记录每个数据块以查看 OpenRouter 发送的内容
		if cfg.Debug {
			fmt.Printf("[调试] 来自 OpenRouter 的原始数据块: %s\n", dataJSON)
		}

		// 处理使用量数据
		if usage, ok := chunk["usage"].(map[string]interface{}); ok {
			if cfg.Debug {
				usageJSON, _ := json.Marshal(usage)
				fmt.Printf("[调试] 收到来自 OpenAI 的使用量: %s\n", string(usageJSON))
			}

			// 将 float64 转换为 int（JSON 将数字解析为 float64）
			inputTokens := 0
			outputTokens := 0
			if val, ok := usage["prompt_tokens"].(float64); ok {
				inputTokens = int(val)
			}
			if val, ok := usage["completion_tokens"].(float64); ok {
				outputTokens = int(val)
			}

			usageData = map[string]interface{}{
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
			}

			// 如果存在缓存指标则添加
			if promptTokensDetails, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
				if cachedTokens, ok := promptTokensDetails["cached_tokens"].(float64); ok && cachedTokens > 0 {
					usageData["cache_read_input_tokens"] = int(cachedTokens)
				}
			}
			if cfg.Debug {
				usageDataJSON, _ := json.Marshal(usageData)
				fmt.Printf("[调试] 累积的使用量数据: %s\n", string(usageDataJSON))
			}
		}

		// 从 choices 中提取 delta
		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}

		choice := choices[0].(map[string]interface{})
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		// 处理推理 delta（思考块）
		// 支持 OpenRouter 和 OpenAI 两种格式：
		// - OpenRouter: delta.reasoning_details（数组）
		// - OpenAI o1/o3: delta.reasoning_content（字符串）

		// 首先检查 OpenAI 的 reasoning_content 格式（o1/o3 模型）
		if reasoningContent, ok := delta["reasoning_content"].(string); ok && reasoningContent != "" {
			// 在第一个思考 delta 时发送思考块的 content_block_start
			if !thinkingBlockStarted {
				thinkingBlockIndex = nextIndex
				nextIndex++

				writeSSEEvent(w, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": thinkingBlockIndex,
					"content_block": map[string]interface{}{
						"type":      "thinking",
						"thinking":  "", // 必需，防止思考块验证失败
						"signature": "", // 必需，让 Claude Code 正确隐藏/显示思考块
					},
				})
				thinkingBlockStarted = true
				thinkingBlockHasContent = true
			}

			// 发送思考 delta
			writeSSEEvent(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": thinkingBlockIndex,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": reasoningContent,
				},
			})
		}

		// 然后检查 OpenRouter 的 reasoning_details 格式
		// 仅在尚未处理 reasoning 字段时处理 reasoning_details
		if reasoningDetailsRaw, ok := delta["reasoning_details"]; ok && delta["reasoning"] == nil {
			if reasoningDetails, ok := reasoningDetailsRaw.([]interface{}); ok && len(reasoningDetails) > 0 {
				for _, detailRaw := range reasoningDetails {
					if detail, ok := detailRaw.(map[string]interface{}); ok {
						// 从详情中提取推理文本
						thinkingText := ""
						detailType, _ := detail["type"].(string)

						switch detailType {
						case "reasoning.text":
							if text, ok := detail["text"].(string); ok {
								thinkingText = text
							}
						case "reasoning.summary":
							if summary, ok := detail["summary"].(string); ok {
								thinkingText = summary
							}
						case "reasoning.encrypted":
							// 在流式传输中跳过加密/编辑的推理
							continue
						}

						if thinkingText != "" {
							// 在第一个思考 delta 时发送思考块的 content_block_start
							if !thinkingBlockStarted {
								thinkingBlockIndex = nextIndex
								nextIndex++

								writeSSEEvent(w, "content_block_start", map[string]interface{}{
									"type":  "content_block_start",
									"index": thinkingBlockIndex,
									"content_block": map[string]interface{}{
										"type":      "thinking",
										"thinking":  "",
										"signature": "", // 必需，让 Claude Code 正确隐藏/显示思考块
									},
								})
								thinkingBlockStarted = true
								_ = w.Flush()
							}

							// 发送思考块 delta
							writeSSEEvent(w, "content_block_delta", map[string]interface{}{
								"type":  "content_block_delta",
								"index": thinkingBlockIndex,
								"delta": map[string]interface{}{
									"type":     "thinking_delta",
									"thinking": thinkingText,
								},
							})
							thinkingBlockHasContent = true
							_ = w.Flush()
						}
					}
				}
			}
		}

		// 直接处理 reasoning 字段（某些模型的简化格式）
		if reasoning, ok := delta["reasoning"].(string); ok && reasoning != "" {
			// 在第一个思考 delta 时发送思考块的 content_block_start
			if !thinkingBlockStarted {
				thinkingBlockIndex = nextIndex
				nextIndex++

				writeSSEEvent(w, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": thinkingBlockIndex,
					"content_block": map[string]interface{}{
						"type":      "thinking",
						"thinking":  "",
						"signature": "", // 必需，让 Claude Code 正确隐藏/显示思考块
					},
				})
				thinkingBlockStarted = true
				_ = w.Flush()
			}

			// 发送思考块 delta
			writeSSEEvent(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": thinkingBlockIndex,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": reasoning,
				},
			})
			thinkingBlockHasContent = true
			_ = w.Flush()
		}

		// 处理文本 delta
		if content, ok := delta["content"].(string); ok && content != "" {
			// 在第一个文本 delta 时发送文本块的 content_block_start
			if !textBlockStarted {
				textBlockIndex = nextIndex
				nextIndex++

				writeSSEEvent(w, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": textBlockIndex,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				})
				textBlockStarted = true
				_ = w.Flush()
			}

			writeSSEEvent(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": textBlockIndex,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": content,
				},
			})
			_ = w.Flush()
		}

		// 处理工具调用 delta
		if toolCallsRaw, ok := delta["tool_calls"]; ok {
			// 调试：记录来自提供商的原始 tool_calls
			if cfg.Debug {
				toolCallsJSON, _ := json.Marshal(toolCallsRaw)
				fmt.Printf("[调试] 原始 tool_calls delta: %s\n", string(toolCallsJSON))
			}

			toolCalls, ok := toolCallsRaw.([]interface{})
			if ok && len(toolCalls) > 0 {
				for _, tcRaw := range toolCalls {
					tcDelta, ok := tcRaw.(map[string]interface{})
					if !ok {
						continue
					}

					// 获取工具调用索引
					tcIndex := 0
					if idx, ok := tcDelta["index"].(float64); ok {
						tcIndex = int(idx)
					}

					// 如果不存在则初始化工具调用跟踪
					if _, exists := currentToolCalls[tcIndex]; !exists {
						currentToolCalls[tcIndex] = &ToolCallState{
							ID:          "",
							Name:        "",
							ArgsBuffer:  "",
							JSONSent:    false,
							ClaudeIndex: 0,
							Started:     false,
						}
					}

					toolCall := currentToolCalls[tcIndex]

					// 如果提供了工具调用 ID 则更新
					if id, ok := tcDelta["id"].(string); ok {
						toolCall.ID = id
					}

					// 如果没有启动文本块，在工具块之前创建占位符
					if !textBlockStarted {
						textBlockIndex = nextIndex
						nextIndex++

						writeSSEEvent(w, "content_block_start", map[string]interface{}{
							"type":  "content_block_start",
							"index": textBlockIndex,
							"content_block": map[string]interface{}{
								"type": "text",
								"text": "",
							},
						})
						// 发送占位文本，防止出现 "(no content)" 错误
						writeSSEEvent(w, "content_block_delta", map[string]interface{}{
							"type":  "content_block_delta",
							"index": textBlockIndex,
							"delta": map[string]interface{}{
								"type": "text_delta",
								"text": "正在调用工具：",
							},
						})
						textBlockStarted = true
						_ = w.Flush()
					}

					// 更新函数名称
					if functionData, ok := tcDelta["function"].(map[string]interface{}); ok {
						if name, ok := functionData["name"].(string); ok {
							toolCall.Name = name
						}

						// 当有完整的初始数据时启动内容块
						if toolCall.ID != "" && toolCall.Name != "" && !toolCall.Started {
							toolCall.ClaudeIndex = nextIndex
							nextIndex++
							toolCall.Started = true

							writeSSEEvent(w, "content_block_start", map[string]interface{}{
								"type":  "content_block_start",
								"index": toolCall.ClaudeIndex,
								"content_block": map[string]interface{}{
									"type":  "tool_use",
									"id":    toolCall.ID,
									"name":  toolCall.Name,
									"input": map[string]interface{}{},
								},
							})
							_ = w.Flush()
						}

						// 处理函数参数
						// 始终累积参数，不管Started状态 - 在流结束时发送完整的 JSON
						// 修复：移除 toolCall.Started 条件，避免在ID/Name到达前丢失参数
						if args, ok := functionData["arguments"].(string); ok {
							if args != "" {
								toolCall.ArgsBuffer += args
							}
						}
					}
				}
			}
		}

		// 处理完成原因
		// 注意：不要在此处中断 - 使用 stream_options.include_usage 时，OpenAI 会在 finish_reason 之后的数据块中发送使用量
		if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
			switch finishReason {
			case "length":
				finalStopReason = "max_tokens"
			case "tool_calls", "function_call":
				finalStopReason = "tool_use"
			case "stop":
				finalStopReason = "end_turn"
			default:
				finalStopReason = "end_turn"
			}
			// 继续处理以捕获使用量数据块（不中断）
		}
	}

	// 发送最终 SSE 事件

	// 如果文本块已启动，发送 content_block_stop
	if textBlockStarted && textBlockIndex != -1 {
		writeSSEEvent(w, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": textBlockIndex,
		})
		_ = w.Flush()
	}

	// 为每个工具调用发送最终 JSON 和 content_block_stop
	for _, toolData := range currentToolCalls {
		// 检查 Started 和 claude_index 是否有效
		if toolData.Started && toolData.ClaudeIndex != -1 {
			// 在关闭块之前发送完整的 JSON 参数
			if toolData.ArgsBuffer != "" {
				var jsonArgs map[string]interface{}
				if err := json.Unmarshal([]byte(toolData.ArgsBuffer), &jsonArgs); err == nil {
					// 清理工具参数（移除无效的 query 参数）
					sanitizedArgs := converter.SanitizeToolArgs(toolData.Name, jsonArgs)
					sanitizedJSON, _ := json.Marshal(sanitizedArgs)

					writeSSEEvent(w, "content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": toolData.ClaudeIndex,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": string(sanitizedJSON),
						},
					})
					_ = w.Flush()
				}
			}

			writeSSEEvent(w, "content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": toolData.ClaudeIndex,
			})
			_ = w.Flush()
		}
	}

	// 如果思考块有内容，发送 content_block_stop
	if thinkingBlockStarted && thinkingBlockHasContent && thinkingBlockIndex != -1 {
		writeSSEEvent(w, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": thinkingBlockIndex,
		})
		_ = w.Flush()
	}

	// 调试：检查是否收到使用量数据
	if cfg.Debug {
		inputTokens, _ := usageData["input_tokens"].(int)
		outputTokens, _ := usageData["output_tokens"].(int)
		if inputTokens == 0 && outputTokens == 0 {
			fmt.Printf("[调试] OpenRouter 流式传输：使用量数据不可用（流式 API 的预期限制）\n")
		}
	}

	// 发送带有 stop_reason 和累积使用量数据的 message_delta
	// 注意：我们发送实际累积的使用量以修复 Claude Code 中的 "0 tokens" 问题
	if cfg.Debug {
		usageDataJSON, _ := json.Marshal(usageData)
		fmt.Printf("[调试] 发送带有使用量数据的 message_delta: %s\n", string(usageDataJSON))
	}
	writeSSEEvent(w, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   finalStopReason,
			"stop_sequence": nil,
		},
		"usage": usageData,
	})
	_ = w.Flush()

	// 发送 message_stop
	writeSSEEvent(w, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})
	_ = w.Flush()

	// 简单日志：单行摘要
	if cfg.SimpleLog {
		inputTokens := 0
		outputTokens := 0

		// 尝试从各种可能的格式中提取令牌数
		if val, ok := usageData["input_tokens"].(int); ok {
			inputTokens = val
		} else if val, ok := usageData["input_tokens"].(float64); ok {
			inputTokens = int(val)
		}

		if val, ok := usageData["output_tokens"].(int); ok {
			outputTokens = val
		} else if val, ok := usageData["output_tokens"].(float64); ok {
			outputTokens = int(val)
		}

		// 调试：显示 usageData 中的实际内容
		if cfg.Debug {
			fmt.Printf("[调试] 使用量数据: %+v\n", usageData)
		}

		// 计算每秒令牌数
		duration := time.Since(startTime).Seconds()
		tokensPerSec := 0.0
		if duration > 0 && outputTokens > 0 {
			tokensPerSec = float64(outputTokens) / duration
		}

		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] [请求] %s 模型=%s 输入=%d 输出=%d 令牌/秒=%.1f\n",
			timestamp,
			cfg.OpenAIBaseURL,
			providerModel,
			inputTokens,
			outputTokens,
			tokensPerSec)
	}

	// 检查扫描器错误
	if err := scanner.Err(); err != nil {
		writeSSEError(w, fmt.Sprintf("流读取错误: %v", err))
	}
}

// writeSSEEvent 写入服务器发送事件
func writeSSEEvent(w *bufio.Writer, event string, data interface{}) {
	dataJSON, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(dataJSON))
}

// writeSSEError 写入错误事件
func writeSSEError(w *bufio.Writer, message string) {
	writeSSEEvent(w, "error", map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": message,
		},
	})
	_ = w.Flush()
}

// callOpenAI 向 OpenAI API 发送 HTTP 请求，带有自动重试逻辑
// 用于处理 max_completion_tokens 参数错误。使用按模型的能力缓存。
func callOpenAI(req *models.OpenAIRequest, cfg *config.Config) (*models.OpenAIResponse, error) {
	// 使用配置的参数尝试请求
	resp, err := callOpenAIInternal(req, cfg)
	if err != nil {
		// 检查是否是 max_tokens 参数错误
		if isMaxTokensParameterError(err.Error()) {
			if cfg.Debug {
				fmt.Printf("[调试] 检测到模型 %s 的 max_completion_tokens 参数错误，正在重试\n", req.Model)
			}
			// 不使用 max_completion_tokens 重试，并按模型缓存能力
			return retryWithoutMaxCompletionTokens(req, cfg)
		}
		// 其他错误 - 原样返回
		return nil, err
	}

	// 首次尝试成功 - 缓存此（提供商，模型）支持 max_completion_tokens
	// 仅在实际发送了 max_completion_tokens 时缓存
	if req.MaxCompletionTokens > 0 {
		cacheKey := config.CacheKey{
			BaseURL: cfg.OpenAIBaseURL,
			Model:   req.Model,
		}
		config.SetModelCapabilities(cacheKey, &config.ModelCapabilities{
			UsesMaxCompletionTokens: true,
		})
		if cfg.Debug {
			fmt.Printf("[调试] 已缓存：模型 %s 支持 max_completion_tokens\n", req.Model)
		}
	}

	return resp, nil
}

// callOpenAIStream 发送流式 HTTP 请求，带有参数错误重试逻辑。
// 使用按模型的能力缓存。
func callOpenAIStream(req *models.OpenAIRequest, cfg *config.Config) (*http.Response, error) {
	// 使用配置的参数尝试
	resp, err := callOpenAIStreamInternal(req, cfg)
	if err != nil {
		// 检查是否是 max_tokens 参数错误
		if isMaxTokensParameterError(err.Error()) {
			if cfg.Debug {
				fmt.Printf("[调试] 检测到流式模型 %s 的 max_completion_tokens 参数错误，正在重试\n", req.Model)
			}
			// 创建不带 max tokens 的重试请求
			retryReq := *req
			retryReq.MaxCompletionTokens = 0
			retryReq.MaxTokens = 0

			// 缓存此（提供商，模型）不支持 max_completion_tokens
			cacheKey := config.CacheKey{
				BaseURL: cfg.OpenAIBaseURL,
				Model:   req.Model,
			}
			config.SetModelCapabilities(cacheKey, &config.ModelCapabilities{
				UsesMaxCompletionTokens: false,
			})

			return callOpenAIStreamInternal(&retryReq, cfg)
		}
		return nil, err
	}

	// 成功 - 如果发送了 max_completion_tokens 则缓存能力
	if req.MaxCompletionTokens > 0 {
		cacheKey := config.CacheKey{
			BaseURL: cfg.OpenAIBaseURL,
			Model:   req.Model,
		}
		config.SetModelCapabilities(cacheKey, &config.ModelCapabilities{
			UsesMaxCompletionTokens: true,
		})
		if cfg.Debug {
			fmt.Printf("[调试] 已缓存：模型 %s 支持 max_completion_tokens（流式）\n", req.Model)
		}
	}

	return resp, nil
}

// callOpenAIStreamInternal 发送流式 HTTP 请求，不带重试逻辑
func callOpenAIStreamInternal(req *models.OpenAIRequest, cfg *config.Config) (*http.Response, error) {
	// 将请求序列化为 JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 API URL
	apiURL := cfg.OpenAIBaseURL + "/chat/completions"

	// 创建 HTTP 请求
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")

	// 跳过 Ollama（localhost）的认证
	if !cfg.IsLocalhost() {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	}

	// OpenRouter 特定的请求头
	if cfg.DetectProvider() == config.ProviderOpenRouter {
		addOpenRouterHeaders(httpReq, cfg)
	}

	// 创建带有较长超时的 HTTP 客户端用于流式传输
	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	// 检查错误
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("OpenAI API 返回状态码 %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// isMaxTokensParameterError 检查错误消息是否表示不支持的
// max_tokens 或 max_completion_tokens 参数问题。
// 使用广泛的关键字匹配来处理不同提供商的不同错误消息格式。
// 不检查状态码 - 仅依赖消息内容。
func isMaxTokensParameterError(errorMessage string) bool {
	errorLower := strings.ToLower(errorMessage)

	// 检查参数错误指示器
	hasParamIndicator := strings.Contains(errorLower, "parameter") ||
		strings.Contains(errorLower, "unsupported") ||
		strings.Contains(errorLower, "invalid")

	// 检查我们特定的参数名称
	hasOurParam := strings.Contains(errorLower, "max_tokens") ||
		strings.Contains(errorLower, "max_completion_tokens")

	// 需要两个指示器都存在以减少误报
	return hasParamIndicator && hasOurParam
}

// retryWithoutMaxCompletionTokens 尝试不使用 max_completion_tokens 重新发送请求。
// 按（提供商，模型）组合缓存结果以供将来请求使用。
func retryWithoutMaxCompletionTokens(req *models.OpenAIRequest, cfg *config.Config) (*models.OpenAIResponse, error) {
	// 创建不带 max_completion_tokens 的请求副本
	retryReq := *req
	retryReq.MaxCompletionTokens = 0
	retryReq.MaxTokens = 0 // 同时清除 max_tokens 以避免问题

	if cfg.Debug {
		fmt.Printf("[调试] 正在为模型 %s 重试，不使用 max_completion_tokens/max_tokens\n", req.Model)
	}

	// 缓存此特定（提供商，模型）不支持 max_completion_tokens
	cacheKey := config.CacheKey{
		BaseURL: cfg.OpenAIBaseURL,
		Model:   req.Model,
	}
	config.SetModelCapabilities(cacheKey, &config.ModelCapabilities{
		UsesMaxCompletionTokens: false,
	})

	// 发送重试请求
	return callOpenAIInternal(&retryReq, cfg)
}

// callOpenAIInternal 是不带重试逻辑的内部实现
func callOpenAIInternal(req *models.OpenAIRequest, cfg *config.Config) (*models.OpenAIResponse, error) {
	// 将请求序列化为 JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 API URL
	apiURL := cfg.OpenAIBaseURL + "/chat/completions"

	// 创建 HTTP 请求
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")

	// 跳过 Ollama（localhost）的认证 - Ollama 不需要认证
	if !cfg.IsLocalhost() {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	}

	// OpenRouter 特定的请求头以获得更好的速率限制
	if cfg.DetectProvider() == config.ProviderOpenRouter {
		addOpenRouterHeaders(httpReq, cfg)
	}

	// 创建带超时的 HTTP 客户端
	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查错误
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API 返回状态码 %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var openaiResp models.OpenAIResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &openaiResp, nil
}

func handleCountTokens(c *fiber.Ctx, cfg *config.Config) error {
	// 简单的令牌计数端点
	return c.JSON(fiber.Map{
		"input_tokens": 100,
	})
}

