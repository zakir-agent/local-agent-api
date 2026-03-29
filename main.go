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

var model string

func main() {
	port := flag.String("port", "8080", "listen port")
	flag.StringVar(&model, "model", "claude-sonnet-4-6", "claude model to use")
	flag.Parse()

	if _, err := exec.LookPath("claude"); err != nil {
		log.Fatal("claude CLI not found in PATH, please install it first")
	}

	initLogDir()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChatCompletions)
	mux.HandleFunc("/v1/messages", handleMessages)
	mux.HandleFunc("/v1/models", handleModels)

	addr := ":" + *port
	srv := &http.Server{Addr: addr, Handler: mux}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("claude-local-api listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down, waiting for in-flight requests...")

	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	closeLogFile()
	log.Println("server stopped")
}
