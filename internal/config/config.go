package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
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

	// Performance
	MaxTokensLimit int
	RequestTimeout int

	// Debug logging
	Debug bool

	// Passthrough mode - directly proxy to Anthropic without conversion
	PassthroughMode bool
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
			// File exists, load it
			if err := godotenv.Load(loc); err == nil {
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

		// Performance
		MaxTokensLimit: getEnvAsIntOrDefault("MAX_TOKENS_LIMIT", 400000),
		RequestTimeout: getEnvAsIntOrDefault("REQUEST_TIMEOUT", 90),

		// Passthrough mode
		PassthroughMode: getEnvAsBoolOrDefault("PASSTHROUGH_MODE", false),
	}

	// Validate required fields
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
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

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
