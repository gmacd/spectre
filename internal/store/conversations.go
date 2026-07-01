package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Message is a single chat message belonging to a conversation.
type Message struct {
	Role    string
	Content string
}

// deriveSource extracts the transport prefix from a conversation id, e.g.
// "cli:default" -> "cli". Conversation ids without a prefix are recorded
// with source "unknown".
func deriveSource(conversationID string) string {
	if i := strings.Index(conversationID, ":"); i > 0 {
		return conversationID[:i]
	}
	return "unknown"
}

// ensureConversation inserts the conversation row if it doesn't already
// exist and bumps updated_at either way.
func (s *Store) ensureConversation(ctx context.Context, conversationID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations (id, source) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
	`, conversationID, deriveSource(conversationID))
	if err != nil {
		return fmt.Errorf("ensure conversation %s: %w", conversationID, err)
	}
	return nil
}

// AppendMessage records a message in conversationID, creating the
// conversation row on first use.
func (s *Store) AppendMessage(ctx context.Context, conversationID, role, content string) error {
	if err := s.ensureConversation(ctx, conversationID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (conversation_id, role, content) VALUES (?, ?, ?)
	`, conversationID, role, content)
	if err != nil {
		return fmt.Errorf("append message to %s: %w", conversationID, err)
	}
	return nil
}

// LoadMessages returns the messages for conversationID in chronological
// order. If limit is > 0, only the most recent limit messages are returned.
func (s *Store) LoadMessages(ctx context.Context, conversationID string, limit int) ([]Message, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if limit > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT role, content FROM (
				SELECT role, content, id FROM messages
				WHERE conversation_id = ?
				ORDER BY id DESC
				LIMIT ?
			) ORDER BY id ASC
		`, conversationID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT role, content FROM messages
			WHERE conversation_id = ?
			ORDER BY id ASC
		`, conversationID)
	}
	if err != nil {
		return nil, fmt.Errorf("load messages for %s: %w", conversationID, err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages for %s: %w", conversationID, err)
	}
	return messages, nil
}
