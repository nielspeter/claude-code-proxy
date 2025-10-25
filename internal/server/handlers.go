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

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/internal/converter"
	"github.com/claude-code-proxy/proxy/pkg/models"
	"github.com/gofiber/fiber/v2"
)

func handleMessages(c *fiber.Ctx, cfg *config.Config) error {
	// Debug: Log raw request
	if cfg.Debug {
		fmt.Printf("\n=== CLAUDE REQUEST ===\n%s\n===================\n", string(c.Body()))
	}

	// Parse Claude request
	var claudeReq models.ClaudeRequest
	if err := c.BodyParser(&claudeReq); err != nil {
		// Log the error and raw body for debugging
		fmt.Printf("[ERROR] Failed to parse request body: %v\n", err)
		fmt.Printf("[ERROR] Raw body: %s\n", string(c.Body()))
		return c.Status(400).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "invalid_request_error",
				"message": fmt.Sprintf("Invalid request body: %v", err),
			},
		})
	}

	// Validate API key (if configured)
	if cfg.AnthropicAPIKey != "" {
		apiKey := c.Get("x-api-key")
		if apiKey != cfg.AnthropicAPIKey {
			return c.Status(401).JSON(fiber.Map{
				"type": "error",
				"error": fiber.Map{
					"type":    "authentication_error",
					"message": "Invalid API key",
				},
			})
		}
	}

	// Convert Claude request to OpenAI format
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

	// Debug: Log converted OpenAI request
	if cfg.Debug {
		openaiReqJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
		fmt.Printf("\n=== OPENAI REQUEST ===\n%s\n===================\n", string(openaiReqJSON))
		if len(claudeReq.Tools) > 0 {
			fmt.Printf("[DEBUG] Request has %d tools\n", len(claudeReq.Tools))
			for i, tool := range openaiReq.Tools {
				fmt.Printf("[DEBUG] Tool %d: %s\n", i, tool.Function.Name)
			}
		}
	}

	// Debug: Check Stream field
	if cfg.Debug {
		if openaiReq.Stream == nil {
			fmt.Printf("[DEBUG] Stream field is nil\n")
		} else {
			fmt.Printf("[DEBUG] Stream field = %v\n", *openaiReq.Stream)
		}
	}

	// Handle streaming vs non-streaming
	if openaiReq.Stream != nil && *openaiReq.Stream {
		return handleStreamingMessages(c, openaiReq, cfg)
	}

	// Track timing for simple log
	startTime := time.Now()

	// Non-streaming response
	openaiResp, err := callOpenAI(openaiReq, cfg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "api_error",
				"message": fmt.Sprintf("OpenAI API error: %v", err),
			},
		})
	}

	// Debug: Log OpenAI response
	if cfg.Debug {
		openaiRespJSON, _ := json.MarshalIndent(openaiResp, "", "  ")
		fmt.Printf("\n=== OPENAI RESPONSE ===\n%s\n====================\n", string(openaiRespJSON))
		if len(openaiResp.Choices) > 0 {
			choice := openaiResp.Choices[0]
			fmt.Printf("[DEBUG] OpenAI response has %d tool_calls\n", len(choice.Message.ToolCalls))
			for i, tc := range choice.Message.ToolCalls {
				fmt.Printf("[DEBUG] ToolCall %d: ID=%s, Name=%s\n", i, tc.ID, tc.Function.Name)
			}
		}
	}

	// Convert OpenAI response to Claude format
	claudeResp, err := converter.ConvertResponse(openaiResp, claudeReq.Model)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"type": "error",
			"error": fiber.Map{
				"type":    "api_error",
				"message": fmt.Sprintf("Response conversion error: %v", err),
			},
		})
	}

	// Debug: Log Claude response
	if cfg.Debug {
		claudeRespJSON, _ := json.MarshalIndent(claudeResp, "", "  ")
		fmt.Printf("\n=== CLAUDE RESPONSE ===\n%s\n====================\n\n", string(claudeRespJSON))
		fmt.Printf("[DEBUG] Claude response has %d content blocks\n", len(claudeResp.Content))
		for i, block := range claudeResp.Content {
			fmt.Printf("[DEBUG] Block %d: type=%s", i, block.Type)
			if block.Type == "tool_use" {
				fmt.Printf(", name=%s, id=%s", block.Name, block.ID)
			}
			fmt.Printf("\n")
		}
	}

	// Simple log: one-line summary
	if cfg.SimpleLog {
		duration := time.Since(startTime).Seconds()
		tokensPerSec := 0.0
		if duration > 0 && claudeResp.Usage.OutputTokens > 0 {
			tokensPerSec = float64(claudeResp.Usage.OutputTokens) / duration
		}
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] [REQ] %s model=%s in=%d out=%d tok/s=%.1f\n",
			timestamp,
			cfg.OpenAIBaseURL,
			openaiReq.Model,
			claudeResp.Usage.InputTokens,
			claudeResp.Usage.OutputTokens,
			tokensPerSec)
	}

	return c.JSON(claudeResp)
}

// handleStreamingMessages handles streaming requests
func handleStreamingMessages(c *fiber.Ctx, openaiReq *models.OpenAIRequest, cfg *config.Config) error {
	// Track timing for simple log
	startTime := time.Now()

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		if cfg.Debug {
			fmt.Printf("[DEBUG] StreamWriter: Starting\n")
		}

		// Marshal request
		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			if cfg.Debug {
				fmt.Printf("[DEBUG] StreamWriter: Failed to marshal: %v\n", err)
			}
			writeSSEError(w, fmt.Sprintf("failed to marshal request: %v", err))
			return
		}

		if cfg.Debug {
			fmt.Printf("[DEBUG] StreamWriter: Making request to %s\n", cfg.OpenAIBaseURL+"/chat/completions")
		}

		// Build API URL
		apiURL := cfg.OpenAIBaseURL + "/chat/completions"

		// Create HTTP request
		httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
		if err != nil {
			writeSSEError(w, fmt.Sprintf("failed to create request: %v", err))
			return
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")

		// Skip auth for Ollama (localhost) - Ollama doesn't require authentication
		if !cfg.IsLocalhost() {
			httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
		}

		// OpenRouter-specific headers for better rate limits
		if cfg.DetectProvider() == config.ProviderOpenRouter {
			if cfg.OpenRouterAppURL != "" {
				httpReq.Header.Set("HTTP-Referer", cfg.OpenRouterAppURL)
			}
			if cfg.OpenRouterAppName != "" {
				httpReq.Header.Set("X-Title", cfg.OpenRouterAppName)
			}
		}

		client := &http.Client{
			Timeout: 300 * time.Second, // Longer timeout for streaming
		}

		// Make request
		resp, err := client.Do(httpReq)
		if err != nil {
			if cfg.Debug {
				fmt.Printf("[DEBUG] StreamWriter: Request failed: %v\n", err)
			}
			writeSSEError(w, fmt.Sprintf("request failed: %v", err))
			return
		}
		defer resp.Body.Close()

		if cfg.Debug {
			fmt.Printf("[DEBUG] StreamWriter: Got response with status %d\n", resp.StatusCode)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if cfg.Debug {
				fmt.Printf("[DEBUG] StreamWriter: Bad status: %s\n", string(body))
			}
			writeSSEError(w, fmt.Sprintf("OpenAI API returned status %d: %s", resp.StatusCode, string(body)))
			return
		}

		if cfg.Debug {
			fmt.Printf("[DEBUG] StreamWriter: Starting streamOpenAIToClaude conversion\n")
		}

		// Stream conversion
		streamOpenAIToClaude(w, resp.Body, openaiReq.Model, cfg, startTime)

		if cfg.Debug {
			fmt.Printf("[DEBUG] StreamWriter: Completed\n")
		}
	})

	return nil
}

// ToolCallState tracks the state of a tool call during streaming (matches Python current_tool_calls)
type ToolCallState struct {
	ID          string // Tool call ID from OpenAI
	Name        string // Function name
	ArgsBuffer  string // Accumulated JSON arguments
	JSONSent    bool   // Flag if we sent the JSON delta
	ClaudeIndex int    // The content block index for Claude
	Started     bool   // Flag if content_block_start was sent
}

// streamOpenAIToClaude converts OpenAI SSE stream to Claude SSE format
// This implementation matches the Python version line-by-line
func streamOpenAIToClaude(w *bufio.Writer, reader io.Reader, providerModel string, cfg *config.Config, startTime time.Time) {
	if cfg.Debug {
		fmt.Printf("[DEBUG] streamOpenAIToClaude: Starting conversion\n")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Increase buffer size

	// State variables (matches Python implementation)
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	textBlockIndex := 1                              // Text block is index 1 (thinking is 0)
	toolBlockCounter := 2                            // Tool calls start at index 2
	currentToolCalls := make(map[int]*ToolCallState) // Python: current_tool_calls = {}
	finalStopReason := "end_turn"                    // Python: final_stop_reason = "end_turn"
	usageData := map[string]interface{}{             // Python: usage_data = {...}
		"input_tokens":                0,
		"output_tokens":               0,
		"cache_creation_input_tokens": 0,
		"cache_read_input_tokens":     0,
		"cache_creation": map[string]interface{}{
			"ephemeral_5m_input_tokens": 0,
			"ephemeral_1h_input_tokens": 0,
		},
	}

	// Thinking block tracking (to show thinking indicator in Claude Code)
	thinkingBlockIndex := 0 // Thinking block is always index 0
	thinkingBlockStarted := false
	thinkingBlockHasContent := false
	textBlockStarted := false // Track if we've sent text block_start

	// Send initial SSE events (matches Python lines 96-101)
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

	w.Flush()

	// Process streaming chunks (matches Python lines 111-210)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Check for [DONE] marker
		if strings.Contains(line, "[DONE]") {
			break
		}

		// Parse data line
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		dataJSON := strings.TrimPrefix(line, "data: ")

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataJSON), &chunk); err != nil {
			continue
		}

		// Log every chunk to see what OpenRouter is sending
		if cfg.Debug {
			fmt.Printf("[DEBUG] Raw chunk from OpenRouter: %s\n", dataJSON)
		}

		// Handle usage data (matches Python lines 120-131)
		if usage, ok := chunk["usage"].(map[string]interface{}); ok {
			if cfg.Debug {
				usageJSON, _ := json.Marshal(usage)
				fmt.Printf("[DEBUG] Received usage from OpenAI: %s\n", string(usageJSON))
			}

			// Convert float64 to int for token counts (JSON unmarshals numbers as float64)
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

			// Add cache metrics if present
			if promptTokensDetails, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
				if cachedTokens, ok := promptTokensDetails["cached_tokens"].(float64); ok && cachedTokens > 0 {
					usageData["cache_read_input_tokens"] = int(cachedTokens)
				}
			}
			if cfg.Debug {
				usageDataJSON, _ := json.Marshal(usageData)
				fmt.Printf("[DEBUG] Accumulated usageData: %s\n", string(usageDataJSON))
			}
		}

		// Extract delta from choices
		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}

		choice := choices[0].(map[string]interface{})
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		// Handle reasoning delta (thinking blocks)
		// Support both OpenRouter and OpenAI formats:
		// - OpenRouter: delta.reasoning_details (array)
		// - OpenAI o1/o3: delta.reasoning_content (string)

		// First, check for OpenAI's reasoning_content format (o1/o3 models)
		if reasoningContent, ok := delta["reasoning_content"].(string); ok && reasoningContent != "" {
			// Send content_block_start for thinking block on first thinking delta
			if !thinkingBlockStarted {
				writeSSEEvent(w, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": thinkingBlockIndex,
					"content_block": map[string]interface{}{
						"type": "thinking",
					},
				})
				thinkingBlockStarted = true
				thinkingBlockHasContent = true
			}

			// Send thinking delta
			writeSSEEvent(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": thinkingBlockIndex,
				"delta": map[string]interface{}{
					"type": "thinking_delta",
					"text": reasoningContent,
				},
			})
		}

		// Then, check for OpenRouter's reasoning_details format
		// Only process reasoning_details if we haven't already processed reasoning field
		if reasoningDetailsRaw, ok := delta["reasoning_details"]; ok && delta["reasoning"] == nil {
			if reasoningDetails, ok := reasoningDetailsRaw.([]interface{}); ok && len(reasoningDetails) > 0 {
				for _, detailRaw := range reasoningDetails {
					if detail, ok := detailRaw.(map[string]interface{}); ok {
						// Extract reasoning text from the detail
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
							// Skip encrypted/redacted reasoning in streaming
							continue
						}

						if thinkingText != "" {
							// Send content_block_start for thinking block on first thinking delta
							if !thinkingBlockStarted {
								writeSSEEvent(w, "content_block_start", map[string]interface{}{
									"type":  "content_block_start",
									"index": thinkingBlockIndex,
									"content_block": map[string]interface{}{
										"type":     "thinking",
										"thinking": "",
									},
								})
								thinkingBlockStarted = true
								w.Flush()
							}

							// Send thinking block delta
							writeSSEEvent(w, "content_block_delta", map[string]interface{}{
								"type":  "content_block_delta",
								"index": thinkingBlockIndex,
								"delta": map[string]interface{}{
									"type":     "thinking_delta",
									"thinking": thinkingText,
								},
							})
							thinkingBlockHasContent = true
							w.Flush()
						}
					}
				}
			}
		}

		// Handle reasoning field directly (simpler format from some models)
		if reasoning, ok := delta["reasoning"].(string); ok && reasoning != "" {
			// Send content_block_start for thinking block on first thinking delta
			if !thinkingBlockStarted {
				writeSSEEvent(w, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": thinkingBlockIndex,
					"content_block": map[string]interface{}{
						"type":     "thinking",
						"thinking": "",
					},
				})
				thinkingBlockStarted = true
				w.Flush()
			}

			// Send thinking block delta
			writeSSEEvent(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": thinkingBlockIndex,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": reasoning,
				},
			})
			thinkingBlockHasContent = true
			w.Flush()
		}

		// Handle text delta (matches Python lines 146-147)
		if content, ok := delta["content"].(string); ok && content != "" {
			// Send content_block_start for text block on first text delta
			if !textBlockStarted {
				writeSSEEvent(w, "content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": textBlockIndex,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				})
				textBlockStarted = true
				w.Flush()
			}

			writeSSEEvent(w, "content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": textBlockIndex,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": content,
				},
			})
			w.Flush()
		}

		// Handle tool call deltas (matches Python lines 149-198)
		if toolCallsRaw, ok := delta["tool_calls"]; ok {
			// Debug: Log raw tool_calls from provider
			if cfg.Debug {
				toolCallsJSON, _ := json.Marshal(toolCallsRaw)
				fmt.Printf("[DEBUG] Raw tool_calls delta: %s\n", string(toolCallsJSON))
			}

			toolCalls, ok := toolCallsRaw.([]interface{})
			if ok && len(toolCalls) > 0 {
				for _, tcRaw := range toolCalls {
					tcDelta, ok := tcRaw.(map[string]interface{})
					if !ok {
						continue
					}

					// Get tool call index (matches Python line 152)
					tcIndex := 0
					if idx, ok := tcDelta["index"].(float64); ok {
						tcIndex = int(idx)
					}

					// Initialize tool call tracking if not exists (matches Python lines 155-163)
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

					// Update tool call ID if provided (matches Python lines 168-169)
					if id, ok := tcDelta["id"].(string); ok {
						toolCall.ID = id
					}

					// Update function name (matches Python lines 172-174)
					if functionData, ok := tcDelta["function"].(map[string]interface{}); ok {
						if name, ok := functionData["name"].(string); ok {
							toolCall.Name = name
						}

						// Start content block when we have complete initial data (matches Python lines 177-183)
						if toolCall.ID != "" && toolCall.Name != "" && !toolCall.Started {
							toolBlockCounter++
							claudeIndex := textBlockIndex + toolBlockCounter
							toolCall.ClaudeIndex = claudeIndex
							toolCall.Started = true

							writeSSEEvent(w, "content_block_start", map[string]interface{}{
								"type":  "content_block_start",
								"index": claudeIndex,
								"content_block": map[string]interface{}{
									"type":  "tool_use",
									"id":    toolCall.ID,
									"name":  toolCall.Name,
									"input": map[string]interface{}{},
								},
							})
							w.Flush()
						}

						// Handle function arguments (matches Python lines 186-198)
						// Python checks: "arguments" in function_data and tool_call["started"] and function_data["arguments"] is not None
						// Go equivalent: type assertion handles nil check, Started flag, and we process even empty strings
						if args, ok := functionData["arguments"].(string); ok && toolCall.Started {
							// Only accumulate if args is not empty
							if args != "" {
								toolCall.ArgsBuffer += args
							}

							// Try to parse complete JSON and send delta when we have valid JSON (matches Python 190-195)
							if toolCall.ArgsBuffer != "" {
								var jsonTest interface{}
								if err := json.Unmarshal([]byte(toolCall.ArgsBuffer), &jsonTest); err == nil {
									// If parsing succeeds and we haven't sent this JSON yet
									if !toolCall.JSONSent {
										writeSSEEvent(w, "content_block_delta", map[string]interface{}{
											"type":  "content_block_delta",
											"index": toolCall.ClaudeIndex,
											"delta": map[string]interface{}{
												"type":         "input_json_delta",
												"partial_json": toolCall.ArgsBuffer,
											},
										})
										w.Flush()
										toolCall.JSONSent = true
									}
								}
							}
							// If JSON is incomplete, continue accumulating (no action needed)
						}
					}
				}
			}
		}

		// Handle finish reason (matches Python lines 200-210)
		// NOTE: Don't break here - with stream_options.include_usage, OpenAI sends usage in a chunk AFTER finish_reason
		if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
			if finishReason == "length" {
				finalStopReason = "max_tokens"
			} else if finishReason == "tool_calls" || finishReason == "function_call" {
				finalStopReason = "tool_use"
			} else if finishReason == "stop" {
				finalStopReason = "end_turn"
			} else {
				finalStopReason = "end_turn"
			}
			// Continue processing to capture usage chunk (don't break)
		}
	}

	// Send final SSE events (matches Python lines 225-234)

	// Send content_block_stop for text block if it was started (matches Python line 226)
	if textBlockStarted {
		writeSSEEvent(w, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": textBlockIndex,
		})
		w.Flush()
	}

	// Send content_block_stop for each tool call (matches Python lines 228-230)
	for _, toolData := range currentToolCalls {
		// Python checks both Started AND claude_index is not None (line 229)
		if toolData.Started && toolData.ClaudeIndex != 0 {
			writeSSEEvent(w, "content_block_stop", map[string]interface{}{
				"type":  "content_block_stop",
				"index": toolData.ClaudeIndex,
			})
			w.Flush()
		}
	}

	// Send content_block_stop for thinking block if it had content
	if thinkingBlockStarted && thinkingBlockHasContent {
		writeSSEEvent(w, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": thinkingBlockIndex,
		})
		w.Flush()
	}

	// Debug: Check if usage data was received
	if cfg.Debug {
		inputTokens, _ := usageData["input_tokens"].(int)
		outputTokens, _ := usageData["output_tokens"].(int)
		if inputTokens == 0 && outputTokens == 0 {
			fmt.Printf("[DEBUG] OpenRouter streaming: Usage data unavailable (expected limitation of streaming API)\n")
		}
	}

	// Send message_delta with stop_reason and accumulated usage data
	// NOTE: Unlike Python (which resets to zeros on line 232), we send the actual accumulated usage
	// This fixes the "0 tokens" issue in Claude Code
	if cfg.Debug {
		usageDataJSON, _ := json.Marshal(usageData)
		fmt.Printf("[DEBUG] Sending message_delta with usageData: %s\n", string(usageDataJSON))
	}
	writeSSEEvent(w, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   finalStopReason,
			"stop_sequence": nil, // Python includes this (line 233)
		},
		"usage": usageData,
	})
	w.Flush()

	// Send message_stop (matches Python line 234)
	writeSSEEvent(w, "message_stop", map[string]interface{}{
		"type": "message_stop",
	})
	w.Flush()

	// Simple log: one-line summary
	if cfg.SimpleLog {
		inputTokens := 0
		outputTokens := 0

		// Try to extract tokens from various possible formats
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

		// Debug: show what we actually have in usageData
		if cfg.Debug {
			fmt.Printf("[DEBUG] usageData: %+v\n", usageData)
		}

		// Calculate tokens per second
		duration := time.Since(startTime).Seconds()
		tokensPerSec := 0.0
		if duration > 0 && outputTokens > 0 {
			tokensPerSec = float64(outputTokens) / duration
		}

		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] [REQ] %s model=%s in=%d out=%d tok/s=%.1f\n",
			timestamp,
			cfg.OpenAIBaseURL,
			providerModel,
			inputTokens,
			outputTokens,
			tokensPerSec)
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		writeSSEError(w, fmt.Sprintf("stream read error: %v", err))
	}
}

// writeSSEEvent writes a Server-Sent Event
func writeSSEEvent(w *bufio.Writer, event string, data interface{}) {
	dataJSON, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", string(dataJSON))
}

// writeSSEError writes an error event
func writeSSEError(w *bufio.Writer, message string) {
	writeSSEEvent(w, "error", map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": message,
		},
	})
	w.Flush()
}

// convertFinishReasonStreaming converts OpenAI finish reason to Claude format (streaming)
func convertFinishReasonStreaming(openaiReason string) string {
	switch openaiReason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// callOpenAI makes an HTTP request to the OpenAI API
func callOpenAI(req *models.OpenAIRequest, cfg *config.Config) (*models.OpenAIResponse, error) {
	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL
	apiURL := cfg.OpenAIBaseURL + "/chat/completions"

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Skip auth for Ollama (localhost) - Ollama doesn't require authentication
	if !cfg.IsLocalhost() {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	}

	// OpenRouter-specific headers for better rate limits
	if cfg.DetectProvider() == config.ProviderOpenRouter {
		if cfg.OpenRouterAppURL != "" {
			httpReq.Header.Set("HTTP-Referer", cfg.OpenRouterAppURL)
		}
		if cfg.OpenRouterAppName != "" {
			httpReq.Header.Set("X-Title", cfg.OpenRouterAppName)
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	// Make request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var openaiResp models.OpenAIResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &openaiResp, nil
}

func handleCountTokens(c *fiber.Ctx, cfg *config.Config) error {
	// Simple token counting endpoint
	return c.JSON(fiber.Map{
		"input_tokens": 100,
	})
}
