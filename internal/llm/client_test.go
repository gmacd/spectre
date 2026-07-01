package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChatCompletion_HappyPath(t *testing.T) {
	var gotReq chatCompletionRequest
	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		resp := chatCompletionResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message Message `json:"message"`
		}{Message: Message{Role: "assistant", Content: "hi there"}})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-model", "secret-key", 5*time.Second)
	reply, err := c.ChatCompletion(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if reply != "hi there" {
		t.Errorf("reply = %q, want %q", reply, "hi there")
	}
	if gotReq.Model != "test-model" {
		t.Errorf("request model = %q, want %q", gotReq.Model, "test-model")
	}
	if gotReq.Stream {
		t.Error("request Stream = true, want false")
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret-key")
	}
}

func TestChatCompletion_NoAPIKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := chatCompletionResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message Message `json:"message"`
		}{Message: Message{Role: "assistant", Content: "ok"}})
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-model", "", 5*time.Second)
	if _, err := c.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header = %q, want empty when no api key configured", gotAuth)
	}
}

func TestChatCompletion_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "backend on fire"}}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-model", "", 5*time.Second)
	_, err := c.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("ChatCompletion returned nil error, want error for 500 status")
	}
}

func TestChatCompletion_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatCompletionResponse{})
	}))
	defer server.Close()

	c := New(server.URL, "test-model", "", 5*time.Second)
	_, err := c.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("ChatCompletion returned nil error, want error for empty choices")
	}
}
