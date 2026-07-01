package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"spectre/internal/config"
)

type sendRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type sendResponse struct {
	ConversationID string `json:"conversation_id"`
	Reply          string `json:"reply"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func runSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	configPath := fs.String("config", "", "path to JSON config file, used to resolve the daemon address if -addr is not given")
	addr := fs.String("addr", "", "daemon REST base URL, e.g. http://127.0.0.1:8787 (default: derived from config's listen_addr)")
	conversation := fs.String("conversation", "cli:default", "conversation id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	message := strings.Join(fs.Args(), " ")
	if message == "" {
		return fmt.Errorf("usage: spectre send [flags] <message>")
	}

	baseURL := *addr
	if baseURL == "" {
		cfg, err := config.Load(*configPath)
		if err != nil {
			return fmt.Errorf("resolve daemon address: -addr not given and could not load config: %w", err)
		}
		baseURL = "http://" + cfg.ListenAddr
	}
	baseURL = strings.TrimRight(baseURL, "/")

	reqBody, err := json.Marshal(sendRequest{ConversationID: *conversation, Message: message})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(baseURL+"/v1/messages", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("request to daemon at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return fmt.Errorf("daemon returned error: %s", errResp.Error)
		}
		return fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var sendResp sendResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println(sendResp.Reply)
	return nil
}
