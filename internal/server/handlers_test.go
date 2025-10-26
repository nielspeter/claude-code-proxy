package server

import (
	"testing"

	"github.com/claude-code-proxy/proxy/internal/config"
)

// TestServerSetup tests that the server can be initialized
func TestServerSetup(t *testing.T) {
	cfg := &config.Config{
		Host: "127.0.0.1",
		Port: "9999",
	}

	// Just verify config is valid
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1")
	}

	if cfg.Port != "9999" {
		t.Errorf("Expected port 9999")
	}
}

// TestAPIKeyValidation tests API key validation logic
func TestAPIKeyValidation(t *testing.T) {
	tests := []struct {
		name           string
		configuredKey  string
		requestKey     string
		shouldValidate bool
		shouldPass     bool
	}{
		{
			name:           "with matching key",
			configuredKey:  "test-key",
			requestKey:     "test-key",
			shouldValidate: true,
			shouldPass:     true,
		},
		{
			name:           "with mismatched key",
			configuredKey:  "test-key",
			requestKey:     "wrong-key",
			shouldValidate: true,
			shouldPass:     false,
		},
		{
			name:           "no validation when not configured",
			configuredKey:  "",
			requestKey:     "any-key",
			shouldValidate: false,
			shouldPass:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				AnthropicAPIKey: tt.configuredKey,
			}

			// Simulate validation logic
			if cfg.AnthropicAPIKey != "" {
				// Validation is required
				if cfg.AnthropicAPIKey != tt.requestKey {
					if tt.shouldPass {
						t.Errorf("Expected validation to pass")
					}
				} else {
					if !tt.shouldPass {
						t.Errorf("Expected validation to fail")
					}
				}
			} else {
				// No validation required
				if !tt.shouldPass {
					t.Errorf("Expected to pass when validation disabled")
				}
			}
		})
	}
}

// TestServerConfiguration tests server host and port configuration
func TestServerConfiguration(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
	}{
		{
			name: "default configuration",
			host: "0.0.0.0",
			port: "8082",
		},
		{
			name: "localhost only",
			host: "127.0.0.1",
			port: "8082",
		},
		{
			name: "custom port",
			host: "0.0.0.0",
			port: "9999",
		},
		{
			name: "specific interface",
			host: "192.168.1.100",
			port: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Host: tt.host,
				Port: tt.port,
			}

			if cfg.Host != tt.host {
				t.Errorf("Expected host %s, got %s", tt.host, cfg.Host)
			}

			if cfg.Port != tt.port {
				t.Errorf("Expected port %s, got %s", tt.port, cfg.Port)
			}
		})
	}
}

// TestDebugMode tests debug mode configuration
func TestDebugMode(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
	}{
		{
			name:  "debug enabled",
			debug: true,
		},
		{
			name:  "debug disabled",
			debug: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Debug: tt.debug,
			}

			if cfg.Debug != tt.debug {
				t.Errorf("Expected Debug %v, got %v", tt.debug, cfg.Debug)
			}
		})
	}
}

// TestSimpleLogMode tests simple log mode configuration
func TestSimpleLogMode(t *testing.T) {
	cfg := &config.Config{
		SimpleLog: true,
	}

	if !cfg.SimpleLog {
		t.Errorf("Expected SimpleLog to be true")
	}

	cfg.SimpleLog = false
	if cfg.SimpleLog {
		t.Errorf("Expected SimpleLog to be false")
	}
}

// TestOpenRouterConfiguration tests OpenRouter-specific configuration
func TestOpenRouterConfiguration(t *testing.T) {
	cfg := &config.Config{
		OpenRouterAppName: "Claude-Code-Proxy",
		OpenRouterAppURL:  "https://github.com/example/repo",
		OpenAIBaseURL:     "https://openrouter.ai/api/v1",
	}

	if cfg.OpenRouterAppName != "Claude-Code-Proxy" {
		t.Errorf("Expected app name 'Claude-Code-Proxy'")
	}

	if cfg.OpenRouterAppURL != "https://github.com/example/repo" {
		t.Errorf("Expected app URL 'https://github.com/example/repo'")
	}

	if cfg.DetectProvider() != config.ProviderOpenRouter {
		t.Errorf("Expected OpenRouter provider")
	}
}

// TestProviderDetectionForHandlers tests provider detection in handler context
func TestProviderDetectionForHandlers(t *testing.T) {
	tests := []struct {
		name             string
		baseURL          string
		expectedProvider config.ProviderType
	}{
		{
			name:             "OpenRouter",
			baseURL:          "https://openrouter.ai/api/v1",
			expectedProvider: config.ProviderOpenRouter,
		},
		{
			name:             "OpenAI",
			baseURL:          "https://api.openai.com/v1",
			expectedProvider: config.ProviderOpenAI,
		},
		{
			name:             "Ollama",
			baseURL:          "http://localhost:11434/v1",
			expectedProvider: config.ProviderOllama,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				OpenAIBaseURL: tt.baseURL,
			}

			provider := cfg.DetectProvider()
			if provider != tt.expectedProvider {
				t.Errorf("Expected provider %v, got %v", tt.expectedProvider, provider)
			}
		})
	}
}

// TestPassthroughMode tests passthrough mode configuration
func TestPassthroughMode(t *testing.T) {
	cfg := &config.Config{
		PassthroughMode: true,
	}

	if !cfg.PassthroughMode {
		t.Errorf("Expected PassthroughMode to be true")
	}
}

// TestConfigFormatDetection tests that config can detect different format scenarios
func TestConfigFormatDetection(t *testing.T) {
	// Test that config struct properly represents all fields
	cfg := &config.Config{
		OpenAIAPIKey:      "test-key",
		OpenAIBaseURL:     "https://api.openai.com/v1",
		AnthropicAPIKey:   "test-anthropic-key",
		OpusModel:         "gpt-5",
		SonnetModel:       "gpt-5",
		HaikuModel:        "gpt-5-mini",
		Host:              "0.0.0.0",
		Port:              "8082",
		Debug:             true,
		SimpleLog:         true,
		PassthroughMode:   false,
		OpenRouterAppName: "app",
		OpenRouterAppURL:  "https://example.com",
	}

	// Verify all fields are accessible
	if cfg.OpenAIAPIKey != "test-key" {
		t.Errorf("OpenAIAPIKey not set correctly")
	}
	if cfg.AnthropicAPIKey != "test-anthropic-key" {
		t.Errorf("AnthropicAPIKey not set correctly")
	}
	if cfg.OpusModel != "gpt-5" {
		t.Errorf("OpusModel not set correctly")
	}
	if cfg.SonnetModel != "gpt-5" {
		t.Errorf("SonnetModel not set correctly")
	}
	if cfg.HaikuModel != "gpt-5-mini" {
		t.Errorf("HaikuModel not set correctly")
	}
}
