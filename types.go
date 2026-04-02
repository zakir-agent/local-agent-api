package main

import "encoding/json"

// OpenAI-compatible request/response types
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/chat/

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Messages []ChatMessage `json:"messages"`
	Model    string        `json:"model,omitempty"`
}

// ResponseMessage matches ChatCompletionMessage from openai-python SDK.
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/chat/chat_completion_message.py
type ResponseMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Refusal   any    `json:"refusal"`
	ToolCalls any    `json:"tool_calls"`
}

// ChatCompletionChoice matches Choice from openai-python SDK.
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/chat/chat_completion.py
type ChatCompletionChoice struct {
	Index        int             `json:"index"`
	Message      ResponseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     any             `json:"logprobs"`
}

// Usage matches CompletionUsage from openai-python SDK.
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/completion_usage.py
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse matches ChatCompletion from openai-python SDK.
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/chat/chat_completion.py
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             Usage                  `json:"usage"`
	SystemFingerprint *string                `json:"system_fingerprint"`
	ServiceTier       string                 `json:"service_tier"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// OpenAI Responses API (POST /v1/responses)
// Ref: https://platform.openai.com/docs/api-reference/responses/create
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/responses/response.py

type ResponsesCreateRequest struct {
	Model        string          `json:"model,omitempty"`
	Input        json.RawMessage `json:"input"`
	Instructions json.RawMessage `json:"instructions,omitempty"`
}

type ResponseOutputTextBlock struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	Annotations []any  `json:"annotations"`
	Logprobs    any    `json:"logprobs,omitempty"`
}

type ResponsesOutputMessageItem struct {
	ID      string                    `json:"id"`
	Type    string                    `json:"type"`
	Role    string                    `json:"role"`
	Status  string                    `json:"status,omitempty"`
	Content []ResponseOutputTextBlock `json:"content"`
}

type ResponsesUsage struct {
	InputTokens        int `json:"input_tokens"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens        int `json:"output_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int `json:"total_tokens"`
}

type ResponsesCreateResponse struct {
	ID                string                       `json:"id"`
	Object            string                       `json:"object"`
	CreatedAt         float64                      `json:"created_at"`
	Model             string                       `json:"model"`
	Output            []ResponsesOutputMessageItem `json:"output"`
	Status            string                       `json:"status"`
	CompletedAt       float64                      `json:"completed_at,omitempty"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details,omitempty"`
	Usage             ResponsesUsage `json:"usage"`
	ParallelToolCalls bool           `json:"parallel_tool_calls"`
	ToolChoice        string         `json:"tool_choice"`
	Tools             []any          `json:"tools"`
}

// Anthropic Messages API types
// Ref: https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/

// MessagesRequest matches MessageCreateParams.
type MessagesRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []ChatMessage `json:"messages"`
}

// ContentBlock matches TextBlock from anthropic-sdk-python.
// Ref: https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/text_block.py
type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Citations any    `json:"citations"`
}

// MessagesUsage matches Usage from anthropic-sdk-python.
// Ref: https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/usage.py
type MessagesUsage struct {
	InputTokens              int  `json:"input_tokens"`
	OutputTokens             int  `json:"output_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens"`
}

// MessagesResponse matches Message from anthropic-sdk-python.
// Ref: https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/message.py
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        MessagesUsage  `json:"usage"`
}

type AnthropicErrorResponse struct {
	Type  string      `json:"type"`
	Error ErrorDetail `json:"error"`
}
