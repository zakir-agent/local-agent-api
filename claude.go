package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type cliUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

type claudeResult struct {
	Result     string   `json:"result"`
	StopReason string   `json:"stop_reason"`
	Usage      cliUsage `json:"usage"`
}

type cliOutput struct {
	Result     string
	StopReason string
	Usage      cliUsage
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

func callClaudeCLI(ctx context.Context, messages []ChatMessage) (*cliOutput, error) {
	prompt := formatMessages(messages)

	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--print", "--model", model, "--output-format", "json")
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

	var cr claudeResult
	if err := json.Unmarshal(out, &cr); err != nil {
		// If JSON parsing fails, use raw output as result
		return &cliOutput{Result: strings.TrimSpace(string(out))}, nil
	}
	return &cliOutput{
		Result:     cr.Result,
		StopReason: cr.StopReason,
		Usage:      cr.Usage,
	}, nil
}
