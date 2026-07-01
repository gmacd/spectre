// Package agent implements the transport-independent core: given an
// inbound message for a conversation, it loads history, calls the LLM, and
// persists the reply. Both the REST API and a future Matrix adapter are
// meant to call into this package rather than duplicating this logic.
package agent

import (
	"context"
	"fmt"

	"spectre/internal/llm"
	"spectre/internal/store"
)

// Store is the persistence dependency HandleMessage needs.
type Store interface {
	AppendMessage(ctx context.Context, conversationID, role, content string) error
	LoadMessages(ctx context.Context, conversationID string, limit int) ([]store.Message, error)
}

// LLM is the chat-completions dependency HandleMessage needs.
type LLM interface {
	ChatCompletion(ctx context.Context, messages []llm.Message) (string, error)
}

// Agent wires a Store and an LLM together behind a single, transport-agnostic
// entry point.
type Agent struct {
	store              Store
	llm                LLM
	systemPrompt       string
	maxHistoryMessages int
}

// New builds an Agent. maxHistoryMessages caps how many prior messages are
// sent to the LLM on each call (0 or negative means unbounded).
func New(s Store, l LLM, systemPrompt string, maxHistoryMessages int) *Agent {
	return &Agent{
		store:              s,
		llm:                l,
		systemPrompt:       systemPrompt,
		maxHistoryMessages: maxHistoryMessages,
	}
}

// HandleMessage appends text as a user message to conversationID, calls the
// LLM with the (capped) conversation history, persists the assistant's
// reply, and returns it.
func (a *Agent) HandleMessage(ctx context.Context, conversationID, text string) (string, error) {
	if err := a.store.AppendMessage(ctx, conversationID, "user", text); err != nil {
		return "", fmt.Errorf("append user message: %w", err)
	}

	history, err := a.store.LoadMessages(ctx, conversationID, a.maxHistoryMessages)
	if err != nil {
		return "", fmt.Errorf("load history: %w", err)
	}

	messages := make([]llm.Message, 0, len(history)+1)
	if a.systemPrompt != "" {
		// The system prompt is injected fresh from config on every call
		// rather than persisted, so a config change takes effect
		// immediately without needing a data migration.
		messages = append(messages, llm.Message{Role: "system", Content: a.systemPrompt})
	}
	for _, m := range history {
		messages = append(messages, llm.Message{Role: m.Role, Content: m.Content})
	}

	reply, err := a.llm.ChatCompletion(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm chat completion: %w", err)
	}

	if err := a.store.AppendMessage(ctx, conversationID, "assistant", reply); err != nil {
		return "", fmt.Errorf("append assistant message: %w", err)
	}

	return reply, nil
}
