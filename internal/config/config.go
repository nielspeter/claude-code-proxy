// Package config handles configuration loading from environment variables and .env files.
//
// It supports multiple config file locations (./.env, ~/.claude/proxy.env, ~/.claude-code-proxy)
// and detects the provider type (OpenRouter, OpenAI, Ollama) based on the OPENAI_BASE_URL.
// The package also handles model overrides for routing Claude model names to alternative providers.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// CacheKey uniquely identifies a (provider, model) combination for capability caching
// Using a struct as map key provides type safety and zero collision risk
type CacheKey struct {
	BaseURL string // Provider base URL (e.g., "https://openrouter.ai/api/v1")
	Model   string // Model name (e.g., "gpt-5", "openai/gpt-5")
}

// ModelCapabilities tracks which parameters a specific model supports
// This is learned dynamically through adaptive retry mechanism
type ModelCapabilities struct {
	UsesMaxCompletionTokens bool      // Does this model use max_completion_tokens?
	LastChecked             time.Time // When was this last verified?
}

// Global capability cache ((baseURL, model) -> capabilities)
// Protected by mutex for thread-safe access across concurrent requests
var (
	modelCapabilityCache = make(map[CacheKey]*ModelCapabilities)
	capabilityCacheMutex sync.RWMutex
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


// GetModelCapabilities retrieves cached capabilities for a (provider, model) combination.
// Returns nil if no capabilities are cached yet (first request for this model).
// Thread-safe with read lock.
func GetModelCapabilities(key CacheKey) *ModelCapabilities {
	capabilityCacheMutex.RLock()
	defer capabilityCacheMutex.RUnlock()
	return modelCapabilityCache[key]
}

// SetModelCapabilities caches the capabilities for a (provider, model) combination.
// This is called after detecting what parameters a specific model supports through adaptive retry.
// Thread-safe with write lock.
func SetModelCapabilities(key CacheKey, capabilities *ModelCapabilities) {
	capabilityCacheMutex.Lock()
	defer capabilityCacheMutex.Unlock()
	capabilities.LastChecked = time.Now()
	modelCapabilityCache[key] = capabilities
}

// ShouldUseMaxCompletionTokens determines if we should send max_completion_tokens
// based on cached model capabilities learned through adaptive detection.
// No hardcoded model patterns - tries max_completion_tokens for ALL models on first request.
func (c *Config) ShouldUseMaxCompletionTokens(modelName string) bool {
	// Build cache key for this (provider, model) combination
	key := CacheKey{
		BaseURL: c.OpenAIBaseURL,
		Model:   modelName,
	}

	// Check if we have cached knowledge about this specific model
	caps := GetModelCapabilities(key)
	if caps != nil {
		// Cache hit - use learned capability
		if c.Debug {
			fmt.Printf("[DEBUG] Cache HIT: %s → max_completion_tokens=%v\n",
				modelName, caps.UsesMaxCompletionTokens)
		}
		return caps.UsesMaxCompletionTokens
	}

	// Cache miss - default to trying max_completion_tokens first
	// The retry mechanism in handlers.go will detect if it's not supported
	// and automatically fall back to max_tokens, then cache the result
	if c.Debug {
		fmt.Printf("[DEBUG] Cache MISS: %s → will auto-detect (try max_completion_tokens)\n", modelName)
	}
	return true
}
