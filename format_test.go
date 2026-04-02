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

// TestResponsesResponseFormat verifies response fields match the OpenAI Responses API shape.
//
// Response: https://github.com/openai/openai-python/blob/main/src/openai/types/responses/response.py
func TestResponsesResponseFormat(t *testing.T) {
	cleanup := mockCLIOutput()
	defer cleanup()

	body := `{"model":"gpt-4o-mini","input":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleResponses(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	m := parseJSON(t, w.Body.Bytes())

	assertKeys(t, "root", m, []string{
		"id", "object", "created_at", "model", "output", "status",
		"completed_at", "usage", "parallel_tool_calls", "tool_choice", "tools",
	})

	if m["object"] != "response" {
		t.Errorf("object: expected 'response', got %v", m["object"])
	}
	if m["status"] != "completed" {
		t.Errorf("status: expected 'completed', got %v", m["status"])
	}
	if m["model"] != "gpt-4o-mini" {
		t.Errorf("model: expected echo of request model, got %v", m["model"])
	}

	out := m["output"].([]any)
	if len(out) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(out))
	}
	item := out[0].(map[string]any)
	assertKeys(t, "output[0]", item, []string{
		"id", "type", "role", "status", "content",
	})
	if item["type"] != "message" {
		t.Errorf("output[0].type: expected 'message', got %v", item["type"])
	}
	if item["role"] != "assistant" {
		t.Errorf("output[0].role: expected 'assistant', got %v", item["role"])
	}
	if item["status"] != "completed" {
		t.Errorf("output[0].status: expected 'completed', got %v", item["status"])
	}

	blocks := item["content"].([]any)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(blocks))
	}
	blk := blocks[0].(map[string]any)
	assertKeys(t, "output[0].content[0]", blk, []string{
		"type", "text", "annotations",
	})
	if blk["type"] != "output_text" {
		t.Errorf("content[0].type: expected 'output_text', got %v", blk["type"])
	}
	if blk["text"] != "hi" {
		t.Errorf("content[0].text: expected CLI result 'hi', got %v", blk["text"])
	}

	usage := m["usage"].(map[string]any)
	assertKeys(t, "usage", usage, []string{
		"input_tokens", "input_tokens_details", "output_tokens",
		"output_tokens_details", "total_tokens",
	})
	if usage["input_tokens"].(float64) != 160 {
		t.Errorf("usage.input_tokens: expected 160, got %v", usage["input_tokens"])
	}
	if usage["output_tokens"].(float64) != 5 {
		t.Errorf("usage.output_tokens: expected 5, got %v", usage["output_tokens"])
	}
	if usage["total_tokens"].(float64) != 165 {
		t.Errorf("usage.total_tokens: expected 165, got %v", usage["total_tokens"])
	}

	inDet := usage["input_tokens_details"].(map[string]any)
	assertKeys(t, "usage.input_tokens_details", inDet, []string{"cached_tokens"})
	if inDet["cached_tokens"].(float64) != 50 {
		t.Errorf("usage.input_tokens_details.cached_tokens: expected 50, got %v", inDet["cached_tokens"])
	}

	outDet := usage["output_tokens_details"].(map[string]any)
	assertKeys(t, "usage.output_tokens_details", outDet, []string{"reasoning_tokens"})
	if outDet["reasoning_tokens"].(float64) != 0 {
		t.Errorf("usage.output_tokens_details.reasoning_tokens: expected 0, got %v", outDet["reasoning_tokens"])
	}

	parallel, ok := m["parallel_tool_calls"].(bool)
	if !ok || parallel {
		t.Errorf("parallel_tool_calls: expected false, got %v", m["parallel_tool_calls"])
	}
	if m["tool_choice"] != "auto" {
		t.Errorf("tool_choice: expected 'auto', got %v", m["tool_choice"])
	}
	tools, ok := m["tools"].([]any)
	if !ok || len(tools) != 0 {
		t.Errorf("tools: expected empty array, got %v", m["tools"])
	}
}

// TestResponsesResponseFormatIncomplete verifies incomplete status and incomplete_details.
func TestResponsesResponseFormatIncomplete(t *testing.T) {
	orig := callClaude
	callClaude = func(ctx context.Context, messages []ChatMessage) (*cliOutput, error) {
		return &cliOutput{
			Result:     "trunc",
			StopReason: "max_tokens",
			Usage: cliUsage{
				InputTokens:              10,
				OutputTokens:             5,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
			},
		}, nil
	}
	defer func() { callClaude = orig }()

	body := `{"input":"run until limit"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleResponses(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	m := parseJSON(t, w.Body.Bytes())

	assertKeys(t, "root", m, []string{
		"id", "object", "created_at", "model", "output", "status",
		"completed_at", "incomplete_details", "usage",
		"parallel_tool_calls", "tool_choice", "tools",
	})

	if m["status"] != "incomplete" {
		t.Errorf("status: expected 'incomplete', got %v", m["status"])
	}

	inc, ok := m["incomplete_details"].(map[string]any)
	if !ok {
		t.Fatalf("incomplete_details: expected object, got %T", m["incomplete_details"])
	}
	assertKeys(t, "incomplete_details", inc, []string{"reason"})
	if inc["reason"] != "max_output_tokens" {
		t.Errorf("incomplete_details.reason: expected 'max_output_tokens', got %v", inc["reason"])
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
