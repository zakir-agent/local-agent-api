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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeCORS(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{Message: msg, Type: "error"},
	})
}

const maxRequestBody = 1 << 20 // 1MB

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

	log.Printf("request: %d messages, last: %.50s...", len(req.Messages), req.Messages[len(req.Messages)-1].Content)

	// Serialize access — no concurrent CLI calls
	mu.Lock()
	defer mu.Unlock()

	start := time.Now()
	result, err := callClaude(r.Context(), req.Messages)
	elapsed := time.Since(start)

	if err != nil {
		errMsg := err.Error()
		log.Printf("error (%s): %s", elapsed, errMsg)
		logRequest(req.Messages, "", errMsg, elapsed)
		switch {
		case strings.Contains(errMsg, "timeout"):
			writeError(w, http.StatusGatewayTimeout, "claude cli timeout")
		case strings.Contains(errMsg, "canceled"):
			// Client disconnected, no point writing response
			log.Printf("request canceled by client")
		case strings.Contains(errMsg, "executable file not found"):
			writeError(w, http.StatusInternalServerError, "claude cli not found")
		default:
			writeError(w, http.StatusBadGateway, fmt.Sprintf("claude cli failed: %s", errMsg))
		}
		return
	}

	log.Printf("done (%s), response length: %d", elapsed, len(result))
	logRequest(req.Messages, result, "", elapsed)

	resp := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "claude-" + model,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: result},
				FinishReason: "stop",
			},
		},
	}

	writeCORS(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeCORS(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":       "claude-" + model,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "anthropic",
			},
		},
	})
}
