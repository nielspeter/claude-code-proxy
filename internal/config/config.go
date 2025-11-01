// Package config handles configuration loading from environment variables and .env files.
//
// It supports multiple config file locations (./.env, ~/.claude/proxy.env, ~/.claude-code-proxy)
// and detects the provider type (OpenRouter, OpenAI, Ollama) based on the OPENAI_BASE_URL.
// The package also handles model overrides for routing Claude model names to alternative providers.
package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// ProviderType represents the backend provider type
type ProviderType string

const (
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderOpenAI     ProviderType = "openai"
	ProviderOllama     ProviderType = "ollama"
	ProviderUnknown    ProviderType = "unknown"
)

// Config holds all proxy configuration
type Config struct {
	// Required
	OpenAIAPIKey string

	// Optional
	OpenAIBaseURL   string
	AnthropicAPIKey string

	// Model routing (pattern-based if not set)
	OpusModel   string
	SonnetModel string
	HaikuModel  string

	// Server settings
	Host string
	Port string

	// Debug logging
	Debug bool

	// Simple logging - one-line summary per request
	SimpleLog bool

	// Passthrough mode - directly proxy to Anthropic without conversion
	PassthroughMode bool

	// OpenRouter-specific (optional, improves rate limits)
	OpenRouterAppName string
	OpenRouterAppURL  string
}

// Load reads configuration from environment variables
// Tries multiple locations: ./.env, ~/.claude/proxy.env, ~/.claude-code-proxy
func Load() (*Config, error) {
	// Try loading .env files in priority order
	locations := []string{
		".env",
		filepath.Join(os.Getenv("HOME"), ".claude", "proxy.env"),
		filepath.Join(os.Getenv("HOME"), ".claude-code-proxy"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			// File exists, load it (overload to override existing env vars)
			if err := godotenv.Overload(loc); err == nil {
				fmt.Printf("📁 Loaded config from: %s\n", loc)
				break
			}
		}
	}

	// Build config from environment
	cfg := &Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:   getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),

		// Pattern-based routing (optional overrides)
		OpusModel:   os.Getenv("ANTHROPIC_DEFAULT_OPUS_MODEL"),
		SonnetModel: os.Getenv("ANTHROPIC_DEFAULT_SONNET_MODEL"),
		HaikuModel:  os.Getenv("ANTHROPIC_DEFAULT_HAIKU_MODEL"),

		// Server settings
		Host: getEnvOrDefault("HOST", "0.0.0.0"),
		Port: getEnvOrDefault("PORT", "8082"),

		// Passthrough mode
		PassthroughMode: getEnvAsBoolOrDefault("PASSTHROUGH_MODE", false),

		// OpenRouter-specific (optional)
		OpenRouterAppName: os.Getenv("OPENROUTER_APP_NAME"),
		OpenRouterAppURL:  os.Getenv("OPENROUTER_APP_URL"),
	}

	// Validate required fields
	// Allow missing API key for Ollama (localhost endpoints)
	if cfg.OpenAIAPIKey == "" {
		if !strings.Contains(cfg.OpenAIBaseURL, "localhost") &&
			!strings.Contains(cfg.OpenAIBaseURL, "127.0.0.1") {
			return nil, fmt.Errorf("OPENAI_API_KEY is required (unless using localhost/Ollama)")
		}
		// Set dummy key for Ollama
		cfg.OpenAIAPIKey = "ollama"
	}

	return cfg, nil
}

// LoadWithDebug loads config and sets debug mode
func LoadWithDebug(debug bool) (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	cfg.Debug = debug
	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// DetectProvider identifies the provider type based on base URL
func (c *Config) DetectProvider() ProviderType {
	baseURL := strings.ToLower(c.OpenAIBaseURL)

	if strings.Contains(baseURL, "openrouter.ai") {
		return ProviderOpenRouter
	}
	if strings.Contains(baseURL, "api.openai.com") {
		return ProviderOpenAI
	}
	if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
		return ProviderOllama
	}
	return ProviderUnknown
}

// IsLocalhost returns true if the base URL points to localhost
func (c *Config) IsLocalhost() bool {
	baseURL := strings.ToLower(c.OpenAIBaseURL)
	return strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")
}

// ReasoningModelCache stores which models support reasoning capabilities.
// This is fetched from OpenRouter's API on startup to avoid hardcoding model names.
type ReasoningModelCache struct {
	models    map[string]bool // model ID -> supports reasoning
	populated bool
}

// Global cache instance
var reasoningCache = &ReasoningModelCache{
	models: make(map[string]bool),
}

// IsReasoningModel checks if a model supports reasoning capabilities.
// For OpenRouter, this uses the cached API data. Otherwise falls back to pattern matching.
func (c *Config) IsReasoningModel(modelName string) bool {
	// For OpenRouter: use cached data if available
	if c.DetectProvider() == ProviderOpenRouter && reasoningCache.populated {
		if isReasoning, found := reasoningCache.models[modelName]; found {
			return isReasoning
		}
	}

	// Fallback to hardcoded pattern matching (OpenAI Direct, Ollama, or cache miss)
	model := strings.ToLower(modelName)
	model = strings.TrimPrefix(model, "azure/")
	model = strings.TrimPrefix(model, "openai/")

	// Check for o-series reasoning models (o1, o2, o3, o4, etc.)
	if strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o2") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4") {
		return true
	}

	// Check for GPT-5 series (gpt-5, gpt-5-mini, gpt-5-turbo, etc.)
	if strings.HasPrefix(model, "gpt-5") {
		return true
	}

	return false
}

// FetchReasoningModels fetches the list of reasoning-capable models from OpenRouter's API.
// This is called on startup to dynamically detect models that support reasoning,
// avoiding the need to hardcode model names like deepseek-r1, etc.
// No authentication required for this endpoint.
func (c *Config) FetchReasoningModels() error {
	// Only fetch for OpenRouter
	if c.DetectProvider() != ProviderOpenRouter {
		return nil
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// OpenRouter provides a filtered endpoint for reasoning models
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models?supported_parameters=reasoning", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch reasoning models: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Populate cache
	for _, model := range result.Data {
		reasoningCache.models[model.ID] = true
	}
	reasoningCache.populated = true

	if c.Debug {
		fmt.Printf("[DEBUG] Cached %d reasoning models from OpenRouter\n", len(result.Data))
	}

	return nil
}
