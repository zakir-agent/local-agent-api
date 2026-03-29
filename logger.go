package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RequestLog struct {
	Time     string        `json:"time"`
	ElapsedMs int64        `json:"elapsed_ms"`
	Messages []ChatMessage `json:"messages"`
	Response string        `json:"response"`
	Error    string        `json:"error,omitempty"`
}

var (
	logDir  = "logs"
	logMu   sync.Mutex
	logFile *os.File
	logDate string
)

func initLogDir() {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("warning: cannot create log dir: %v", err)
	}
}

func getLogFile() (*os.File, error) {
	today := time.Now().Format("2006-01-02")
	if logFile != nil && logDate == today {
		return logFile, nil
	}
	if logFile != nil {
		logFile.Close()
	}
	path := filepath.Join(logDir, today+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	logFile = f
	logDate = today
	return logFile, nil
}

func closeLogFile() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func logRequest(messages []ChatMessage, response string, errMsg string, elapsed time.Duration) {
	entry := RequestLog{
		Time:      time.Now().Format(time.RFC3339),
		ElapsedMs: elapsed.Milliseconds(),
		Messages:  messages,
		Response:  response,
		Error:     errMsg,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("warning: failed to marshal log entry: %v", err)
		return
	}

	logMu.Lock()
	defer logMu.Unlock()

	f, err := getLogFile()
	if err != nil {
		log.Printf("warning: failed to open log file: %v", err)
		return
	}
	f.Write(append(data, '\n'))
}
