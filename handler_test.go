package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupMock(response string, err error) func() {
	orig := callClaude
	callClaude = func(ctx context.Context, messages []ChatMessage) (*cliOutput, error) {
		if err != nil {
			return nil, err
		}
		return &cliOutput{
			Result:     response,
			StopReason: "end_turn",
			Usage:      cliUsage{InputTokens: 10, OutputTokens: 5},
		}, nil
	}
	return func() { callClaude = orig }
}

func TestChatCompletions(t *testing.T) {
	cleanup := setupMock("Hello!", nil)
	defer cleanup()

	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %q", resp.Choices[0].FinishReason)
	}
	if resp.Object != "chat.completion" {
		t.Errorf("expected object 'chat.completion', got %q", resp.Object)
	}
}

func TestChatCompletionsEmptyMessages(t *testing.T) {
	body := `{"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChatCompletionsInvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChatCompletionsMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handleChatCompletions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestChatCompletionsCORS(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handleChatCompletions(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestResponsesStringInput(t *testing.T) {
	cleanup := setupMock("Joke!", nil)
	defer cleanup()

	body := `{"model":"gpt-4o-mini","input":"tell me a joke"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleResponses(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ResponsesCreateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Object != "response" {
		t.Errorf("expected object 'response', got %q", resp.Object)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status completed, got %q", resp.Status)
	}
	if len(resp.Output) != 1 || resp.Output[0].Type != "message" {
		t.Fatalf("expected one message output, got %+v", resp.Output)
	}
	if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Text != "Joke!" {
		t.Errorf("unexpected output text: %+v", resp.Output[0].Content)
	}
	if resp.Output[0].Content[0].Type != "output_text" {
		t.Errorf("expected output_text, got %q", resp.Output[0].Content[0].Type)
	}
	if resp.Usage.TotalTokens != resp.Usage.InputTokens+resp.Usage.OutputTokens {
		t.Errorf("usage total mismatch: %+v", resp.Usage)
	}
}

func TestResponsesInputMessageList(t *testing.T) {
	cleanup := setupMock("Hi there", nil)
	defer cleanup()

	body := `{"input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleResponses(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp ResponsesCreateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Output[0].Content[0].Text != "Hi there" {
		t.Errorf("expected Hi there, got %q", resp.Output[0].Content[0].Text)
	}
}

func TestResponsesInstructionsPrependsSystem(t *testing.T) {
	var got []ChatMessage
	orig := callClaude
	callClaude = func(ctx context.Context, messages []ChatMessage) (*cliOutput, error) {
		got = messages
		return &cliOutput{
			Result:     "x",
			StopReason: "end_turn",
			Usage: cliUsage{
				InputTokens:  1,
				OutputTokens: 1,
			},
		}, nil
	}
	defer func() { callClaude = orig }()

	body := `{"input":[{"role":"user","content":"hi"}],"instructions":"Be brief."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleResponses(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(got) != 2 || got[0].Role != "system" || got[0].Content != "Be brief." {
		t.Fatalf("expected system instruction first: %+v", got)
	}
}

func TestResponsesEmptyInput(t *testing.T) {
	body := `{"input":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	w := httptest.NewRecorder()

	handleResponses(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResponsesMultimodalRejected(t *testing.T) {
	body := `{"input":[{"role":"user","content":[{"type":"input_image","image_url":"http://x"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	w := httptest.NewRecorder()

	handleResponses(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResponsesCORS(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/responses", nil)
	w := httptest.NewRecorder()

	handleResponses(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestChatCompletionsCLIError(t *testing.T) {
	cleanup := setupMock("", fmt.Errorf("cli error: something went wrong"))
	defer cleanup()

	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleChatCompletions(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestModels(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	handleModels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["object"] != "list" {
		t.Errorf("expected object 'list', got %v", resp["object"])
	}
	data := resp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected 1 model, got %d", len(data))
	}
}

func TestMessages(t *testing.T) {
	cleanup := setupMock("Hello from Claude!", nil)
	defer cleanup()

	body := `{"model":"claude-sonnet","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp MessagesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Type != "message" {
		t.Errorf("expected type 'message', got %q", resp.Type)
	}
	if resp.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", resp.Role)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "Hello from Claude!" {
		t.Errorf("unexpected content: %+v", resp.Content)
	}
	if resp.StopReason == nil || *resp.StopReason != "end_turn" {
		t.Errorf("expected stop_reason 'end_turn', got %v", resp.StopReason)
	}
}

func TestMessagesEmptyMessages(t *testing.T) {
	body := `{"model":"claude-sonnet","max_tokens":1024,"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	w := httptest.NewRecorder()

	handleMessages(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp AnthropicErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Type != "error" {
		t.Errorf("expected type 'error', got %q", resp.Type)
	}
}

func TestMessagesCORS(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/messages", nil)
	w := httptest.NewRecorder()

	handleMessages(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestFormatMessages(t *testing.T) {
	messages := []ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
	}
	result := formatMessages(messages)
	expected := "[system] You are helpful.\n\n[user] Hello"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
