# Implementation checklist

See `docs/ARCHITECTURE.md` for the design these steps implement.

- [x] `go mod init` + `internal/config` (Load/validate) + tests
- [x] `internal/store`: add `modernc.org/sqlite`, schema embed, CRUD + tests
- [x] `internal/llm` client + httptest-based tests; verified against the real llama-swap endpoint (`http://llm.homelab.internal`, model `coder-big`) — response shape matches (extra `reasoning_content`/`usage`/`timings` fields are ignored harmlessly), and a full `serve`/`send` round trip correctly recalled context across messages
- [x] `internal/agent.HandleMessage` + tests with fake LLM
- [x] `internal/api`: `/v1/health` then `/v1/messages`
- [x] `cmd/spectre`: `serve` and `send` subcommands wired up
- [x] graceful shutdown, `version` subcommand, cold-start test

## Deferred / future work (not in initial build)

- [ ] Tool/function-calling by the LLM
- [ ] Matrix integration (`internal/matrix`, needs a Matrix SDK dependency — requires separate approval)
- [ ] Auth on the REST API (needed if `listen_addr` ever binds non-loopback)
- [ ] Streaming responses

## Follow-up improvements

- [x] Log `conversation_id` in HTTP middleware (now adds it from request context)
- [x] Validate `Content-Type: application/json` on `POST /v1/messages`
- [x] Deduplicate `SendRequest`/`SendResponse`/`ErrorResponse` types in `internal/api/types.go`
- [x] Make `send` client timeout configurable via `-timeout` flag
- [x] Add "trace" log level option to `parseLogLevel`
