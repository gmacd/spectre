package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

type fakeAgent struct {
	reply string
	err   error

	gotConversationID string
	gotText            string
}

func (f *fakeAgent) HandleMessage(ctx context.Context, conversationID, text string) (string, error) {
	f.gotConversationID = conversationID
	f.gotText = text
	if f.err != nil {
		return "", f.err
	}
	return f.reply, nil
}

type fakePinger struct {
	err error
}

func (f *fakePinger) Ping(ctx context.Context) error {
	return f.err
}

func testServer(agent *fakeAgent, pinger *fakePinger) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer("127.0.0.1:0", agent, pinger, true, logger)
}

func TestHandleHealth_OK(t *testing.T) {
	s := testServer(&fakeAgent{}, &fakePinger{})
	req := httptest.NewRequest("GET", "/v1/health", nil)
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "ok" || resp.DB != "ok" || !resp.LLMConfigured {
		t.Errorf("unexpected health response: %+v", resp)
	}
}

func TestHandleHealth_DBError(t *testing.T) {
	s := testServer(&fakeAgent{}, &fakePinger{err: errors.New("db locked")})
	req := httptest.NewRequest("GET", "/v1/health", nil)
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.DB == "ok" {
		t.Errorf("expected DB status to reflect ping error, got %+v", resp)
	}
}

func TestHandleMessages_HappyPath(t *testing.T) {
	agent := &fakeAgent{reply: "hi there"}
	s := testServer(agent, &fakePinger{})

	body, _ := json.Marshal(SendRequest{ConversationID: "cli:default", Message: "hello"})
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var resp SendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Reply != "hi there" || resp.ConversationID != "cli:default" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if agent.gotConversationID != "cli:default" || agent.gotText != "hello" {
		t.Errorf("agent received conversation_id=%q text=%q", agent.gotConversationID, agent.gotText)
	}
}

func TestHandleMessages_MissingFields(t *testing.T) {
	s := testServer(&fakeAgent{}, &fakePinger{})

	body, _ := json.Marshal(SendRequest{ConversationID: "", Message: ""})
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleMessages_InvalidJSON(t *testing.T) {
	s := testServer(&fakeAgent{}, &fakePinger{})

	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleMessages_WrongContentType(t *testing.T) {
	s := testServer(&fakeAgent{}, &fakePinger{})

	body, _ := json.Marshal(SendRequest{ConversationID: "cli:default", Message: "hello"})
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleMessages_AgentError(t *testing.T) {
	agent := &fakeAgent{err: errors.New("llm unreachable")}
	s := testServer(agent, &fakePinger{})

	body, _ := json.Marshal(SendRequest{ConversationID: "cli:default", Message: "hello"})
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	s.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}
