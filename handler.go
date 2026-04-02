package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Tool / call items in Responses API input lists; not supported for local CLI.
var responsesSkippedInputTypes = map[string]struct{}{
	"function_call":         {},
	"function_call_output":  {},
	"web_search_call":       {},
	"file_search_call":      {},
	"computer_call":         {},
	"computer_call_output":  {},
	"code_interpreter_call": {},
	"reasoning":             {},
	"mcp_list_tools":        {},
	"mcp_approval_request":  {},
}

var mu sync.Mutex

func writeCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key, anthropic-version")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	writeCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: encode: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{Message: msg, Type: "error"},
	})
}

func writeAnthropicError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, AnthropicErrorResponse{
		Type:  "error",
		Error: ErrorDetail{Message: msg, Type: "invalid_request_error"},
	})
}

const maxRequestBody = 1 << 20 // 1MB

// callWithCLI handles the common flow: log, lock, call CLI, log result, handle errors.
// Returns (result, true) on success or ("", false) on error (error response already written).
func callWithCLI(w http.ResponseWriter, r *http.Request, messages []ChatMessage, errWriter func(http.ResponseWriter, int, string)) (*cliOutput, bool) {
	log.Printf("request: %d messages, last: %.50s...", len(messages), messages[len(messages)-1].Content)

	mu.Lock()
	defer mu.Unlock()

	start := time.Now()
	out, err := callClaude(r.Context(), messages)
	elapsed := time.Since(start)

	if err != nil {
		errMsg := err.Error()
		log.Printf("error (%s): %s", elapsed, errMsg)
		logRequest(messages, "", errMsg, elapsed)
		switch {
		case strings.Contains(errMsg, "timeout"):
			errWriter(w, http.StatusGatewayTimeout, "CLI timeout")
		case strings.Contains(errMsg, "canceled"):
			log.Printf("request canceled by client")
		case strings.Contains(errMsg, "executable file not found"):
			errWriter(w, http.StatusInternalServerError, "CLI not found")
		default:
			errWriter(w, http.StatusBadGateway, fmt.Sprintf("CLI failed: %s", errMsg))
		}
		return nil, false
	}

	log.Printf("done (%s), response length: %d", elapsed, len(out.Result))
	logRequest(messages, out.Result, "", elapsed)
	return out, true
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages is required and must not be empty")
		return
	}

	out, ok := callWithCLI(w, r, req.Messages, writeError)
	if !ok {
		return
	}

	promptTokens := out.Usage.InputTokens + out.Usage.CacheCreationInputTokens + out.Usage.CacheReadInputTokens
	totalTokens := promptTokens + out.Usage.OutputTokens

	finishReason := "stop"
	if out.StopReason == "max_tokens" {
		finishReason = "length"
	}

	writeJSON(w, http.StatusOK, ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Message:      ResponseMessage{Role: "assistant", Content: out.Result},
				FinishReason: finishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: out.Usage.OutputTokens,
			TotalTokens:      totalTokens,
		},
	})
}

func mergeResponsesInstructions(msgs []ChatMessage, instr json.RawMessage) []ChatMessage {
	if len(instr) == 0 || string(instr) == "null" {
		return msgs
	}
	var s string
	if err := json.Unmarshal(instr, &s); err == nil && s != "" {
		return append([]ChatMessage{{Role: "system", Content: s}}, msgs...)
	}
	return msgs
}

func parseResponsesInputContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", fmt.Errorf("unsupported message content shape")
	}
	var b strings.Builder
	for _, p := range blocks {
		switch p.Type {
		case "input_text", "":
			b.WriteString(p.Text)
		case "input_image", "input_file":
			return "", fmt.Errorf("multimodal Responses input is not supported")
		default:
			return "", fmt.Errorf("unsupported content type %q", p.Type)
		}
	}
	return b.String(), nil
}

func parseResponsesInputItem(item json.RawMessage) ([]ChatMessage, error) {
	var head struct {
		Type string `json:"type"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(item, &head); err != nil {
		return nil, err
	}
	if head.Type != "" {
		if _, skip := responsesSkippedInputTypes[head.Type]; skip {
			return nil, nil
		}
		if head.Type != "message" && head.Role == "" {
			return nil, nil
		}
	}
	var m struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(item, &m); err != nil {
		return nil, err
	}
	if m.Role == "" {
		return nil, nil
	}
	text, err := parseResponsesInputContent(m.Content)
	if err != nil {
		return nil, err
	}
	return []ChatMessage{{Role: m.Role, Content: text}}, nil
}

func parseResponsesInput(raw json.RawMessage) ([]ChatMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("input is required")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("input must not be empty")
		}
		return []ChatMessage{{Role: "user", Content: s}}, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("input must be a string or non-empty array")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("input must not be empty")
	}
	var msgs []ChatMessage
	for _, it := range items {
		part, err := parseResponsesInputItem(it)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, part...)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no usable messages in input (tools/multimodal items are not supported)")
	}
	return msgs, nil
}

func handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req ResponsesCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	msgs, err := parseResponsesInput(req.Input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	msgs = mergeResponsesInstructions(msgs, req.Instructions)
	if len(msgs) == 0 {
		writeError(w, http.StatusBadRequest, "messages is required and must not be empty")
		return
	}

	out, ok := callWithCLI(w, r, msgs, writeError)
	if !ok {
		return
	}

	promptTokens := out.Usage.InputTokens + out.Usage.CacheCreationInputTokens + out.Usage.CacheReadInputTokens
	totalTokens := promptTokens + out.Usage.OutputTokens
	now := float64(time.Now().Unix())

	status := "completed"
	var incomplete *struct {
		Reason string `json:"reason"`
	}
	if out.StopReason == "max_tokens" {
		status = "incomplete"
		incomplete = &struct {
			Reason string `json:"reason"`
		}{Reason: "max_output_tokens"}
	}

	usage := ResponsesUsage{
		InputTokens:  promptTokens,
		OutputTokens: out.Usage.OutputTokens,
		TotalTokens:  totalTokens,
	}
	usage.InputTokensDetails.CachedTokens = out.Usage.CacheReadInputTokens
	usage.OutputTokensDetails.ReasoningTokens = 0

	respModel := model
	if req.Model != "" {
		respModel = req.Model
	}

	writeJSON(w, http.StatusOK, ResponsesCreateResponse{
		ID:        fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		Object:    "response",
		CreatedAt: now,
		Model:     respModel,
		Output: []ResponsesOutputMessageItem{
			{
				ID:     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []ResponseOutputTextBlock{
					{
						Type:        "output_text",
						Text:        out.Result,
						Annotations: []any{},
					},
				},
			},
		},
		Status:            status,
		CompletedAt:       now,
		IncompleteDetails: incomplete,
		Usage:             usage,
		ParallelToolCalls: false,
		ToolChoice:        "auto",
		Tools:             []any{},
	})
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeAnthropicError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req MessagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		writeAnthropicError(w, http.StatusBadRequest, "messages is required and must not be empty")
		return
	}

	out, ok := callWithCLI(w, r, req.Messages, writeAnthropicError)
	if !ok {
		return
	}

	stopReason := out.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}

	writeJSON(w, http.StatusOK, MessagesResponse{
		ID:   fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Type: "message",
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: out.Result},
		},
		Model:        model,
		StopReason:   &stopReason,
		StopSequence: nil,
		Usage: MessagesUsage{
			InputTokens:              out.Usage.InputTokens,
			OutputTokens:             out.Usage.OutputTokens,
			CacheCreationInputTokens: &out.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     &out.Usage.CacheReadInputTokens,
		},
	})
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ownedBy := "anthropic"
	if cliBackend == "cursor" {
		ownedBy = "cursor"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       model,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": ownedBy,
			},
		},
	})
}
