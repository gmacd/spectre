package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spectre.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpen_IdempotentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spectre.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open on existing db: %v", err)
	}
	defer s2.Close()

	if err := s2.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestAppendAndLoadMessages_Order(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const convID = "cli:default"

	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you"},
	}
	for _, m := range msgs {
		if err := s.AppendMessage(ctx, convID, m.Role, m.Content); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}

	got, err := s.LoadMessages(ctx, convID, 0)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(got) != len(msgs) {
		t.Fatalf("got %d messages, want %d", len(got), len(msgs))
	}
	for i, m := range msgs {
		if got[i] != m {
			t.Errorf("message %d = %+v, want %+v", i, got[i], m)
		}
	}
}

func TestLoadMessages_Limit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const convID = "cli:default"

	for i := 0; i < 5; i++ {
		if err := s.AppendMessage(ctx, convID, "user", string(rune('a'+i))); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}

	got, err := s.LoadMessages(ctx, convID, 2)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
	if got[0].Content != "d" || got[1].Content != "e" {
		t.Errorf("got messages %+v, want last two [d e] in order", got)
	}
}

func TestLoadMessages_DifferentConversationsIsolated(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.AppendMessage(ctx, "cli:a", "user", "in a"); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := s.AppendMessage(ctx, "cli:b", "user", "in b"); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	gotA, err := s.LoadMessages(ctx, "cli:a", 0)
	if err != nil {
		t.Fatalf("LoadMessages a: %v", err)
	}
	if len(gotA) != 1 || gotA[0].Content != "in a" {
		t.Errorf("conversation a = %+v, want single message 'in a'", gotA)
	}

	gotB, err := s.LoadMessages(ctx, "cli:b", 0)
	if err != nil {
		t.Fatalf("LoadMessages b: %v", err)
	}
	if len(gotB) != 1 || gotB[0].Content != "in b" {
		t.Errorf("conversation b = %+v, want single message 'in b'", gotB)
	}
}
