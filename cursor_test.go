package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapCursorUsage(t *testing.T) {
	u := mapCursorUsage(cursorUsage{
		InputTokens:      100,
		OutputTokens:     20,
		CacheReadTokens:  30,
		CacheWriteTokens: 40,
	})
	if u.InputTokens != 100 || u.OutputTokens != 20 {
		t.Fatalf("unexpected tokens: %+v", u)
	}
	if u.CacheReadInputTokens != 30 || u.CacheCreationInputTokens != 40 {
		t.Fatalf("unexpected cache fields: %+v", u)
	}
}

func TestCursorAgentResultSuccessClearsStopReason(t *testing.T) {
	const raw = `{"type":"result","subtype":"success","is_error":false,"result":" hi ","usage":{"inputTokens":1,"outputTokens":2,"cacheReadTokens":3,"cacheWriteTokens":4}}`
	var cr cursorAgentResult
	if err := json.Unmarshal([]byte(raw), &cr); err != nil {
		t.Fatal(err)
	}
	if cr.IsError {
		t.Fatal("expected success")
	}
	stop := cr.Subtype
	if stop == "success" {
		stop = ""
	}
	if stop != "" {
		t.Fatalf("expected empty stop after success, got %q", stop)
	}
	if strings.TrimSpace(cr.Result) != "hi" {
		t.Fatalf("result: %q", cr.Result)
	}
}
