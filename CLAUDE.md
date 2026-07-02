# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Spectre is a single-process, Go-based personal AI chat assistant. It talks to
a configurable LLM backend over an OpenAI-compatible chat completions API
(a llama-swap instance on the user's homelab network), persists conversation
history locally in sqlite, and is reachable via a CLI today, with Matrix
integration planned as a follow-up.

Full design doc: `docs/ARCHITECTURE.md` (package layout, schema, config
shape, REST API contract, CLI behavior — read it before making structural
changes). Implementation checklist / what's deferred: `docs/TASKS.md`.

## Commands

```
go build ./...                        # build everything
go test ./...                         # run all tests
go test ./internal/agent/...          # run one package's tests
go test ./internal/store/ -run TestAppendAndLoadMessages_Order  # single test
go vet ./...
```

No Makefile/CI config exists yet — these are plain `go` toolchain commands.
Go 1.26.4 is pinned in `go.mod`.

Running the daemon locally:

```
go run ./cmd/spectre serve -config <path>
go run ./cmd/spectre send "hello"
```

## Architecture

Strict one-way dependency chain — do not introduce reverse imports:

```
internal/config   JSON config struct + Load(path) + validation, no other deps
internal/store    sqlite persistence (conversations + messages)
internal/llm      client for the OpenAI-compatible chat completions endpoint
internal/agent    transport-independent core: Agent.HandleMessage(ctx, conversationID, text) (string, error)
                  depends on store + llm (via small local interfaces, not concrete types)
internal/api      REST server (daemon side); routes call into agent
cmd/spectre       entrypoint; os.Args subcommand dispatch only (serve/send/version)
```

`agent.Agent` depends on `store` and `llm` through minimal interfaces
defined in `internal/agent/agent.go` (`Store`, `LLM`), not their concrete
types — this is what makes `agent` testable with fakes and keeps it
transport-agnostic. A future `internal/matrix` package would become a
second consumer of `agent`, parallel to `internal/api`; this is the seam
that keeps Matrix support an additive follow-up rather than a rewrite.

`spectre send` builds its HTTP request inline in `cmd/spectre` (in
`send.go`) rather than via a dedicated client package — intentionally, until
a second consumer of that logic exists.

**Dependency policy**: only `modernc.org/sqlite` (pure-Go, no cgo) is
approved. Any new dependency (e.g. a Matrix SDK like `mautrix-go` for the
follow-up phase) requires explicit approval before being added — don't add
one speculatively.

### Core request flow

`agent.HandleMessage(ctx, conversationID, text)`: append user message to
store → load last `max_history_messages` messages (prepending
`system_prompt` fresh from config, not persisted, so config changes apply
immediately without a migration) → call the LLM → persist the assistant
reply → return it. This signature is the deliberate reuse seam for a future
Matrix adapter, so preserve it when touching this code.

### Config

Single JSON file, default path `$XDG_CONFIG_HOME/spectre/config.json`
falling back to `~/.config/spectre/config.json`, overridable via `-config`
on both `serve` and `send`. `~` in `db_path` is expanded. See
`docs/ARCHITECTURE.md` for the full shape; required fields are `db_path`,
`llm.base_url`, `llm.model` — everything else has a default in
`internal/config/config.go`.

### REST API

`POST /v1/messages` and `GET /v1/health` only, routed with Go 1.22+
`http.ServeMux` method+pattern syntax (`mux.HandleFunc("POST /v1/messages",
...)`) — no router dependency. **No auth**, acceptable only because
`listen_addr` defaults to loopback; adding auth is a prerequisite for ever
binding non-loopback.

### Logging

`log/slog`, text handler to stderr, level from config `log_level`. HTTP
middleware logs method/path/status/duration/conversation_id at Info level.
Message *content* must not be logged at Info or above — conversations may be
sensitive; content logging, if ever added, belongs at Debug only.

## Explicitly out of scope (don't implement without discussion)

- Tool/function-calling by the LLM
- Matrix integration (`internal/matrix`, needs a new SDK dependency)
- Auth on the REST API
- Streaming responses
