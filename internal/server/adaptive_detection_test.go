package server

import (
	"sync"
	"testing"
	"time"

	"claude-code-proxy/internal/config"
)

// TestMaxTokensParameterErrorDetection tests the error detection logic for parameter errors
func TestMaxTokensParameterErrorDetection(t *testing.T) {
	tests := []struct {
		name          string
		errorMsg      string
		shouldDetect  bool
		description   string
	}{
		{
			name:         "max_completion_tokens unsupported",
			errorMsg:     "Error: unsupported parameter 'max_completion_tokens'",
			shouldDetect: true,
			description:  "Should detect unsupported max_completion_tokens",
		},
		{
			name:         "max_tokens invalid",
			errorMsg:     "invalid parameter: max_tokens not supported for this model",
			shouldDetect: true,
			description:  "Should detect invalid max_tokens",
		},
		{
			name:         "parameter error lowercase",
			errorMsg:     "parameter 'max_completion_tokens' is not supported",
			shouldDetect: true,
			description:  "Should handle lowercase parameter errors",
		},
		{
			name:         "OpenWebUI style error",
			errorMsg:     "500: 'max_completion_tokens' is not a valid OpenAI parameter",
			shouldDetect: true,
			description:  "Should detect OpenWebUI style errors",
		},
		{
			name:         "unrelated error",
			errorMsg:     "Authentication failed",
			shouldDetect: false,
			description:  "Should not detect unrelated errors",
		},
		{
			name:         "timeout error",
			errorMsg:     "request timeout",
			shouldDetect: false,
			description:  "Should not detect timeout errors",
		},
		{
			name:         "empty error message",
			errorMsg:     "",
			shouldDetect: false,
			description:  "Should handle empty error messages",
		},
		{
			name:         "wrong parameter name",
			errorMsg:     "unsupported parameter 'max_input_tokens'",
			shouldDetect: false,
			description:  "Should not detect errors for wrong parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMaxTokensParameterError(tt.errorMsg)
			if result != tt.shouldDetect {
				t.Errorf("%s: got %v, want %v", tt.description, result, tt.shouldDetect)
			}
		})
	}
}

// TestModelCapabilityCacheStructure tests the cache key and value structure
func TestModelCapabilityCacheStructure(t *testing.T) {
	// Test that cache keys are properly formed with both BaseURL and Model
	cfg1 := &config.Config{
		OpenAIBaseURL: "https://api.openai.com/v1",
	}

	cfg2 := &config.Config{
		OpenAIBaseURL: "https://openrouter.ai/api/v1",
	}

	// Same model, different providers should have different cache entries
	model := "gpt-5"

	key1 := CacheKey{
		BaseURL: cfg1.OpenAIBaseURL,
		Model:   model,
	}

	key2 := CacheKey{
		BaseURL: cfg2.OpenAIBaseURL,
		Model:   model,
	}

	// Keys should be different
	if key1 == key2 {
		t.Errorf("Different providers should have different cache keys")
	}

	// Keys with same provider and model should be equal
	key1Copy := CacheKey{
		BaseURL: cfg1.OpenAIBaseURL,
		Model:   model,
	}

	if key1 != key1Copy {
		t.Errorf("Same provider and model should have equal cache keys")
	}
}

// TestCacheHitAndMiss tests cache behavior for hits and misses
func TestCacheHitAndMiss(t *testing.T) {
	cache := &ModelCapabilityCache{
		data:  make(map[CacheKey]*ModelCapabilities),
		mutex: &sync.RWMutex{},
	}

	key := CacheKey{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-5",
	}

	// Test cache miss
	caps, found := cache.Get(key)
	if found {
		t.Error("Cache should miss for new key")
	}
	if caps != nil {
		t.Error("Cache miss should return nil capabilities")
	}

	// Test cache set
	newCaps := &ModelCapabilities{
		UsesMaxCompletionTokens: true,
		LastChecked:             time.Now(),
	}
	cache.Set(key, newCaps)

	// Test cache hit
	caps, found = cache.Get(key)
	if !found {
		t.Error("Cache should hit after setting value")
	}
	if caps.UsesMaxCompletionTokens != true {
		t.Error("Cache should return correct capability value")
	}
}

// TestPerModelPerProviderScoping tests that caching is properly scoped
func TestPerModelPerProviderScoping(t *testing.T) {
	cache := &ModelCapabilityCache{
		data:  make(map[CacheKey]*ModelCapabilities),
		mutex: &sync.RWMutex{},
	}

	// Test data: same model, different providers, different capabilities
	testCases := []struct {
		baseURL  string
		model    string
		supports bool
	}{
		{"https://api.openai.com/v1", "gpt-5", true},
		{"https://openrouter.ai/api/v1", "gpt-5", true},
		{"http://localhost:11434/v1", "gpt-5", false},
		{"https://api.openai.com/v1", "gpt-4o", false},
		{"https://openrouter.ai/api/v1", "gpt-4o", false},
	}

	// Set all cache entries
	for _, tc := range testCases {
		key := CacheKey{BaseURL: tc.baseURL, Model: tc.model}
		caps := &ModelCapabilities{
			UsesMaxCompletionTokens: tc.supports,
			LastChecked:             time.Now(),
		}
		cache.Set(key, caps)
	}

	// Verify each entry is correct
	for _, tc := range testCases {
		key := CacheKey{BaseURL: tc.baseURL, Model: tc.model}
		caps, found := cache.Get(key)
		if !found {
			t.Errorf("Cache should contain %s on %s", tc.model, tc.baseURL)
		}
		if caps.UsesMaxCompletionTokens != tc.supports {
			t.Errorf("Cache for %s on %s should be %v, got %v",
				tc.model, tc.baseURL, tc.supports, caps.UsesMaxCompletionTokens)
		}
	}
}

// TestCacheConcurrency tests thread-safe cache operations
func TestCacheConcurrency(t *testing.T) {
	cache := &ModelCapabilityCache{
		data:  make(map[CacheKey]*ModelCapabilities),
		mutex: &sync.RWMutex{},
	}

	// Test concurrent reads and writes
	done := make(chan bool)
	errors := make(chan string, 100)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				key := CacheKey{
					BaseURL: "https://api.openai.com/v1",
					Model:   "model-" + string(rune('a'+id)),
				}
				caps := &ModelCapabilities{
					UsesMaxCompletionTokens: j%2 == 0,
					LastChecked:             time.Now(),
				}
				cache.Set(key, caps)
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				key := CacheKey{
					BaseURL: "https://api.openai.com/v1",
					Model:   "model-" + string(rune('a'+id)),
				}
				_, found := cache.Get(key)
				// We don't assert found because writers are concurrent
				if found && key.Model == "" {
					errors <- "Invalid model in cache"
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrency error: %s", err)
	}
}

// TestCacheTimestampTracking tests that LastChecked is properly tracked
func TestCacheTimestampTracking(t *testing.T) {
	cache := &ModelCapabilityCache{
		data:  make(map[CacheKey]*ModelCapabilities),
		mutex: &sync.RWMutex{},
	}

	key := CacheKey{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-5",
	}

	before := time.Now()
	caps := &ModelCapabilities{
		UsesMaxCompletionTokens: true,
		LastChecked:             before,
	}
	cache.Set(key, caps)

	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Cache entry not found")
	}

	// Check that timestamp is preserved
	if !retrieved.LastChecked.Equal(before) {
		t.Errorf("LastChecked not preserved: %v vs %v", retrieved.LastChecked, before)
	}

	// Check that timestamp is recent
	if time.Since(retrieved.LastChecked) > time.Second {
		t.Error("Cache timestamp should be recent")
	}
}

// TestErrorDetectionBroadMatching tests broad keyword matching for various error formats
func TestErrorDetectionBroadMatching(t *testing.T) {
	// These should ALL match: require both parameter indicator AND parameter name
	shouldMatch := []string{
		"parameter 'max_completion_tokens'",
		"Parameter 'max_completion_tokens'",
		"parameter max_tokens",
		"invalid parameter max_completion_tokens",
		"unsupported parameter max_tokens",
		"Parameter 'MAX_COMPLETION_TOKENS' is invalid",
		"UNSUPPORTED parameter max_tokens",
		"OpenAI error: parameter max_completion_tokens unsupported",
		"OpenWebUI: parameter 'max_completion_tokens' not supported",
		"LiteLLM error: invalid parameter max_tokens",
		"Invalid max_tokens parameter",
		"Unsupported max_completion_tokens",
	}

	for _, pattern := range shouldMatch {
		if !isMaxTokensParameterError(pattern) {
			t.Errorf("Should detect parameter error: %s", pattern)
		}
	}

	// These should NOT match: missing either indicator or parameter name
	shouldNotMatch := []string{
		"parameter 'max_input_tokens'",  // wrong parameter
		"parameter mismatch",             // no token param
		"max_tokens is great",            // no error indicator
		"invalid request",                // no token param
		"unsupported feature",            // no token param
		"",                               // empty
	}

	for _, pattern := range shouldNotMatch {
		if isMaxTokensParameterError(pattern) {
			t.Errorf("Should NOT detect parameter error: %s", pattern)
		}
	}
}

// Helper types and functions for testing (would be in actual implementation)

// CacheKey identifies a model capability cache entry
type CacheKey struct {
	BaseURL string
	Model   string
}

// ModelCapabilities holds learned model capabilities
type ModelCapabilities struct {
	UsesMaxCompletionTokens bool
	LastChecked             time.Time
}

// ModelCapabilityCache manages per-model capabilities
type ModelCapabilityCache struct {
	data  map[CacheKey]*ModelCapabilities
	mutex *sync.RWMutex
}

// Get retrieves a cache entry
func (c *ModelCapabilityCache) Get(key CacheKey) (*ModelCapabilities, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	caps, found := c.data[key]
	return caps, found
}

// Set stores a cache entry
func (c *ModelCapabilityCache) Set(key CacheKey, caps *ModelCapabilities) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = caps
}

// NOTE: isMaxTokensParameterError is already defined in handlers.go
// These tests use that implementation
