package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

const cliTimeout = 90 * time.Second

var (
	model    string
	agentCLI string

	callAgentCLI func(context.Context, []ChatMessage) (*cliOutput, error) = callClaudeCLI
)

func main() {
	port := flag.String("port", "8080", "listen port")
	flag.StringVar(&model, "model", "composer-2-fast", "model for the backend CLI (--model)")
	agentCLIFlag := flag.String("agent-cli", "cursor", "backend: claude (Claude Code) or cursor (Cursor Agent headless)")
	flag.Parse()

	switch *agentCLIFlag {
	case "claude":
		agentCLI = "claude"
		callAgentCLI = callClaudeCLI
		if _, err := exec.LookPath("claude"); err != nil {
			log.Fatal("claude CLI not found in PATH, please install it first")
		}
	case "cursor":
		agentCLI = "cursor"
		callAgentCLI = callCursorAgent
		if _, err := exec.LookPath("cursor-agent"); err != nil {
			log.Fatal("Cursor Agent CLI (cursor-agent) not found in PATH, please install it first")
		}
	default:
		log.Fatal("-agent-cli must be claude or cursor")
	}

	initLogDir()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("/v1/responses", handleResponses)
	mux.HandleFunc("/v1/messages", handleMessages)
	mux.HandleFunc("/v1/models", handleModels)

	addr := ":" + *port
	srv := &http.Server{Addr: addr, Handler: mux}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("local-agent-api listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down, waiting for in-flight requests...")

	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout+10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	closeLogFile()
	log.Println("server stopped")
}
