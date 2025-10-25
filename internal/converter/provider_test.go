package converter

import (
	"encoding/json"
	"testing"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/pkg/models"
)

// TestProviderSpecificRequestConversion tests that requests are converted correctly per provider
// TODO: Implement provider-specific features (reasoning, tool_choice) in v1.1.0
func TestProviderSpecificRequestConversion(t *testing.T) {
	t.Skip("Provider-specific features (reasoning, tool_choice) not yet implemented - planned for v1.1.0")

	t.Run("OpenRouter with reasoning model", func(t *testing.T) {
		cfg := &config.Config{
			SonnetModel: "anthropic/claude-sonnet-4-5",
		}
		cfg.OpenAIBaseURL = "https://openrouter.ai/api/v1"

		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-5-20250805",
			MaxTokens: 1000,
			Messages: []models.ClaudeMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		// Verify model is mapped correctly
		if openaiReq.Model != "anthropic/claude-sonnet-4-5" {
			t.Errorf("Model = %q, want %q", openaiReq.Model, "anthropic/claude-sonnet-4-5")
		}

		// Verify OpenRouter reasoning format
		if openaiReq.Reasoning == nil {
			t.Error("OpenRouter request should have Reasoning object")
		} else if enabled, ok := openaiReq.Reasoning["enabled"].(bool); !ok || !enabled {
			t.Error("Reasoning.enabled should be true")
		}

		// Should NOT have ReasoningEffort (that's for OpenAI Direct)
		if openaiReq.ReasoningEffort != "" {
			t.Error("OpenRouter request should NOT have ReasoningEffort")
		}
	})

	t.Run("OpenAI Direct with reasoning model", func(t *testing.T) {
		cfg := &config.Config{
			SonnetModel: "gpt-5",
		}
		cfg.OpenAIBaseURL = "https://api.openai.com/v1"

		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-5-20250805",
			MaxTokens: 1000,
			Messages: []models.ClaudeMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		// Verify model is mapped correctly
		if openaiReq.Model != "gpt-5" {
			t.Errorf("Model = %q, want %q", openaiReq.Model, "gpt-5")
		}

		// Verify OpenAI Direct reasoning format
		if openaiReq.ReasoningEffort == "" {
			t.Error("OpenAI Direct request should have ReasoningEffort")
		}

		// Should NOT have Reasoning object (that's for OpenRouter)
		if openaiReq.Reasoning != nil {
			t.Error("OpenAI Direct request should NOT have Reasoning object")
		}
	})

	t.Run("Ollama with tool_choice when tools present", func(t *testing.T) {
		cfg := &config.Config{
			SonnetModel: "qwen2.5:14b",
		}
		cfg.OpenAIBaseURL = "http://localhost:11434/v1"

		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-5-20250805",
			MaxTokens: 1000,
			Messages: []models.ClaudeMessage{
				{Role: "user", Content: "Hello"},
			},
			Tools: []models.Tool{
				{
					Name:        "test_tool",
					Description: "A test tool",
					InputSchema: map[string]interface{}{
						"type": "object",
					},
				},
			},
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		// Verify model is mapped correctly
		if openaiReq.Model != "qwen2.5:14b" {
			t.Errorf("Model = %q, want %q", openaiReq.Model, "qwen2.5:14b")
		}

		// Verify tool_choice is set to force tool usage
		if openaiReq.ToolChoice != "required" {
			t.Errorf("ToolChoice = %v, want %q", openaiReq.ToolChoice, "required")
		}

		// Should NOT have reasoning parameters
		if openaiReq.Reasoning != nil {
			t.Error("Ollama request should NOT have Reasoning object")
		}
		if openaiReq.ReasoningEffort != "" {
			t.Error("Ollama request should NOT have ReasoningEffort")
		}
	})

	t.Run("Ollama without tool_choice when no tools", func(t *testing.T) {
		cfg := &config.Config{
			SonnetModel: "qwen2.5:14b",
		}
		cfg.OpenAIBaseURL = "http://localhost:11434/v1"

		claudeReq := models.ClaudeRequest{
			Model:     "claude-sonnet-4-5-20250805",
			MaxTokens: 1000,
			Messages: []models.ClaudeMessage{
				{Role: "user", Content: "Hello"},
			},
			Tools: []models.Tool{}, // No tools
		}

		openaiReq, err := ConvertRequest(claudeReq, cfg)
		if err != nil {
			t.Fatalf("ConvertRequest() error = %v", err)
		}

		// Should NOT have tool_choice when no tools
		if openaiReq.ToolChoice != nil {
			t.Errorf("ToolChoice should be nil when no tools present, got %v", openaiReq.ToolChoice)
		}
	})
}

// TestModelMappingVerification tests that we're using the correct model for each provider
func TestModelMappingVerification(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		envModel    string
		claudeModel string
		wantModel   string
	}{
		{
			name:        "OpenRouter Grok mapping",
			baseURL:     "https://openrouter.ai/api/v1",
			envModel:    "x-ai/grok-code-fast-1",
			claudeModel: "claude-sonnet-4-5-20250805",
			wantModel:   "x-ai/grok-code-fast-1",
		},
		{
			name:        "OpenRouter Gemini mapping",
			baseURL:     "https://openrouter.ai/api/v1",
			envModel:    "google/gemini-2.5-flash",
			claudeModel: "claude-haiku-4-5-20251001",
			wantModel:   "google/gemini-2.5-flash",
		},
		{
			name:        "OpenAI Direct GPT-5 mapping",
			baseURL:     "https://api.openai.com/v1",
			envModel:    "gpt-5",
			claudeModel: "claude-sonnet-4-5-20250805",
			wantModel:   "gpt-5",
		},
		{
			name:        "Ollama qwen2.5 mapping",
			baseURL:     "http://localhost:11434/v1",
			envModel:    "qwen2.5:14b",
			claudeModel: "claude-sonnet-4-5-20250805",
			wantModel:   "qwen2.5:14b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SonnetModel: tt.envModel,
				HaikuModel:  tt.envModel,
			}
			cfg.OpenAIBaseURL = tt.baseURL

			claudeReq := models.ClaudeRequest{
				Model:     tt.claudeModel,
				MaxTokens: 100,
				Messages: []models.ClaudeMessage{
					{Role: "user", Content: "test"},
				},
			}

			openaiReq, err := ConvertRequest(claudeReq, cfg)
			if err != nil {
				t.Fatalf("ConvertRequest() error = %v", err)
			}

			if openaiReq.Model != tt.wantModel {
				t.Errorf("Got model %q, want %q for provider %s",
					openaiReq.Model, tt.wantModel, tt.baseURL)
			}
		})
	}
}

// TestResponseConversionWithReasoning tests that reasoning responses are converted correctly
func TestResponseConversionWithReasoning(t *testing.T) {
	t.Run("OpenRouter/Grok reasoning format", func(t *testing.T) {
		finishReason := "stop"
		// Simulate Grok's reasoning_details format
		openaiResp := &models.OpenAIResponse{
			ID:      "gen-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "x-ai/grok-code-fast-1",
			Choices: []models.OpenAIChoice{
				{
					Index: 0,
					Message: models.OpenAIMessage{
						Role:    "assistant",
						Content: "The answer is 42",
						ReasoningDetails: []interface{}{
							map[string]interface{}{
								"type":    "reasoning.summary",
								"summary": "First I analyzed the question...",
							},
						},
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

		claudeResp, err := ConvertResponse(openaiResp, "claude-sonnet-4-5-20250805")
		if err != nil {
			t.Fatalf("ConvertResponse() error = %v", err)
		}

		// Should have thinking block
		foundThinking := false
		for _, block := range claudeResp.Content {
			if block.Type == "thinking" {
				foundThinking = true
				if block.Thinking == "" {
					t.Error("Thinking block should have content")
				}
			}
		}

		if !foundThinking {
			t.Error("Response should contain thinking block from reasoning_details")
		}
	})
}

// TestProviderIsolationInPractice ensures we don't accidentally mix providers
func TestProviderIsolationInPractice(t *testing.T) {
	tests := []struct {
		name              string
		configuredBaseURL string
		model             string
		shouldNotContain  []string // Strings that shouldn't appear in the request
	}{
		{
			name:              "OpenRouter config shouldn't use OpenAI models",
			configuredBaseURL: "https://openrouter.ai/api/v1",
			model:             "x-ai/grok-code-fast-1",
			shouldNotContain:  []string{"gpt-5", "gpt-4o"},
		},
		{
			name:              "OpenAI Direct config shouldn't use OpenRouter models",
			configuredBaseURL: "https://api.openai.com/v1",
			model:             "gpt-5",
			shouldNotContain:  []string{"x-ai/", "google/", "anthropic/"},
		},
		{
			name:              "Ollama config shouldn't use cloud models",
			configuredBaseURL: "http://localhost:11434/v1",
			model:             "qwen2.5:14b",
			shouldNotContain:  []string{"gpt-", "x-ai/", "google/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SonnetModel: tt.model,
			}
			cfg.OpenAIBaseURL = tt.configuredBaseURL

			claudeReq := models.ClaudeRequest{
				Model:     "claude-sonnet-4-5-20250805",
				MaxTokens: 100,
				Messages: []models.ClaudeMessage{
					{Role: "user", Content: "test"},
				},
			}

			openaiReq, err := ConvertRequest(claudeReq, cfg)
			if err != nil {
				t.Fatalf("ConvertRequest() error = %v", err)
			}

			// Serialize the request to check its content
			reqJSON, _ := json.Marshal(openaiReq)
			reqStr := string(reqJSON)

			// Verify none of the forbidden strings appear
			for _, forbidden := range tt.shouldNotContain {
				if contains(reqStr, forbidden) {
					t.Errorf("Request for %s should not contain %q but found it in: %s",
						tt.configuredBaseURL, forbidden, reqStr)
				}
			}

			// Verify the correct model is being used
			if openaiReq.Model != tt.model {
				t.Errorf("Expected model %q, got %q", tt.model, openaiReq.Model)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || hasSubstr(s, substr)))
}

func hasSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
