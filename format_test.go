package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// assertKeys checks that all expected keys exist in the map and no unexpected keys are present.
func assertKeys(t *testing.T, path string, m map[string]any, expected []string) {
	t.Helper()
	set := make(map[string]bool, len(expected))
	for _, k := range expected {
		set[k] = true
	}
	for _, k := range expected {
		if _, ok := m[k]; !ok {
			t.Errorf("%s: missing required field %q", path, k)
		}
	}
	for k := range m {
		if !set[k] {
			t.Errorf("%s: unexpected field %q", path, k)
		}
	}
}

func parseJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	return m
}

func mockCLIOutput() func() {
	orig := callClaude
	callClaude = func(ctx context.Context, messages []ChatMessage) (*cliOutput, error) {
		return &cliOutput{
			Result:     "hi",
			StopReason: "end_turn",
			Usage:      cliUsage{InputTokens: 10, OutputTokens: 5, CacheCreationInputTokens: 100, CacheReadInputTokens: 50},
		}, nil
	}
	return func() { callClaude = orig }
}

// TestChatCompletionsResponseFormat verifies response fields match the openai-python SDK.
//
// ChatCompletion: https://github.com/openai/openai-python/blob/main/src/openai/types/chat/chat_completion.py
// Choice:         https://github.com/openai/openai-python/blob/main/src/openai/types/chat/chat_completion.py
// Message:        https://github.com/openai/openai-python/blob/main/src/openai/types/chat/chat_completion_message.py
// CompletionUsage: https://github.com/openai/openai-python/blob/main/src/openai/types/completion_usage.py
func TestChatCompletionsResponseFormat(t *testing.T) {
	cleanup := mockCLIOutput()
	defer cleanup()

	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleChatCompletions(w, req)

	m := parseJSON(t, w.Body.Bytes())

	// ChatCompletion required fields: id, choices, created, model, object
	// Optional fields we include: usage, system_fingerprint, service_tier
	assertKeys(t, "root", m, []string{
		"id", "object", "created", "model", "choices", "usage", "system_fingerprint", "service_tier",
	})

	if m["object"] != "chat.completion" {
		t.Errorf("object: expected 'chat.completion', got %v", m["object"])
	}

	// Choice fields: finish_reason, index, message, logprobs
	choices := m["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(choices))
	}
	choice := choices[0].(map[string]any)
	assertKeys(t, "choices[0]", choice, []string{
		"index", "message", "finish_reason", "logprobs",
	})

	// ChatCompletionMessage fields: role, content, refusal, tool_calls
	// (annotations, audio, function_call are omitted — not relevant for our proxy)
	msg := choice["message"].(map[string]any)
	assertKeys(t, "choices[0].message", msg, []string{
		"role", "content", "refusal", "tool_calls",
	})

	// CompletionUsage required fields: prompt_tokens, completion_tokens, total_tokens
	usage := m["usage"].(map[string]any)
	assertKeys(t, "usage", usage, []string{
		"prompt_tokens", "completion_tokens", "total_tokens",
	})

	if usage["prompt_tokens"].(float64) == 0 {
		t.Error("usage.prompt_tokens should not be 0")
	}
	if usage["completion_tokens"].(float64) == 0 {
		t.Error("usage.completion_tokens should not be 0")
	}
}

// TestMessagesResponseFormat verifies response fields match the anthropic-sdk-python.
//
// Message:    https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/message.py
// TextBlock:  https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/text_block.py
// Usage:      https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/types/usage.py
func TestMessagesResponseFormat(t *testing.T) {
	cleanup := mockCLIOutput()
	defer cleanup()

	body := `{"model":"claude-sonnet","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleMessages(w, req)

	m := parseJSON(t, w.Body.Bytes())

	// Message fields: id, type, role, content, model, stop_reason, stop_sequence, usage
	// (container is omitted — not relevant for our proxy)
	assertKeys(t, "root", m, []string{
		"id", "type", "role", "content", "model", "stop_reason", "stop_sequence", "usage",
	})

	if m["type"] != "message" {
		t.Errorf("type: expected 'message', got %v", m["type"])
	}
	if m["role"] != "assistant" {
		t.Errorf("role: expected 'assistant', got %v", m["role"])
	}

	// TextBlock fields: type, text, citations
	content := m["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block := content[0].(map[string]any)
	assertKeys(t, "content[0]", block, []string{
		"type", "text", "citations",
	})
	if block["type"] != "text" {
		t.Errorf("content[0].type: expected 'text', got %v", block["type"])
	}

	// Usage required fields: input_tokens, output_tokens
	// Optional fields we include: cache_creation_input_tokens, cache_read_input_tokens
	usage := m["usage"].(map[string]any)
	assertKeys(t, "usage", usage, []string{
		"input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens",
	})

	if usage["input_tokens"].(float64) != 10 {
		t.Errorf("usage.input_tokens: expected 10, got %v", usage["input_tokens"])
	}
	if usage["cache_creation_input_tokens"].(float64) != 100 {
		t.Errorf("usage.cache_creation_input_tokens: expected 100, got %v", usage["cache_creation_input_tokens"])
	}
	if usage["cache_read_input_tokens"].(float64) != 50 {
		t.Errorf("usage.cache_read_input_tokens: expected 50, got %v", usage["cache_read_input_tokens"])
	}
}

// TestModelsResponseFormat verifies response fields match the OpenAI models list format.
// Ref: https://github.com/openai/openai-python/blob/main/src/openai/types/model.py
func TestModelsResponseFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	handleModels(w, req)

	m := parseJSON(t, w.Body.Bytes())

	assertKeys(t, "root", m, []string{
		"object", "data",
	})

	if m["object"] != "list" {
		t.Errorf("object: expected 'list', got %v", m["object"])
	}

	// Model fields: id, object, created, owned_by
	data := m["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(data))
	}
	modelObj := data[0].(map[string]any)
	assertKeys(t, "data[0]", modelObj, []string{
		"id", "object", "created", "owned_by",
	})

	if modelObj["object"] != "model" {
		t.Errorf("data[0].object: expected 'model', got %v", modelObj["object"])
	}
}
