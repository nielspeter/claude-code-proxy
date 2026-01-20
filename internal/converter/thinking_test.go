package converter

import (
	"encoding/json"
	"testing"

	"github.com/claude-code-proxy/proxy/pkg/models"
)

// TestThinkingBlockSignature verifies that thinking blocks include signature field
func TestThinkingBlockSignature(t *testing.T) {
	// Test 1: OpenRouter reasoning_details format
	openaiResp := &models.OpenAIResponse{
		ID: "test-1",
		Choices: []models.OpenAIChoice{
			{
				Message: models.OpenAIMessage{
					Content: "Answer",
					ReasoningDetails: []interface{}{
						map[string]interface{}{
							"type": "reasoning.text",
							"text": "Let me think about this...",
						},
					},
				},
			},
		},
		Usage: models.OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
		},
	}

	claudeResp, err := ConvertResponse(openaiResp, "claude-opus-4")
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	// Verify thinking block has signature
	if len(claudeResp.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks, got %d", len(claudeResp.Content))
	}

	thinkingBlock := claudeResp.Content[0]
	if thinkingBlock.Type != "thinking" {
		t.Errorf("Expected first block to be thinking, got %s", thinkingBlock.Type)
	}

	// Critical: Check signature field exists
	// Marshal to verify field is present in JSON output
	jsonBytes, _ := json.Marshal(thinkingBlock)
	var check map[string]interface{}
	json.Unmarshal(jsonBytes, &check)
	if _, hasSignature := check["signature"]; !hasSignature {
		t.Error("Thinking block missing 'signature' field - required for Claude Code")
	}

	t.Logf("✓ Thinking block has signature field")
}

// TestNativeClaudeThinkingFormat tests provider returning native Claude format
func TestNativeClaudeThinkingFormat(t *testing.T) {
	// Some providers may return Claude-style content array directly
	openaiResp := &models.OpenAIResponse{
		ID: "test-2",
		Choices: []models.OpenAIChoice{
			{
				Message: models.OpenAIMessage{
					// Content as array (native Claude format)
					Content: []interface{}{
						map[string]interface{}{
							"type":     "thinking",
							"thinking": "Analyzing the problem...",
						},
						map[string]interface{}{
							"type": "text",
							"text": "The answer is 42",
						},
					},
				},
			},
		},
		Usage: models.OpenAIUsage{
			PromptTokens:     5,
			CompletionTokens: 10,
		},
	}

	claudeResp, err := ConvertResponse(openaiResp, "claude-opus-4.5-thinking")
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	if len(claudeResp.Content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(claudeResp.Content))
	}

	// Check thinking block
	thinkingBlock := claudeResp.Content[0]
	if thinkingBlock.Type != "thinking" {
		t.Errorf("Expected thinking block, got %s", thinkingBlock.Type)
	}
	if thinkingBlock.Thinking != "Analyzing the problem..." {
		t.Errorf("Wrong thinking text: %s", thinkingBlock.Thinking)
	}

	// Verify signature exists
	jsonBytes, _ := json.Marshal(thinkingBlock)
	var check map[string]interface{}
	json.Unmarshal(jsonBytes, &check)
	if _, hasSignature := check["signature"]; !hasSignature {
		t.Error("Native Claude thinking block missing 'signature' field")
	}

	// Check text block
	textBlock := claudeResp.Content[1]
	if textBlock.Type != "text" {
		t.Errorf("Expected text block, got %s", textBlock.Type)
	}
	if textBlock.Text != "The answer is 42" {
		t.Errorf("Wrong text: %s", textBlock.Text)
	}

	t.Logf("✓ Native Claude format handled correctly")
}
