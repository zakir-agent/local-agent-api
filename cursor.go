package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type cursorUsage struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	CacheReadTokens  int `json:"cacheReadTokens"`
	CacheWriteTokens int `json:"cacheWriteTokens"`
}

type cursorAgentResult struct {
	Type    string      `json:"type"`
	Subtype string      `json:"subtype"`
	IsError bool        `json:"is_error"`
	Result  string      `json:"result"`
	Usage   cursorUsage `json:"usage"`
}

func mapCursorUsage(u cursorUsage) cliUsage {
	return cliUsage{
		InputTokens:              u.InputTokens,
		CacheCreationInputTokens: u.CacheWriteTokens,
		CacheReadInputTokens:     u.CacheReadTokens,
		OutputTokens:             u.OutputTokens,
	}
}

func callCursorAgent(ctx context.Context, messages []ChatMessage) (*cliOutput, error) {
	prompt := formatMessages(messages)

	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()

	args := []string{
		"--print", "--output-format", "json",
		"--trust",
		"--mode", "ask",
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, "cursor-agent", args...)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout")
		}
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("canceled")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("cli error: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("exec error: %w", err)
	}

	var cr cursorAgentResult
	if err := json.Unmarshal(out, &cr); err != nil {
		return &cliOutput{Result: strings.TrimSpace(string(out))}, nil
	}
	if cr.IsError {
		return nil, fmt.Errorf("cli error: %s", strings.TrimSpace(cr.Result))
	}
	if cr.Type != "" && cr.Type != "result" {
		return nil, fmt.Errorf("cli error: unexpected response type %q", cr.Type)
	}

	stop := cr.Subtype
	if stop == "success" {
		stop = ""
	}

	return &cliOutput{
		Result:     strings.TrimSpace(cr.Result),
		StopReason: stop,
		Usage:      mapCursorUsage(cr.Usage),
	}, nil
}
