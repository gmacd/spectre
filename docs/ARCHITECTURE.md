# Spectre architecture

Spectre is a single-process, Go-based personal AI chat assistant. It talks to
a configurable LLM backend over an OpenAI-compatible chat completions API
(initially a llama-swap instance on the user's homelab network), persists
conversation history locally in sqlite, and is reachable via a CLI today with
Matrix planned as a follow-up.

Tool/function-calling by the LLM is a long-term goal but explicitly out of
scope for the initial build — the core chat loop is structured so it can be
added later without a rewrite.

## Package layout

```
spectre/
├── go.mod
├── cmd/spectre/main.go       entrypoint; os.Args subcommand dispatch only
├── internal/config/          JSON config struct + Load(path) + validation
├── internal/store/           sqlite persistence (conversations + messages)
├── internal/llm/             client for the OpenAI-compatible chat endpoint
├── internal/agent/           transport-independent core: HandleMessage(ctx, conversationID, text) (string, error)
└── internal/api/             REST server (daemon side): routes call into agent
```

`spectre send` builds its HTTP request inline in `cmd/spectre` rather than
via a separate client package — it's a handful of lines and doesn't warrant
its own package until a second consumer exists.

Dependency direction: `agent` depends on `store` + `llm`; `api` depends on
`agent`. A future `internal/matrix` package would become a second consumer
of `agent`, parallel to `api` — this is what keeps Matrix integration a
follow-up rather than a rewrite.

Routing uses Go 1.22+ `http.ServeMux` method+pattern support
(`mux.HandleFunc("POST /v1/messages", ...)`) — no router dependency needed.

## Dependencies

Only `modernc.org/sqlite` (pure-Go sqlite driver, no cgo) is approved and in
use. Every other dependency (e.g. a Matrix SDK such as `mautrix-go` for the
follow-up phase) requires separate explicit approval before being added.

## SQLite schema (`internal/store`)

Applied via `go:embed` + `CREATE TABLE IF NOT EXISTS` on daemon startup.

```sql
CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY,        -- e.g. "cli:default", later "matrix:!roomid:server"
    source      TEXT NOT NULL,           -- 'cli' | 'rest' | 'matrix' (future)
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role            TEXT NOT NULL CHECK (role IN ('system','user','assistant')),
    content         TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation_id_id ON messages (conversation_id, id);
```

`PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout=5000` are set on
connection open for smoother single-writer/many-reader behavior.

## JSON config shape

```json
{
  "listen_addr": "127.0.0.1:8787",
  "db_path": "~/.local/share/spectre/spectre.db",
  "llm": {
    "base_url": "http://llm.homelab.internal",
    "model": "llama-3.1-70b-instruct",
    "api_key": "",
    "system_prompt": "You are Spectre, a helpful personal assistant.",
    "timeout_seconds": 120,
    "max_history_messages": 40
  },
  "log_level": "info"
}
```

- `llm.api_key` — optional bearer token, unused today, future-proofing so
  the config shape doesn't need to change if a backend requires one.
- `llm.max_history_messages` — every `send` resends conversation history to
  the LLM; this caps it to the last N messages (count-based, not
  token-aware — good enough for v1, revisit if needed).

Default config path: `$XDG_CONFIG_HOME/spectre/config.json`, falling back to
`~/.config/spectre/config.json`, overridable via `-config` on both `serve`
and `send`.

`listen_addr` defaults to loopback and the REST API has **no auth** — only
acceptable while loopback-only. Binding non-loopback (e.g. for a future
remote Matrix bridge process) requires adding auth first.

## REST API (daemon)

**`POST /v1/messages`**
```json
// request
{ "conversation_id": "cli:default", "message": "hello" }
// response 200
{ "conversation_id": "cli:default", "reply": "hi there" }
// error
{ "error": "llm request failed: ..." }
```

**`GET /v1/health`** → `{ "status": "ok", "db": "ok", "llm_configured": true }`
(cheap: pings DB, does not round-trip to the LLM on every check).

## CLI

Manual `os.Args[1]` dispatch, each subcommand with its own
`flag.NewFlagSet(name, flag.ExitOnError)`:

- `spectre serve -config <path>` — loads config, opens store, builds llm
  client + agent, starts the REST server, blocks on
  `signal.NotifyContext(os.Interrupt, syscall.SIGTERM)`, graceful
  `http.Server.Shutdown` on signal.
- `spectre send [-conversation cli:default] [-addr http://127.0.0.1:8787] "message text"`
  — message is a positional arg; POSTs to `/v1/messages`; prints `reply` to
  stdout; non-zero exit + stderr message on error.
- `spectre version` — prints build info via `runtime/debug.ReadBuildInfo()`.

## Talking to the llama-swap endpoint (`internal/llm`)

Standard OpenAI-compatible non-streaming request/response (streaming is
explicitly deferred):

```go
type chatCompletionRequest struct {
    Model    string        `json:"model"`
    Messages []chatMessage `json:"messages"`
    Stream   bool          `json:"stream"`
}
type chatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}
type chatCompletionResponse struct {
    Choices []struct{ Message chatMessage `json:"message"` } `json:"choices"`
    Error   *struct{ Message string `json:"message"` } `json:"error,omitempty"`
}
```

- One `http.Client{Timeout: ...}` built once at startup, reused across calls.
- `http.NewRequestWithContext` so caller cancellation/timeouts propagate.
- Non-2xx responses surface the response body in the returned error; guard
  against an empty `Choices` slice before indexing.
- The real response shape against the actual llama-swap endpoint should be
  verified with a manual `curl` (this environment cannot reach
  `llm.homelab.internal` to check in advance).

`agent.HandleMessage(ctx, conversationID, text)` flow: append user message →
load last `max_history_messages` messages (prepending `system_prompt` if not
already present) → call LLM → append assistant reply → return it. This
signature is the reuse seam for a future Matrix adapter.

## Logging

`log/slog`, text handler to stderr, level from `log_level` config. HTTP
middleware logs method/path/status/duration/conversation_id at Info. Message
*content* is not logged at Info (only at Debug, if at all) — personal
conversations may be sensitive.

## Explicitly out of scope for the initial build

- Tool/function-calling by the LLM (deferred; `agent.HandleMessage`'s
  signature is chosen so this can be added without restructuring).
- Matrix integration (deferred follow-up; will need a Matrix SDK dependency
  such as `mautrix-go`, requiring separate approval when that phase starts).
- Auth on the REST API (fine while loopback-only; revisit if `listen_addr`
  ever changes).
- Streaming responses.

See `docs/TASKS.md` for the implementation checklist.
