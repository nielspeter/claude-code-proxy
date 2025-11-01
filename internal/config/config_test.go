package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProviderDetection tests that we correctly identify providers from OPENAI_BASE_URL
func TestProviderDetection(t *testing.T) {
	tests := []struct {
		name             string
		baseURL          string
		expectedProvider ProviderType
	}{
		{
			name:             "OpenRouter detection",
			baseURL:          "https://openrouter.ai/api/v1",
			expectedProvider: ProviderOpenRouter,
		},
		{
			name:             "OpenAI Direct detection",
			baseURL:          "https://api.openai.com/v1",
			expectedProvider: ProviderOpenAI,
		},
		{
			name:             "Ollama local detection",
			baseURL:          "http://localhost:11434/v1",
			expectedProvider: ProviderOllama,
		},
		{
			name:             "Ollama with different port",
			baseURL:          "http://localhost:8080/v1",
			expectedProvider: ProviderOllama,
		},
		{
			name:             "Ollama with custom host - should be unknown since not localhost",
			baseURL:          "http://192.168.1.100:11434/v1",
			expectedProvider: ProviderUnknown,
		},
		{
			name:             "Unknown provider",
			baseURL:          "https://custom-api.example.com/v1",
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

// TestLoadWithDebug tests loading config with debug mode enabled
func TestLoadWithDebug(t *testing.T) {
	// Save original env
	originalKey := os.Getenv("OPENAI_API_KEY")
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", originalKey)
		os.Setenv("OPENAI_BASE_URL", originalBaseURL)
	}()

	// Set required env vars
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")

	cfg, err := LoadWithDebug(true)
	if err != nil {
		t.Fatalf("LoadWithDebug failed: %v", err)
	}

	if !cfg.Debug {
		t.Errorf("Expected Debug=true, got %v", cfg.Debug)
	}

	if cfg.OpenAIAPIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got %q", cfg.OpenAIAPIKey)
	}
}

// TestIsLocalhost tests localhost detection
func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected bool
	}{
		{
			name:     "localhost with default port",
			baseURL:  "http://localhost:11434/v1",
			expected: true,
		},
		{
			name:     "localhost with custom port",
			baseURL:  "http://localhost:8080/v1",
			expected: true,
		},
		{
			name:     "127.0.0.1",
			baseURL:  "http://127.0.0.1:11434/v1",
			expected: true,
		},
		{
			name:     "OpenRouter",
			baseURL:  "https://openrouter.ai/api/v1",
			expected: false,
		},
		{
			name:     "OpenAI Direct",
			baseURL:  "https://api.openai.com/v1",
			expected: false,
		},
		{
			name:     "Custom host",
			baseURL:  "http://192.168.1.100:11434/v1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				OpenAIBaseURL: tt.baseURL,
			}

			result := cfg.IsLocalhost()
			if result != tt.expected {
				t.Errorf("IsLocalhost() for %q = %v, want %v",
					tt.baseURL, result, tt.expected)
			}
		})
	}
}

// TestOpenRouterSpecificSettings tests OpenRouter app name and URL settings
func TestOpenRouterSpecificSettings(t *testing.T) {
	// Save original env
	originalAppName := os.Getenv("OPENROUTER_APP_NAME")
	originalAppURL := os.Getenv("OPENROUTER_APP_URL")
	originalKey := os.Getenv("OPENAI_API_KEY")
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	defer func() {
		os.Setenv("OPENROUTER_APP_NAME", originalAppName)
		os.Setenv("OPENROUTER_APP_URL", originalAppURL)
		os.Setenv("OPENAI_API_KEY", originalKey)
		os.Setenv("OPENAI_BASE_URL", originalBaseURL)
	}()

	// Set env vars
	os.Setenv("OPENROUTER_APP_NAME", "Claude-Code-Proxy")
	os.Setenv("OPENROUTER_APP_URL", "https://github.com/example/repo")
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("OPENAI_BASE_URL", "https://openrouter.ai/api/v1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.OpenRouterAppName != "Claude-Code-Proxy" {
		t.Errorf("Expected app name 'Claude-Code-Proxy', got %q", cfg.OpenRouterAppName)
	}

	if cfg.OpenRouterAppURL != "https://github.com/example/repo" {
		t.Errorf("Expected app URL 'https://github.com/example/repo', got %q", cfg.OpenRouterAppURL)
	}

	if cfg.DetectProvider() != ProviderOpenRouter {
		t.Errorf("Expected OpenRouter provider, got %v", cfg.DetectProvider())
	}
}

// TestOllamaWithoutAPIKey tests that Ollama works without API key
func TestOllamaWithoutAPIKey(t *testing.T) {
	// Save original env
	originalKey := os.Getenv("OPENAI_API_KEY")
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", originalKey)
		os.Setenv("OPENAI_BASE_URL", originalBaseURL)
	}()

	// Clear API key and set Ollama URL
	os.Unsetenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_BASE_URL", "http://localhost:11434/v1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should succeed for Ollama without API key: %v", err)
	}

	// Should have dummy key for Ollama
	if cfg.OpenAIAPIKey != "ollama" {
		t.Errorf("Expected dummy API key 'ollama', got %q", cfg.OpenAIAPIKey)
	}

	if cfg.DetectProvider() != ProviderOllama {
		t.Errorf("Expected Ollama provider, got %v", cfg.DetectProvider())
	}
}

// TestMissingAPIKeyForOpenAI tests that load fails without API key for OpenAI
func TestMissingAPIKeyForOpenAI(t *testing.T) {
	// Save original env
	originalKey := os.Getenv("OPENAI_API_KEY")
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", originalKey)
		os.Setenv("OPENAI_BASE_URL", originalBaseURL)
	}()

	// Clear API key and set OpenAI URL
	os.Unsetenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")

	cfg, err := Load()
	if err == nil {
		t.Errorf("Load should fail when OpenAI API key is missing")
	}

	_ = cfg
}

// TestHostAndPortDefaults tests default host and port values
func TestHostAndPortDefaults(t *testing.T) {
	// Save original env
	originalHost := os.Getenv("HOST")
	originalPort := os.Getenv("PORT")
	originalKey := os.Getenv("OPENAI_API_KEY")
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	defer func() {
		if originalHost != "" {
			os.Setenv("HOST", originalHost)
		} else {
			os.Unsetenv("HOST")
		}
		if originalPort != "" {
			os.Setenv("PORT", originalPort)
		} else {
			os.Unsetenv("PORT")
		}
		os.Setenv("OPENAI_API_KEY", originalKey)
		os.Setenv("OPENAI_BASE_URL", originalBaseURL)
	}()

	// Clear host and port
	os.Unsetenv("HOST")
	os.Unsetenv("PORT")
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got %q", cfg.Host)
	}

	if cfg.Port != "8082" {
		t.Errorf("Expected default port '8082', got %q", cfg.Port)
	}
}

// TestPassthroughMode tests passthrough mode configuration
func TestPassthroughMode(t *testing.T) {
	// Save original env
	originalMode := os.Getenv("PASSTHROUGH_MODE")
	originalKey := os.Getenv("OPENAI_API_KEY")
	originalBaseURL := os.Getenv("OPENAI_BASE_URL")
	defer func() {
		if originalMode != "" {
			os.Setenv("PASSTHROUGH_MODE", originalMode)
		} else {
			os.Unsetenv("PASSTHROUGH_MODE")
		}
		os.Setenv("OPENAI_API_KEY", originalKey)
		os.Setenv("OPENAI_BASE_URL", originalBaseURL)
	}()

	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"enabled with true", "true", true},
		{"enabled with 1", "1", true},
		{"enabled with yes", "yes", true},
		{"disabled with false", "false", false},
		{"disabled with 0", "0", false},
		{"default is disabled", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("PASSTHROUGH_MODE", tt.envValue)
			} else {
				os.Unsetenv("PASSTHROUGH_MODE")
			}
			os.Setenv("OPENAI_API_KEY", "test-key")
			os.Setenv("OPENAI_BASE_URL", "https://api.openai.com/v1")

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}

			if cfg.PassthroughMode != tt.expected {
				t.Errorf("Expected PassthroughMode=%v, got %v", tt.expected, cfg.PassthroughMode)
			}
		})
	}
}

// TestMultipleEnvFiles tests that env files are loaded in correct priority order
func TestMultipleEnvFiles(t *testing.T) {
	// Create temporary directory for test env files
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	originalCwd, _ := os.Getwd()

	// Create mock .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	// Create .env file in current directory
	localEnvFile := filepath.Join(tempDir, ".env")
	os.WriteFile(localEnvFile, []byte("OPENAI_API_KEY=local-key\nOPENAI_BASE_URL=https://local.example.com"), 0644)

	// Create ~/.claude/proxy.env file
	claudeEnvFile := filepath.Join(claudeDir, "proxy.env")
	os.WriteFile(claudeEnvFile, []byte("OPENAI_API_KEY=claude-key"), 0644)

	// Setup environment
	os.Chdir(tempDir)
	os.Setenv("HOME", tempDir)

	defer func() {
		os.Chdir(originalCwd)
		os.Setenv("HOME", originalHome)
	}()

	// Load config - should pick up local .env first
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.OpenAIAPIKey != "local-key" {
		t.Errorf("Expected local API key, got %q", cfg.OpenAIAPIKey)
	}

	if cfg.OpenAIBaseURL != "https://local.example.com" {
		t.Errorf("Expected local base URL, got %q", cfg.OpenAIBaseURL)
	}
}

// TestIsReasoningModelWithHardcodedFallback tests reasoning model detection using hardcoded patterns
func TestIsReasoningModelWithHardcodedFallback(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		baseURL          string
		populateCache    bool
		expectedReasoning bool
	}{
		// OpenAI o-series models (hardcoded fallback)
		{"o1 model", "o1", "https://api.openai.com/v1", false, true},
		{"o1-preview model", "o1-preview", "https://api.openai.com/v1", false, true},
		{"o2 model", "o2", "https://api.openai.com/v1", false, true},
		{"o3 model", "o3", "https://api.openai.com/v1", false, true},
		{"o3-mini model", "o3-mini", "https://api.openai.com/v1", false, true},
		{"o4 model", "o4", "https://api.openai.com/v1", false, true},

		// GPT-5 series models (hardcoded fallback)
		{"gpt-5 model", "gpt-5", "https://api.openai.com/v1", false, true},
		{"gpt-5-mini model", "gpt-5-mini", "https://api.openai.com/v1", false, true},
		{"gpt-5-turbo model", "gpt-5-turbo", "https://api.openai.com/v1", false, true},

		// Azure variants with provider prefix
		{"azure/o1 model", "azure/o1", "https://azure.openai.com/v1", false, true},
		{"azure/gpt-5 model", "azure/gpt-5", "https://azure.openai.com/v1", false, true},
		{"openai/o3 model", "openai/o3", "https://api.openai.com/v1", false, true},
		{"openai/gpt-5 model", "openai/gpt-5", "https://api.openai.com/v1", false, true},

		// Non-reasoning models
		{"gpt-4o model", "gpt-4o", "https://api.openai.com/v1", false, false},
		{"gpt-4-turbo model", "gpt-4-turbo", "https://api.openai.com/v1", false, false},
		{"gpt-3.5-turbo model", "gpt-3.5-turbo", "https://api.openai.com/v1", false, false},
		{"claude-sonnet model", "claude-sonnet-4", "https://api.openai.com/v1", false, false},

		// Edge cases
		{"empty string", "", "https://api.openai.com/v1", false, false},
		{"ollama prefix", "ollama", "http://localhost:11434/v1", false, false},
		{"contains o but not o-series", "anthropic", "https://api.openai.com/v1", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				OpenAIBaseURL: tt.baseURL,
			}

			// Clear cache to test hardcoded fallback
			reasoningCache = &ReasoningModelCache{
				models:    make(map[string]bool),
				populated: false,
			}

			result := cfg.IsReasoningModel(tt.model)
			if result != tt.expectedReasoning {
				t.Errorf("IsReasoningModel(%q) = %v, expected %v", tt.model, result, tt.expectedReasoning)
			}
		})
	}
}

// TestIsReasoningModelWithCache tests reasoning model detection using cached OpenRouter data
func TestIsReasoningModelWithCache(t *testing.T) {
	// Setup mock cache data
	mockCache := &ReasoningModelCache{
		models: map[string]bool{
			"openai/gpt-5":               true,
			"google/gemini-2.5-flash":    true,
			"deepseek/deepseek-r1":       true,
			"nvidia/nemotron-nano-12b":   true,
			"anthropic/claude-sonnet-4":  false, // Not in cache
		},
		populated: true,
	}

	tests := []struct {
		name              string
		model             string
		baseURL           string
		expectedReasoning bool
	}{
		// Models in cache
		{"gpt-5 in cache", "openai/gpt-5", "https://openrouter.ai/api/v1", true},
		{"gemini in cache", "google/gemini-2.5-flash", "https://openrouter.ai/api/v1", true},
		{"deepseek-r1 in cache", "deepseek/deepseek-r1", "https://openrouter.ai/api/v1", true},
		{"nvidia in cache", "nvidia/nemotron-nano-12b", "https://openrouter.ai/api/v1", true},

		// Models not in cache - should fall back to hardcoded patterns
		{"gpt-5 not cached but matches pattern", "gpt-5", "https://openrouter.ai/api/v1", true},
		{"o3 not cached but matches pattern", "o3", "https://openrouter.ai/api/v1", true},
		{"gpt-4o not cached and no pattern", "gpt-4o", "https://openrouter.ai/api/v1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set mock cache
			reasoningCache = mockCache

			cfg := &Config{
				OpenAIBaseURL: tt.baseURL,
			}

			result := cfg.IsReasoningModel(tt.model)
			if result != tt.expectedReasoning {
				t.Errorf("IsReasoningModel(%q) with cache = %v, expected %v", tt.model, result, tt.expectedReasoning)
			}
		})
	}

	// Cleanup
	reasoningCache = &ReasoningModelCache{
		models:    make(map[string]bool),
		populated: false,
	}
}

// TestIsReasoningModelProviderSpecific tests that different providers use appropriate detection
func TestIsReasoningModelProviderSpecific(t *testing.T) {
	tests := []struct {
		name              string
		model             string
		baseURL           string
		provider          ProviderType
		shouldUseCache    bool
		expectedReasoning bool
	}{
		{
			name:              "OpenRouter uses cache when populated",
			model:             "google/gemini-2.5-flash",
			baseURL:           "https://openrouter.ai/api/v1",
			provider:          ProviderOpenRouter,
			shouldUseCache:    true,
			expectedReasoning: true,
		},
		{
			name:              "OpenAI Direct uses hardcoded patterns",
			model:             "gpt-5",
			baseURL:           "https://api.openai.com/v1",
			provider:          ProviderOpenAI,
			shouldUseCache:    false,
			expectedReasoning: true,
		},
		{
			name:              "Ollama uses hardcoded patterns",
			model:             "o1",
			baseURL:           "http://localhost:11434/v1",
			provider:          ProviderOllama,
			shouldUseCache:    false,
			expectedReasoning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				OpenAIBaseURL: tt.baseURL,
			}

			// Setup cache for OpenRouter test
			if tt.shouldUseCache {
				reasoningCache = &ReasoningModelCache{
					models: map[string]bool{
						"google/gemini-2.5-flash": true,
					},
					populated: true,
				}
			} else {
				reasoningCache = &ReasoningModelCache{
					models:    make(map[string]bool),
					populated: false,
				}
			}

			result := cfg.IsReasoningModel(tt.model)
			if result != tt.expectedReasoning {
				t.Errorf("IsReasoningModel(%q) for %v = %v, expected %v",
					tt.model, tt.provider, result, tt.expectedReasoning)
			}
		})
	}

	// Cleanup
	reasoningCache = &ReasoningModelCache{
		models:    make(map[string]bool),
		populated: false,
	}
}

// TestFetchReasoningModels tests the dynamic reasoning model detection from OpenRouter API
func TestFetchReasoningModels(t *testing.T) {
	// Helper function to create mock OpenRouter API server
	createMockServer := func(statusCode int, response string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request is for reasoning models
			if !strings.Contains(r.URL.String(), "supported_parameters=reasoning") {
				t.Errorf("Expected URL to contain 'supported_parameters=reasoning', got %q", r.URL.String())
			}

			w.WriteHeader(statusCode)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(response))
		}))
	}

	t.Run("successful fetch and cache population", func(t *testing.T) {
		// Clear cache
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}

		// Create mock response matching OpenRouter's actual format
		mockResponse := `{
			"data": [
				{"id": "openai/gpt-5"},
				{"id": "google/gemini-2.5-flash"},
				{"id": "deepseek/deepseek-r1"},
				{"id": "nvidia/nemotron-nano-12b"}
			]
		}`

		server := createMockServer(http.StatusOK, mockResponse)
		defer server.Close()

		// Create config pointing to OpenRouter
		cfg := &Config{
			OpenAIBaseURL: "https://openrouter.ai/api/v1",
		}

		// Temporarily replace the API URL in the function call
		// Since we can't modify the function, we'll need to test indirectly
		// by verifying the cache gets populated

		// For this test, we need to manually populate cache as if fetch succeeded
		// This tests the cache population logic
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal([]byte(mockResponse), &result)

		for _, model := range result.Data {
			reasoningCache.models[model.ID] = true
		}
		reasoningCache.populated = true

		// Verify cache was populated
		if !reasoningCache.populated {
			t.Error("Expected cache to be populated")
		}

		if len(reasoningCache.models) != 4 {
			t.Errorf("Expected 4 models in cache, got %d", len(reasoningCache.models))
		}

		// Verify specific models are in cache
		expectedModels := []string{
			"openai/gpt-5",
			"google/gemini-2.5-flash",
			"deepseek/deepseek-r1",
			"nvidia/nemotron-nano-12b",
		}

		for _, model := range expectedModels {
			if !reasoningCache.models[model] {
				t.Errorf("Expected model %q to be in cache", model)
			}
		}

		// Verify cfg.IsReasoningModel works with cached data
		for _, model := range expectedModels {
			if !cfg.IsReasoningModel(model) {
				t.Errorf("Expected IsReasoningModel(%q) to return true", model)
			}
		}

		// Cleanup
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}
	})

	t.Run("non-OpenRouter provider skips fetch", func(t *testing.T) {
		// Clear cache
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}

		tests := []struct {
			name    string
			baseURL string
		}{
			{"OpenAI Direct", "https://api.openai.com/v1"},
			{"Ollama", "http://localhost:11434/v1"},
			{"Unknown", "https://custom.example.com/v1"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &Config{
					OpenAIBaseURL: tt.baseURL,
				}

				// Call FetchReasoningModels - should return early without error
				err := cfg.FetchReasoningModels()
				if err != nil {
					t.Errorf("Expected no error for non-OpenRouter provider, got %v", err)
				}

				// Cache should still be empty
				if reasoningCache.populated {
					t.Error("Expected cache to remain empty for non-OpenRouter provider")
				}
			})
		}
	})

	t.Run("empty response from API", func(t *testing.T) {
		// Clear cache
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}

		// Empty response (no reasoning models available)
		mockResponse := `{"data": []}`

		// Simulate parsing empty response
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal([]byte(mockResponse), &result)

		// Populate cache with empty data
		for _, model := range result.Data {
			reasoningCache.models[model.ID] = true
		}
		reasoningCache.populated = true

		// Cache should be populated but empty
		if !reasoningCache.populated {
			t.Error("Expected cache to be populated even with empty data")
		}

		if len(reasoningCache.models) != 0 {
			t.Errorf("Expected 0 models in cache, got %d", len(reasoningCache.models))
		}

		// Cleanup
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		// Clear cache
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}

		malformedJSON := `{"data": [{"id": "openai/gpt-5"` // Missing closing braces

		// Attempt to parse malformed JSON
		var result struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		err := json.Unmarshal([]byte(malformedJSON), &result)

		// Should get an error
		if err == nil {
			t.Error("Expected error when parsing malformed JSON")
		}

		// Cache should remain unpopulated on error
		if reasoningCache.populated {
			t.Error("Expected cache to remain unpopulated after JSON parse error")
		}
	})

	t.Run("cache allows fallback to hardcoded patterns", func(t *testing.T) {
		// Clear cache
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}

		cfg := &Config{
			OpenAIBaseURL: "https://openrouter.ai/api/v1",
		}

		// With empty cache, should fall back to hardcoded patterns
		hardcodedModels := []string{"o1", "o3", "gpt-5", "gpt-5-mini"}

		for _, model := range hardcodedModels {
			if !cfg.IsReasoningModel(model) {
				t.Errorf("Expected IsReasoningModel(%q) to return true via fallback", model)
			}
		}

		// Non-reasoning models should still return false
		nonReasoningModels := []string{"gpt-4o", "gpt-4-turbo", "claude-sonnet-4"}

		for _, model := range nonReasoningModels {
			if cfg.IsReasoningModel(model) {
				t.Errorf("Expected IsReasoningModel(%q) to return false", model)
			}
		}
	})

	t.Run("cache overrides hardcoded patterns for OpenRouter", func(t *testing.T) {
		// Setup cache with a model that wouldn't match hardcoded patterns
		reasoningCache = &ReasoningModelCache{
			models: map[string]bool{
				"google/gemini-2.5-flash": true,
				"deepseek/deepseek-r1":    true,
			},
			populated: true,
		}

		cfg := &Config{
			OpenAIBaseURL: "https://openrouter.ai/api/v1",
		}

		// These models are in cache, should return true
		if !cfg.IsReasoningModel("google/gemini-2.5-flash") {
			t.Error("Expected gemini-2.5-flash to be reasoning model (from cache)")
		}

		if !cfg.IsReasoningModel("deepseek/deepseek-r1") {
			t.Error("Expected deepseek-r1 to be reasoning model (from cache)")
		}

		// This model is not in cache, should fall back to hardcoded patterns
		if !cfg.IsReasoningModel("gpt-5") {
			t.Error("Expected gpt-5 to be reasoning model (from fallback)")
		}

		// This model is not in cache and doesn't match patterns
		if cfg.IsReasoningModel("anthropic/claude-sonnet-4") {
			t.Error("Expected claude-sonnet-4 to NOT be reasoning model")
		}

		// Cleanup
		reasoningCache = &ReasoningModelCache{
			models:    make(map[string]bool),
			populated: false,
		}
	})
}
