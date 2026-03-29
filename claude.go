package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const cliTimeout = 120 * time.Second

type claudeResult struct {
	Result string `json:"result"`
}

func formatMessages(messages []ChatMessage) string {
	var b strings.Builder
	for i, m := range messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "[%s] %s", m.Role, m.Content)
	}
	return b.String()
}

func callClaude(ctx context.Context, messages []ChatMessage) (string, error) {
	prompt := formatMessages(messages)

	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", model, "--output-format", "json")
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout")
		}
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("canceled")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("cli error: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("exec error: %w", err)
	}

	var cr claudeResult
	if err := json.Unmarshal(out, &cr); err != nil {
		// If JSON parsing fails, use raw output as result
		return strings.TrimSpace(string(out)), nil
	}
	return cr.Result, nil
}
