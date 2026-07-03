// Package api exposes spectre's daemon functionality over a local REST API.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// conversationIDKey is the context key for the conversation id extracted
// from incoming requests.
type conversationIDKey struct{}

// setConversationID returns a copy of ctx with conversationID stored.
func setConversationID(ctx context.Context, conversationID string) context.Context {
	return context.WithValue(ctx, conversationIDKey{}, conversationID)
}

// getConversationID retrieves the conversation id from ctx, if present.
func getConversationID(ctx context.Context) string {
	if v, ok := ctx.Value(conversationIDKey{}).(string); ok {
		return v
	}
	return ""
}

// Agent is the dependency handlers call into to produce a reply.
type Agent interface {
	HandleMessage(ctx context.Context, conversationID, text string) (string, error)
}

// Pinger is the health-check dependency for the backing store.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server is the daemon's REST API.
type Server struct {
	agent         Agent
	db            Pinger
	llmConfigured bool
	logger        *slog.Logger
	httpServer    *http.Server
}

// NewServer builds a Server listening on addr. llmConfigured is reported
// verbatim on /v1/health and does not itself contact the LLM backend.
func NewServer(addr string, a Agent, db Pinger, llmConfigured bool, logger *slog.Logger) *Server {
	s := &Server{
		agent:         a,
		db:            db,
		llmConfigured: llmConfigured,
		logger:        logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("POST /v1/messages", s.handleMessages)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.loggingMiddleware(mux),
	}
	return s
}

// ListenAndServe blocks serving the REST API until it is shut down or an
// error occurs. It always returns a non-nil error (http.ErrServerClosed on
// a clean Shutdown).
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		fields := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
		}
		if cid := getConversationID(r.Context()); cid != "" {
			fields = append(fields, "conversation_id", cid)
		}
		s.logger.Info("request", fields...)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
