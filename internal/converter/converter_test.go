package converter

import (
	"testing"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/pkg/models"
)

// TestExtractSystemText tests system message extraction from various formats
func TestExtractSystemText(t *testing.T) {
	tests := []struct {
		name     string
		system   interface{}
		expected string
	}{
		{
			name:     "nil system",
			system:   nil,
			expected: "",
		},
		{
			name:     "string system",
			system:   "You are a helpful assistant",
			expected: "You are a helpful assistant",
		},
		{
			name: "array system with single block",
			system: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "You are Claude Code",
				},
			},
			expected: "You are Claude Code",
		},
		{
			name: "array system with multiple blocks",
			system: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "You are Claude Code",
				},
				map[string]interface{}{
					"type": "text",
					"text": "Be helpful and concise",
				},
			},
			expected: "You are Claude Code\nBe helpful and concise",
		},
		{
			name: "array system with non-text blocks (should skip)",
			system: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "First part",
				},
				map[string]interface{}{
					"type": "image",
					"data": "base64...",
				},
				map[string]interface{}{
					"type": "text",
					"text": "Second part",
				},
			},
			expected: "First part\nSecond part",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSystemText(tt.system)
			if result != tt.expected {
				t.Errorf("extractSystemText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestMapModel tests model routing logic
func TestMapModel(t *testing.T) {
	cfg := &config.Config{
		OpusModel:   "",
		SonnetModel: "",
		HaikuModel:  "",
	}

	tests := []struct {
		name        string
		claudeModel string
		expected    string
	}{
		{
			name:        "haiku model",
			claudeModel: "claude-haiku-3-5-20241022",
			expected:    "gpt-5-mini",
		},
		{
			name:        "sonnet-4 model",
			claudeModel: "claude-sonnet-4-20250514",
			expected:    "gpt-5",
		},
		{
			name:        "sonnet-5 model",
			claudeModel: "claude-sonnet-5-20250101",
			expected:    "gpt-5",
		},
		{
			name:        "sonnet-3 model",
			claudeModel: "claude-3-5-sonnet-20241022",
			expected:    "gpt-5", // All sonnets now map to gpt-5
		},
		{
			name:        "opus model",
			claudeModel: "claude-opus-4-20250514",
			expected:    "gpt-5",
		},
		{
			name:        "non-claude model (passthrough)",
			claudeModel: "gpt-4o",
			expected:    "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapModel(tt.claudeModel, cfg)
			if result != tt.expected {
				t.Errorf("mapModel(%q) = %q, want %q", tt.claudeModel, result, tt.expected)
			}
		})
	}
}

// TestMapModelWithOverrides tests model routing with env overrides
func TestMapModelWithOverrides(t *testing.T) {
	cfg := &config.Config{
		OpusModel:   "custom-opus-model",
		SonnetModel: "custom-sonnet-model",
		HaikuModel:  "custom-haiku-model",
	}

	tests := []struct {
		name        string
		claudeModel string
		expected    string
	}{
		{
			name:        "haiku with override",
			claudeModel: "claude-haiku-3-5-20241022",
			expected:    "custom-haiku-model",
		},
		{
			name:        "sonnet with override",
			claudeModel: "claude-sonnet-4-20250514",
			expected:    "custom-sonnet-model",
		},
		{
			name:        "opus with override",
			claudeModel: "claude-opus-4-20250514",
			expected:    "custom-opus-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapModel(tt.claudeModel, cfg)
			if result != tt.expected {
				t.Errorf("mapModel(%q) = %q, want %q", tt.claudeModel, result, tt.expected)
			}
		})
	}
}

// TestConvertRequest tests full request conversion
func TestConvertRequest(t *testing.T) {
	cfg := &config.Config{
		OpusModel:   "",
		SonnetModel: "",
		HaikuModel:  "",
	}

	t.Run("simple request with string system", func(t *testing.T) {
		temp := 0.7
		stream := false
		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1000,
			System:    "You are helpful",
			Messages: []models.ClaudeMessage{
				{
					Role:    "user",
					Content: "Hello",
				},
			},
			Temperature: &temp,
			Stream:      &stream,
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		if openaiReq.Model != "gpt-5" {
			t.Errorf("Model = %q, want %q", openaiReq.Model, "gpt-5")
		}

		if len(openaiReq.Messages) != 2 {
			t.Errorf("Messages length = %d, want 2", len(openaiReq.Messages))
		}

		if openaiReq.Messages[0].Role != "system" {
			t.Errorf("First message role = %q, want %q", openaiReq.Messages[0].Role, "system")
		}

		if openaiReq.MaxCompletionTokens != 1000 {
			t.Errorf("MaxCompletionTokens = %d, want 1000", openaiReq.MaxCompletionTokens)
		}

		if *openaiReq.Temperature != temp {
			t.Errorf("Temperature = %f, want %f", *openaiReq.Temperature, temp)
		}
	})

	t.Run("request with array system", func(t *testing.T) {
		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1000,
			System: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Part 1",
				},
				map[string]interface{}{
					"type": "text",
					"text": "Part 2",
				},
			},
			Messages: []models.ClaudeMessage{
				{
					Role:    "user",
					Content: "Hello",
				},
			},
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		systemContent, ok := openaiReq.Messages[0].Content.(string)
		if !ok {
			t.Fatal("System message content is not a string")
		}

		expected := "Part 1\nPart 2"
		if systemContent != expected {
			t.Errorf("System content = %q, want %q", systemContent, expected)
		}
	})

	t.Run("request with tools", func(t *testing.T) {
		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1000,
			Messages: []models.ClaudeMessage{
				{
					Role:    "user",
					Content: "Hello",
				},
			},
			Tools: []models.Tool{
				{
					Name:        "get_weather",
					Description: "Get weather information",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		if len(openaiReq.Tools) != 1 {
			t.Fatalf("Tools length = %d, want 1", len(openaiReq.Tools))
		}

		if openaiReq.Tools[0].Type != "function" {
			t.Errorf("Tool type = %q, want %q", openaiReq.Tools[0].Type, "function")
		}

		if openaiReq.Tools[0].Function.Name != "get_weather" {
			t.Errorf("Tool name = %q, want %q", openaiReq.Tools[0].Function.Name, "get_weather")
		}
	})
}

// TestConvertResponse tests OpenAI → Claude response conversion
func TestConvertResponse(t *testing.T) {
	t.Run("simple text response", func(t *testing.T) {
		finishReason := "stop"
		openaiResp := &models.OpenAIResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-5",
			Choices: []models.OpenAIChoice{
				{
					Index: 0,
					Message: models.OpenAIMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: &finishReason,
				},
			},
			Usage: models.OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		claudeResp, err := ConvertResponse(openaiResp, "claude-sonnet-4-20250514")
		if err != nil {
			t.Fatalf("ConvertResponse() error = %v", err)
		}

		if claudeResp.ID != "chatcmpl-123" {
			t.Errorf("ID = %q, want %q", claudeResp.ID, "chatcmpl-123")
		}

		if claudeResp.Model != "claude-sonnet-4-20250514" {
			t.Errorf("Model = %q, want %q", claudeResp.Model, "claude-sonnet-4-20250514")
		}

		if len(claudeResp.Content) != 1 {
			t.Fatalf("Content length = %d, want 1", len(claudeResp.Content))
		}

		if claudeResp.Content[0].Type != "text" {
			t.Errorf("Content type = %q, want %q", claudeResp.Content[0].Type, "text")
		}

		if claudeResp.Content[0].Text != "Hello! How can I help you?" {
			t.Errorf("Content text = %q, want %q", claudeResp.Content[0].Text, "Hello! How can I help you?")
		}

		if *claudeResp.StopReason != "end_turn" {
			t.Errorf("StopReason = %q, want %q", *claudeResp.StopReason, "end_turn")
		}

		if claudeResp.Usage.InputTokens != 10 {
			t.Errorf("InputTokens = %d, want 10", claudeResp.Usage.InputTokens)
		}

		if claudeResp.Usage.OutputTokens != 20 {
			t.Errorf("OutputTokens = %d, want 20", claudeResp.Usage.OutputTokens)
		}
	})

	t.Run("response with tool calls", func(t *testing.T) {
		finishReason := "tool_calls"
		openaiResp := &models.OpenAIResponse{
			ID:      "chatcmpl-456",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-5",
			Choices: []models.OpenAIChoice{
				{
					Index: 0,
					Message: models.OpenAIMessage{
						Role:    "assistant",
						Content: "",
						ToolCalls: []models.OpenAIToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco"}`,
								},
							},
						},
					},
					FinishReason: &finishReason,
				},
			},
			Usage: models.OpenAIUsage{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
			},
		}

		claudeResp, err := ConvertResponse(openaiResp, "claude-sonnet-4-20250514")
		if err != nil {
			t.Fatalf("ConvertResponse() error = %v", err)
		}

		if len(claudeResp.Content) != 1 {
			t.Fatalf("Content length = %d, want 1", len(claudeResp.Content))
		}

		if claudeResp.Content[0].Type != "tool_use" {
			t.Errorf("Content type = %q, want %q", claudeResp.Content[0].Type, "tool_use")
		}

		if claudeResp.Content[0].ID != "call_123" {
			t.Errorf("Tool call ID = %q, want %q", claudeResp.Content[0].ID, "call_123")
		}

		if claudeResp.Content[0].Name != "get_weather" {
			t.Errorf("Tool name = %q, want %q", claudeResp.Content[0].Name, "get_weather")
		}

		if *claudeResp.StopReason != "tool_use" {
			t.Errorf("StopReason = %q, want %q", *claudeResp.StopReason, "tool_use")
		}
	})
}

// TestConvertFinishReason tests finish reason mapping
func TestConvertFinishReason(t *testing.T) {
	tests := []struct {
		openaiReason string
		claudeReason string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"content_filter", "end_turn"},
		{"unknown", "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.openaiReason, func(t *testing.T) {
			// Create a mock response to test the conversion
			openaiResp := &models.OpenAIResponse{
				ID: "test",
				Choices: []models.OpenAIChoice{
					{
						Index: 0,
						Message: models.OpenAIMessage{
							Role:    "assistant",
							Content: "test",
						},
						FinishReason: &tt.openaiReason,
					},
				},
				Usage: models.OpenAIUsage{},
			}

			claudeResp, err := ConvertResponse(openaiResp, "test-model")
			if err != nil {
				t.Fatalf("ConvertResponse() error = %v", err)
			}

			if *claudeResp.StopReason != tt.claudeReason {
				t.Errorf("finish reason %q mapped to %q, want %q",
					tt.openaiReason, *claudeResp.StopReason, tt.claudeReason)
			}
		})
	}
}

// TestConvertMessagesWithComplexContent tests message conversion with arrays
func TestConvertMessagesWithComplexContent(t *testing.T) {
	t.Run("message with array content", func(t *testing.T) {
		messages := []models.ClaudeMessage{
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Hello",
					},
					map[string]interface{}{
						"type": "text",
						"text": "How are you?",
					},
				},
			},
		}

		result := convertMessages(messages, "")

		if len(result) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(result))
		}

		contentStr, ok := result[0].Content.(string)
		if !ok {
			t.Fatal("Content is not a string")
		}

		expected := "Hello\nHow are you?"
		if contentStr != expected {
			t.Errorf("Content = %q, want %q", contentStr, expected)
		}
	})

	t.Run("message with tool_result", func(t *testing.T) {
		messages := []models.ClaudeMessage{
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "call_123",
						"content":     "Weather is sunny",
					},
				},
			},
		}

		result := convertMessages(messages, "")

		if len(result) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(result))
		}

		if result[0].Role != "tool" {
			t.Errorf("Role = %q, want %q", result[0].Role, "tool")
		}

		if result[0].ToolCallID != "call_123" {
			t.Errorf("ToolCallID = %q, want %q", result[0].ToolCallID, "call_123")
		}

		contentStr, ok := result[0].Content.(string)
		if !ok {
			t.Fatal("Content is not a string")
		}

		if contentStr != "Weather is sunny" {
			t.Errorf("Content = %q, want %q", contentStr, "Weather is sunny")
		}
	})

	t.Run("message with tool_use blocks", func(t *testing.T) {
		messages := []models.ClaudeMessage{
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type": "tool_use",
						"id":   "call_456",
						"name": "get_weather",
						"input": map[string]interface{}{
							"location": "San Francisco",
						},
					},
				},
			},
		}

		result := convertMessages(messages, "")

		if len(result) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(result))
		}

		if result[0].Role != "assistant" {
			t.Errorf("Role = %q, want %q", result[0].Role, "assistant")
		}

		if len(result[0].ToolCalls) != 1 {
			t.Fatalf("Expected 1 tool call, got %d", len(result[0].ToolCalls))
		}

		toolCall := result[0].ToolCalls[0]
		if toolCall.ID != "call_456" {
			t.Errorf("ToolCall ID = %q, want %q", toolCall.ID, "call_456")
		}

		if toolCall.Type != "function" {
			t.Errorf("ToolCall Type = %q, want %q", toolCall.Type, "function")
		}

		if toolCall.Function.Name != "get_weather" {
			t.Errorf("Function Name = %q, want %q", toolCall.Function.Name, "get_weather")
		}

		if toolCall.Function.Arguments != `{"location":"San Francisco"}` {
			t.Errorf("Function Arguments = %q, want %q", toolCall.Function.Arguments, `{"location":"San Francisco"}`)
		}
	})

	t.Run("message with mixed text and tool_use blocks", func(t *testing.T) {
		messages := []models.ClaudeMessage{
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Let me check the weather for you.",
					},
					map[string]interface{}{
						"type": "tool_use",
						"id":   "call_789",
						"name": "get_weather",
						"input": map[string]interface{}{
							"location": "New York",
						},
					},
				},
			},
		}

		result := convertMessages(messages, "")

		if len(result) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(result))
		}

		contentStr, ok := result[0].Content.(string)
		if !ok {
			t.Fatal("Content is not a string")
		}

		if contentStr != "Let me check the weather for you." {
			t.Errorf("Content = %q, want %q", contentStr, "Let me check the weather for you.")
		}

		if len(result[0].ToolCalls) != 1 {
			t.Fatalf("Expected 1 tool call, got %d", len(result[0].ToolCalls))
		}

		if result[0].ToolCalls[0].ID != "call_789" {
			t.Errorf("ToolCall ID = %q, want %q", result[0].ToolCalls[0].ID, "call_789")
		}
	})

	t.Run("complete tool call cycle", func(t *testing.T) {
		// Simulates: assistant calls tool → user provides result
		messages := []models.ClaudeMessage{
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "I'll get the weather for you.",
					},
					map[string]interface{}{
						"type": "tool_use",
						"id":   "call_abc",
						"name": "get_weather",
						"input": map[string]interface{}{
							"location": "London",
						},
					},
				},
			},
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "call_abc",
						"content":     "Temperature: 15°C, Cloudy",
					},
				},
			},
		}

		result := convertMessages(messages, "")

		if len(result) != 2 {
			t.Fatalf("Expected 2 messages, got %d", len(result))
		}

		// First message: assistant with tool call
		if result[0].Role != "assistant" {
			t.Errorf("First message role = %q, want %q", result[0].Role, "assistant")
		}

		if len(result[0].ToolCalls) != 1 {
			t.Fatalf("Expected 1 tool call, got %d", len(result[0].ToolCalls))
		}

		if result[0].ToolCalls[0].ID != "call_abc" {
			t.Errorf("ToolCall ID = %q, want %q", result[0].ToolCalls[0].ID, "call_abc")
		}

		// Second message: tool result
		if result[1].Role != "tool" {
			t.Errorf("Second message role = %q, want %q", result[1].Role, "tool")
		}

		if result[1].ToolCallID != "call_abc" {
			t.Errorf("ToolCallID = %q, want %q", result[1].ToolCallID, "call_abc")
		}

		contentStr, ok := result[1].Content.(string)
		if !ok {
			t.Fatal("Tool result content is not a string")
		}

		if contentStr != "Temperature: 15°C, Cloudy" {
			t.Errorf("Tool result content = %q, want %q", contentStr, "Temperature: 15°C, Cloudy")
		}
	})

	t.Run("multiple tool_use blocks in one message", func(t *testing.T) {
		messages := []models.ClaudeMessage{
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type": "tool_use",
						"id":   "call_1",
						"name": "get_weather",
						"input": map[string]interface{}{
							"location": "Tokyo",
						},
					},
					map[string]interface{}{
						"type": "tool_use",
						"id":   "call_2",
						"name": "get_time",
						"input": map[string]interface{}{
							"timezone": "Asia/Tokyo",
						},
					},
				},
			},
		}

		result := convertMessages(messages, "")

		if len(result) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(result))
		}

		if len(result[0].ToolCalls) != 2 {
			t.Fatalf("Expected 2 tool calls, got %d", len(result[0].ToolCalls))
		}

		if result[0].ToolCalls[0].Function.Name != "get_weather" {
			t.Errorf("First tool name = %q, want %q", result[0].ToolCalls[0].Function.Name, "get_weather")
		}

		if result[0].ToolCalls[1].Function.Name != "get_time" {
			t.Errorf("Second tool name = %q, want %q", result[0].ToolCalls[1].Function.Name, "get_time")
		}
	})
}

// Benchmark tests
func BenchmarkExtractSystemText(b *testing.B) {
	system := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "You are Claude Code",
		},
		map[string]interface{}{
			"type": "text",
			"text": "Be helpful and concise",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractSystemText(system)
	}
}

func BenchmarkConvertRequest(b *testing.B) {
	cfg := &config.Config{}
	claudeReq := models.ClaudeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1000,
		System:    "You are helpful",
		Messages: []models.ClaudeMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertRequest(claudeReq, cfg)
	}
}

func BenchmarkConvertResponse(b *testing.B) {
	finishReason := "stop"
	openaiResp := &models.OpenAIResponse{
		ID: "test",
		Choices: []models.OpenAIChoice{
			{
				Index: 0,
				Message: models.OpenAIMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you?",
				},
				FinishReason: &finishReason,
			},
		},
		Usage: models.OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ConvertResponse(openaiResp, "claude-sonnet-4-20250514")
	}
}
