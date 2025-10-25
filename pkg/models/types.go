package models

// ClaudeMessage represents a message in Claude API format
type ClaudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentBlock
}

// ContentBlock represents a content block in Claude format
type ContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	Thinking  string      `json:"thinking,omitempty"`  // For thinking blocks
	Signature string      `json:"signature,omitempty"` // Required for thinking blocks to be hidden
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
}

// ClaudeRequest represents the full Claude API request
type ClaudeRequest struct {
	Model         string          `json:"model"`
	Messages      []ClaudeMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        *bool           `json:"stream,omitempty"`
	System        interface{}     `json:"system,omitempty"` // Can be string OR array of content blocks
	Tools         []Tool          `json:"tools,omitempty"`
}

// Tool represents a function/tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role             string           `json:"role"`
	Content          interface{}      `json:"content,omitempty"` // string or null
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ReasoningDetails []interface{}    `json:"reasoning_details,omitempty"` // OpenRouter reasoning
}

// OpenAIToolCall represents a tool call in OpenAI format
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// OpenAIRequest represents the full OpenAI API request
type OpenAIRequest struct {
	Model               string                 `json:"model"`
	Messages            []OpenAIMessage        `json:"messages"`
	MaxTokens           int                    `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                    `json:"max_completion_tokens,omitempty"`
	Temperature         *float64               `json:"temperature,omitempty"`
	TopP                *float64               `json:"top_p,omitempty"`
	Stop                []string               `json:"stop,omitempty"`
	Stream              *bool                  `json:"stream,omitempty"`
	StreamOptions       map[string]interface{} `json:"stream_options,omitempty"`   // OpenAI standard
	Usage               map[string]interface{} `json:"usage,omitempty"`            // OpenRouter
	Reasoning           map[string]interface{} `json:"reasoning,omitempty"`        // OpenRouter reasoning tokens
	ReasoningEffort     string                 `json:"reasoning_effort,omitempty"` // OpenAI Chat Completions reasoning (GPT-5 models)
	Tools               []OpenAITool           `json:"tools,omitempty"`
	ToolChoice          interface{}            `json:"tool_choice,omitempty"` // Force tool usage: "auto", "required", or specific tool
}

// OpenAITool represents a tool in OpenAI format
type OpenAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

// ClaudeResponse represents the Claude API response
type ClaudeResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// OpenAIResponse represents the OpenAI API response
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

// OpenAIChoice represents a choice in the OpenAI response
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason *string       `json:"finish_reason"`
}

// OpenAIUsage represents token usage in OpenAI format
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
