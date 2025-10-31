package converter

import (
	"testing"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/pkg/models"
)

func TestIsReasoningModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		// GPT-5 series (reasoning models)
		{"gpt-5", "gpt-5", true},
		{"gpt-5 uppercase", "GPT-5", true},
		{"gpt-5-mini", "gpt-5-mini", true},
		{"gpt-5-turbo", "gpt-5-turbo", true},
		{"azure/gpt-5", "azure/gpt-5", true},
		{"openai/gpt-5", "openai/gpt-5", true},
		{"azure/gpt-5-mini", "azure/gpt-5-mini", true},

		// o-series reasoning models
		{"o1", "o1", true},
		{"o1-preview", "o1-preview", true},
		{"o1-mini", "o1-mini", true},
		{"o2", "o2", true},
		{"o2-preview", "o2-preview", true},
		{"o2-mini", "o2-mini", true},
		{"o3", "o3", true},
		{"o3-mini", "o3-mini", true},
		{"o4", "o4", true},
		{"o4-turbo", "o4-turbo", true},
		{"azure/o1", "azure/o1", true},
		{"azure/o2", "azure/o2", true},
		{"openai/o3", "openai/o3", true},

		// GPT-4 series (NOT reasoning models)
		{"gpt-4", "gpt-4", false},
		{"gpt-4o", "gpt-4o", false},
		{"gpt-4-turbo", "gpt-4-turbo", false},
		{"gpt-4.1", "gpt-4.1", false},
		{"gpt-4o-mini", "gpt-4o-mini", false},
		{"azure/gpt-4o", "azure/gpt-4o", false},
		{"openai/gpt-4-turbo", "openai/gpt-4-turbo", false},

		// GPT-3.5 series (NOT reasoning models)
		{"gpt-3.5-turbo", "gpt-3.5-turbo", false},
		{"gpt-3.5-turbo-16k", "gpt-3.5-turbo-16k", false},

		// Other models (NOT reasoning models)
		{"claude-3-opus", "claude-3-opus", false},
		{"claude-sonnet-4", "claude-sonnet-4", false},
		{"gemini-pro", "gemini-pro", false},
		{"llama-3-70b", "llama-3-70b", false},

		// Edge cases
		{"empty string", "", false},
		{"o prefix but not reasoning", "ollama", false},
		{"contains gpt-5 but not start", "meta-gpt-5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReasoningModel(tt.model)
			if result != tt.expected {
				t.Errorf("isReasoningModel(%q) = %v, expected %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestReasoningModelTokenParameter(t *testing.T) {
	tests := []struct {
		name                string
		model               string
		maxTokens           int
		expectMaxTokens     int
		expectMaxCompletion int
	}{
		{
			name:                "gpt-5 uses max_completion_tokens",
			model:               "gpt-5",
			maxTokens:           100,
			expectMaxTokens:     0,
			expectMaxCompletion: 100,
		},
		{
			name:                "o1 uses max_completion_tokens",
			model:               "o1",
			maxTokens:           200,
			expectMaxTokens:     0,
			expectMaxCompletion: 200,
		},
		{
			name:                "o2 uses max_completion_tokens",
			model:               "o2",
			maxTokens:           150,
			expectMaxTokens:     0,
			expectMaxCompletion: 150,
		},
		{
			name:                "azure/o3 uses max_completion_tokens",
			model:               "azure/o3",
			maxTokens:           150,
			expectMaxTokens:     0,
			expectMaxCompletion: 150,
		},
		{
			name:                "gpt-4o uses max_tokens",
			model:               "gpt-4o",
			maxTokens:           100,
			expectMaxTokens:     100,
			expectMaxCompletion: 0,
		},
		{
			name:                "gpt-4-turbo uses max_tokens",
			model:               "gpt-4-turbo",
			maxTokens:           200,
			expectMaxTokens:     200,
			expectMaxCompletion: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal Claude request
			claudeReq := models.ClaudeRequest{
				Model:     tt.model,
				MaxTokens: tt.maxTokens,
				Messages: []models.ClaudeMessage{
					{Role: "user", Content: "test"},
				},
			}

			// Create a minimal config
			cfg := &config.Config{
				OpenAIAPIKey:  "test-key",
				OpenAIBaseURL: "https://api.openai.com/v1",
			}

			// Convert the request
			openaiReq, err := ConvertRequest(claudeReq, cfg)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			// Verify token parameters
			if openaiReq.MaxTokens != tt.expectMaxTokens {
				t.Errorf("MaxTokens = %d, expected %d", openaiReq.MaxTokens, tt.expectMaxTokens)
			}
			if openaiReq.MaxCompletionTokens != tt.expectMaxCompletion {
				t.Errorf("MaxCompletionTokens = %d, expected %d", openaiReq.MaxCompletionTokens, tt.expectMaxCompletion)
			}
		})
	}
}
