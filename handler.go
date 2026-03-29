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
	json.NewEncoder(w).Encode(v)
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
			errWriter(w, http.StatusGatewayTimeout, "claude cli timeout")
		case strings.Contains(errMsg, "canceled"):
			log.Printf("request canceled by client")
		case strings.Contains(errMsg, "executable file not found"):
			errWriter(w, http.StatusInternalServerError, "claude cli not found")
		default:
			errWriter(w, http.StatusBadGateway, fmt.Sprintf("claude cli failed: %s", errMsg))
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
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       model,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "anthropic",
			},
		},
	})
}
