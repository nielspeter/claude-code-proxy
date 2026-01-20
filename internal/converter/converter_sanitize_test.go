package converter

import (
	"reflect"
	"testing"

	"github.com/claude-code-proxy/proxy/pkg/models"
)

func TestConvertResponse_Sanitization(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		argsJSON       string
		expectedInput  map[string]interface{}
	}{
		{
			name:     "Edit with query (should be removed)",
			toolName: "Edit",
			argsJSON: `{"file_path":"test.txt", "old_string":"foo", "new_string":"bar", "query":"fix bug"}`,
			expectedInput: map[string]interface{}{
				"file_path":  "test.txt",
				"old_string": "foo",
				"new_string": "bar",
			},
		},
		{
			name:     "ReadFile with only query (mapped to file_path)",
			toolName: "ReadFile",
			argsJSON: `{"query":"path/to/file.txt"}`,
			expectedInput: map[string]interface{}{
				"file_path": "path/to/file.txt",
			},
		},
		{
			name:     "ReadFile with file_path and query (query removed)",
			toolName: "ReadFile",
			argsJSON: `{"file_path":"real.txt", "query":"path/to/file.txt"}`,
			expectedInput: map[string]interface{}{
				"file_path": "real.txt",
			},
		},
		{
			name:     "Grep with query mapped to pattern",
			toolName: "Grep",
			argsJSON: `{"path":"src", "query":"TODO"}`,
			expectedInput: map[string]interface{}{
				"path":    "src",
				"pattern": "TODO",
			},
		},
		{
			name:     "Bash with query mapped to command",
			toolName: "Bash",
			argsJSON: `{"query":"ls -la"}`,
			expectedInput: map[string]interface{}{
				"command": "ls -la",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finishReason := "tool_calls"
			openaiResp := &models.OpenAIResponse{
				Choices: []models.OpenAIChoice{
					{
						Message: models.OpenAIMessage{
							ToolCalls: []models.OpenAIToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: struct {
										Name      string `json:"name"`
										Arguments string `json:"arguments"`
									}{
										Name:      tt.toolName,
										Arguments: tt.argsJSON,
									},
								},
							},
						},
						FinishReason: &finishReason,
					},
				},
			}

			claudeResp, err := ConvertResponse(openaiResp, "model")
			if err != nil {
				t.Fatalf("ConvertResponse failed: %v", err)
			}

			if len(claudeResp.Content) != 1 {
				t.Fatalf("Expected 1 content block, got %d", len(claudeResp.Content))
			}
			
			toolUse := claudeResp.Content[0]
			if toolUse.Type != "tool_use" {
				t.Fatalf("Expected tool_use, got %s", toolUse.Type)
			}

			// The Input should be a map now
			inputMap, ok := toolUse.Input.(map[string]interface{})
			if !ok {
				t.Fatalf("Input is not a map, got %T: %v", toolUse.Input, toolUse.Input)
			}

			if !reflect.DeepEqual(inputMap, tt.expectedInput) {
				t.Errorf("Input mismatch.\nGot:  %v\nWant: %v", inputMap, tt.expectedInput)
			}
		})
	}
}
