package agent

import (
	"context"
	"errors"
	"testing"

	"spectre/internal/llm"
	"spectre/internal/store"
)

type fakeStore struct {
	messages map[string][]store.Message
}

func newFakeStore() *fakeStore {
	return &fakeStore{messages: make(map[string][]store.Message)}
}

func (f *fakeStore) AppendMessage(ctx context.Context, conversationID, role, content string) error {
	f.messages[conversationID] = append(f.messages[conversationID], store.Message{Role: role, Content: content})
	return nil
}

func (f *fakeStore) LoadMessages(ctx context.Context, conversationID string, limit int) ([]store.Message, error) {
	all := f.messages[conversationID]
	if limit <= 0 || limit >= len(all) {
		return append([]store.Message(nil), all...), nil
	}
	return append([]store.Message(nil), all[len(all)-limit:]...), nil
}

type fakeLLM struct {
	lastMessages []llm.Message
	reply        string
	err          error
}

func (f *fakeLLM) ChatCompletion(ctx context.Context, messages []llm.Message) (string, error) {
	f.lastMessages = messages
	if f.err != nil {
		return "", f.err
	}
	return f.reply, nil
}

func TestHandleMessage_RoundTrip(t *testing.T) {
	s := newFakeStore()
	l := &fakeLLM{reply: "hi there"}
	a := New(s, l, "you are helpful", 40)

	reply, err := a.HandleMessage(context.Background(), "cli:default", "hello")
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if reply != "hi there" {
		t.Errorf("reply = %q, want %q", reply, "hi there")
	}

	stored := s.messages["cli:default"]
	if len(stored) != 2 {
		t.Fatalf("stored messages = %d, want 2", len(stored))
	}
	if stored[0] != (store.Message{Role: "user", Content: "hello"}) {
		t.Errorf("stored[0] = %+v", stored[0])
	}
	if stored[1] != (store.Message{Role: "assistant", Content: "hi there"}) {
		t.Errorf("stored[1] = %+v", stored[1])
	}

	if len(l.lastMessages) != 2 {
		t.Fatalf("llm received %d messages, want 2 (system + user)", len(l.lastMessages))
	}
	if l.lastMessages[0] != (llm.Message{Role: "system", Content: "you are helpful"}) {
		t.Errorf("first message to llm = %+v, want system prompt", l.lastMessages[0])
	}
	if l.lastMessages[1] != (llm.Message{Role: "user", Content: "hello"}) {
		t.Errorf("second message to llm = %+v, want user message", l.lastMessages[1])
	}
}

func TestHandleMessage_HistoryAccumulates(t *testing.T) {
	s := newFakeStore()
	l := &fakeLLM{reply: "ok"}
	a := New(s, l, "", 40)

	if _, err := a.HandleMessage(context.Background(), "cli:default", "first"); err != nil {
		t.Fatalf("HandleMessage 1: %v", err)
	}
	if _, err := a.HandleMessage(context.Background(), "cli:default", "second"); err != nil {
		t.Fatalf("HandleMessage 2: %v", err)
	}

	// Second call should see: [assistant:ok, user:second] plus the earlier
	// [user:first] -> total history sent to llm before appending the new
	// reply is 3 messages (first user, first assistant reply, second user).
	if len(l.lastMessages) != 3 {
		t.Fatalf("llm received %d messages on second call, want 3, got %+v", len(l.lastMessages), l.lastMessages)
	}
	if l.lastMessages[0].Content != "first" {
		t.Errorf("history not accumulating correctly: %+v", l.lastMessages)
	}
}

func TestHandleMessage_HistoryCapped(t *testing.T) {
	s := newFakeStore()
	l := &fakeLLM{reply: "ok"}
	a := New(s, l, "", 2)

	for i := 0; i < 3; i++ {
		if _, err := a.HandleMessage(context.Background(), "cli:default", "msg"); err != nil {
			t.Fatalf("HandleMessage %d: %v", i, err)
		}
	}

	if len(l.lastMessages) > 2 {
		t.Errorf("llm received %d messages, want at most 2 (max_history_messages cap)", len(l.lastMessages))
	}
}

func TestHandleMessage_LLMError(t *testing.T) {
	s := newFakeStore()
	l := &fakeLLM{err: errors.New("backend down")}
	a := New(s, l, "", 40)

	_, err := a.HandleMessage(context.Background(), "cli:default", "hello")
	if err == nil {
		t.Fatal("HandleMessage returned nil error, want error from llm")
	}

	// The user message should still have been persisted even though the
	// llm call failed, so nothing is silently lost.
	stored := s.messages["cli:default"]
	if len(stored) != 1 || stored[0].Content != "hello" {
		t.Errorf("stored messages = %+v, want user message preserved despite llm error", stored)
	}
}
