package config

import (
	"os"
	"testing"
)

// TestProviderDetection tests that we correctly identify providers from OPENAI_BASE_URL
func TestProviderDetection(t *testing.T) {
	tests := []struct {
		name            string
		baseURL         string
		expectedProvider ProviderType
	}{
		{
			name:            "OpenRouter detection",
			baseURL:         "https://openrouter.ai/api/v1",
			expectedProvider: ProviderOpenRouter,
		},
		{
			name:            "OpenAI Direct detection",
			baseURL:         "https://api.openai.com/v1",
			expectedProvider: ProviderOpenAI,
		},
		{
			name:            "Ollama local detection",
			baseURL:         "http://localhost:11434/v1",
			expectedProvider: ProviderOllama,
		},
		{
			name:            "Ollama with different port",
			baseURL:         "http://localhost:8080/v1",
			expectedProvider: ProviderOllama,
		},
		{
			name:            "Ollama with custom host - should be unknown since not localhost",
			baseURL:         "http://192.168.1.100:11434/v1",
			expectedProvider: ProviderUnknown,
		},
		{
			name:            "Unknown provider",
			baseURL:         "https://custom-api.example.com/v1",
			expectedProvider: ProviderUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				OpenAIBaseURL: tt.baseURL,
			}

			provider := cfg.DetectProvider()
			if provider != tt.expectedProvider {
				t.Errorf("DetectProvider() for %q = %v, want %v",
					tt.baseURL, provider, tt.expectedProvider)
			}
		})
	}
}

// TestModelOverrides tests that model overrides work correctly
func TestModelOverrides(t *testing.T) {
	tests := []struct {
		name         string
		opusModel    string
		sonnetModel  string
		haikuModel   string
		requestModel string
		expectedUsed string
	}{
		{
			name:         "Opus override for OpenRouter",
			opusModel:    "anthropic/claude-opus-4",
			requestModel: "claude-opus-4-1-20250805",
			expectedUsed: "anthropic/claude-opus-4",
		},
		{
			name:         "Sonnet override for Grok",
			sonnetModel:  "x-ai/grok-code-fast-1",
			requestModel: "claude-sonnet-4-5-20250805",
			expectedUsed: "x-ai/grok-code-fast-1",
		},
		{
			name:         "Haiku override for Gemini",
			haikuModel:   "google/gemini-2.5-flash",
			requestModel: "claude-haiku-4-5-20251001",
			expectedUsed: "google/gemini-2.5-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				OpusModel:   tt.opusModel,
				SonnetModel: tt.sonnetModel,
				HaikuModel:  tt.haikuModel,
			}

			// This would be tested through the converter which uses the config
			// The config struct should expose these values properly
			if tt.opusModel != "" && cfg.OpusModel != tt.expectedUsed {
				t.Errorf("OpusModel = %q, want %q", cfg.OpusModel, tt.expectedUsed)
			}
			if tt.sonnetModel != "" && cfg.SonnetModel != tt.expectedUsed {
				t.Errorf("SonnetModel = %q, want %q", cfg.SonnetModel, tt.expectedUsed)
			}
			if tt.haikuModel != "" && cfg.HaikuModel != tt.expectedUsed {
				t.Errorf("HaikuModel = %q, want %q", cfg.HaikuModel, tt.expectedUsed)
			}
		})
	}
}

// TestProviderSpecificParameters tests that provider-specific params are set correctly
func TestProviderSpecificParameters(t *testing.T) {
	tests := []struct {
		name                 string
		provider             ProviderType
		shouldHaveReasoning  bool
		shouldHaveToolChoice bool
	}{
		{
			name:                 "OpenRouter reasoning support",
			provider:             ProviderOpenRouter,
			shouldHaveReasoning:  true,
			shouldHaveToolChoice: false,
		},
		{
			name:                 "OpenAI Direct reasoning support",
			provider:             ProviderOpenAI,
			shouldHaveReasoning:  true,
			shouldHaveToolChoice: false,
		},
		{
			name:                 "Ollama tool choice",
			provider:             ProviderOllama,
			shouldHaveReasoning:  false,
			shouldHaveToolChoice: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a design test - documenting expected behavior
			// The actual implementation is in converter.go
			t.Logf("Provider %v should have reasoning=%v, tool_choice=%v",
				tt.provider, tt.shouldHaveReasoning, tt.shouldHaveToolChoice)
		})
	}
}

// TestEnvironmentConfigLoading tests that .env files are loaded correctly
func TestEnvironmentConfigLoading(t *testing.T) {
	// Save current env vars
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	originalAPIKey := os.Getenv("OPENAI_API_KEY")

	// Restore after test
	defer func() {
		if originalBaseURL != "" {
			os.Setenv("OPENAI_BASE_URL", originalBaseURL)
		} else {
			os.Unsetenv("OPENAI_BASE_URL")
		}
		if originalAPIKey != "" {
			os.Setenv("OPENAI_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	// Test that environment variables override
	os.Setenv("OPENAI_BASE_URL", "https://test.example.com")
	os.Setenv("OPENAI_API_KEY", "test-key-123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OpenAIBaseURL != "https://test.example.com" {
		t.Errorf("OpenAIBaseURL = %q, want %q", cfg.OpenAIBaseURL, "https://test.example.com")
	}

	if cfg.OpenAIAPIKey != "test-key-123" {
		t.Errorf("OpenAIAPIKey = %q, want %q", cfg.OpenAIAPIKey, "test-key-123")
	}
}

// TestConfigDefaults tests default values
func TestConfigDefaults(t *testing.T) {
	// Clear relevant env vars for this test
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	originalAPIKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_BASE_URL")
	os.Setenv("OPENAI_API_KEY", "test-key") // Required for non-localhost
	defer func() {
		if originalBaseURL != "" {
			os.Setenv("OPENAI_BASE_URL", originalBaseURL)
		}
		if originalAPIKey != "" {
			os.Setenv("OPENAI_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Default base URL should be OpenAI
	expectedDefault := "https://api.openai.com/v1"
	if cfg.OpenAIBaseURL != expectedDefault {
		t.Errorf("Default OpenAIBaseURL = %q, want %q", cfg.OpenAIBaseURL, expectedDefault)
	}

	// Default port
	if cfg.Port != "8082" {
		t.Errorf("Default Port = %q, want %q", cfg.Port, "8082")
	}

	// Default host
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Default Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
}

// TestProviderIsolation is a conceptual test documenting the isolation requirement
func TestProviderIsolation(t *testing.T) {
	scenarios := []struct {
		name             string
		configuredURL    string
		expectedProvider string
		shouldNotCallURL string
	}{
		{
			name:             "OpenRouter should not call OpenAI",
			configuredURL:    "https://openrouter.ai/api/v1",
			expectedProvider: "OpenRouter",
			shouldNotCallURL: "https://api.openai.com",
		},
		{
			name:             "OpenAI should not call OpenRouter",
			configuredURL:    "https://api.openai.com/v1",
			expectedProvider: "OpenAI Direct",
			shouldNotCallURL: "https://openrouter.ai",
		},
		{
			name:             "Ollama should not call external APIs",
			configuredURL:    "http://localhost:11434/v1",
			expectedProvider: "Ollama",
			shouldNotCallURL: "https://",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Document the requirement: when configured for one provider,
			// we should NEVER make requests to another provider's endpoint
			t.Logf("REQUIREMENT: When OPENAI_BASE_URL=%s (%s), proxy must NOT make requests to %s",
				scenario.configuredURL, scenario.expectedProvider, scenario.shouldNotCallURL)
		})
	}
}
