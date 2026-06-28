# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**grok2api-go** — A Go API gateway that proxies requests to Grok (x.ai) and exposes OpenAI-compatible (`/v1/chat/completions`, `/v1/images/generations`, etc.) and Anthropic-compatible (`/v1/messages`) REST APIs. Manages multiple Grok SSO tokens in pools (basic/super/heavy) with lease-based account selection, quota tracking, and automatic retry.

## Build & Run

```bash
# Build
go build -o grok2api .

# Build with version override
go build -ldflags "-X main.Version=1.2.3" -o grok2api .

# Run (listens on 0.0.0.0:8000 by default)
go run .

# No tests exist in this project
```

Environment variables that affect startup: `LOG_LEVEL`, `LOG_FILE_ENABLED`, `SERVER_HOST`, `SERVER_PORT`, `ACCOUNT_STORAGE`, `ACCOUNT_LOCAL_PATH`, `DATA_DIR`, `LOG_DIR`. All documented in `.env.example`.

## Architecture

### Entry Point: `main.go`

Single-file bootstrap with a documented 13-step lifecycle: config load → JSONL account repo → account directory → TLS transport → leader election → media cache → API server → background goroutines → graceful shutdown. Leader election uses `flock` on Unix (always-leader on Windows); only the leader runs quota refresh loops.

### Package Layout (`internal/`)

| Package | Responsibility |
|---------|---------------|
| `account/` | Multi-account pool management: `Record` (persistent model), `Directory` (in-memory runtime with lease-based `Reserve`/`Release`/`Feedback`), `Repository` interface, JSONL text file implementation, state machine for account lifecycle, quota windows, quota refresh service |
| `api/` | HTTP handlers and middleware. Server struct wires all deps. Uses Gin (`gin-gonic/gin`) with route groups: OpenAI (`/v1/`), Anthropic (`/v1/messages`), Admin (`/admin/api/`). Auth middleware via `verifyAPIKey` / `verifyAdminKey` |
| `grok/` | Upstream Grok protocol: TLS-fingerprinted transport (browser impersonation via `tls-client`), header construction with Chrome header ordering, chat payload builders, SSE stream adapters (grok.com + console.x.ai), gRPC-Web for auth ops, usage fetching, asset management |
| `config/` | TOML-based layered config: `config.defaults.toml` → user `config.toml` → `GROK_*` env overrides. Hot-reload on file mtime change per-request |
| `model/` | Model registry — 33 models mapped to mode IDs, tiers, and capability bitmasks |
| `logger/` | Leveled logger (DEBUG/INFO/WARN/ERROR) with daily file rotation |
| `platform/` | Error types, path resolution, token sanitization |
| `storage/` | Media file cache with SQLite-backed LRU eviction |

### Key Patterns

**Account selection (lease-based)**: `Reserve()` returns a `*Lease` with a token; caller uses it then calls `Release()`. `Feedback()` updates health/quota after each request. Two strategies: quota-aware (scores by remaining quota, inflight, failures) and random (with cooling period).

**Retry**: Each chat handler retries up to `maxRetries` times, excluding failed tokens. On-demand quota refresh triggers on first attempt failure.

**Stream processing**: Two SSE adapters — `StreamAdapter` for grok.com (text/thinking/image events with citation tracking) and `ConsoleStreamAdapter` for console.x.ai (text delta parsing). Both convert Grok's proprietary SSE to OpenAI-compatible `chat.completion.chunk` frames.

**Anti-fingerprinting**: All upstream requests use TLS fingerprinting (Chrome profiles 120-146), ordered HTTP/2 headers via `fhttp`, randomized `x-statsig-id`, and proper `Sec-Ch-Ua-*` client hints.

**Config hot-reload**: Every request checks config file mtime; cheap no-op when unchanged. Admin API can update config at runtime, persisted to user config file.

**Storage**: Account data stored in `accounts.jsonl` (JSONL text file, one JSON record per line, atomic temp-rename writes, revision-based incremental sync). Media cache uses `local_media_cache.db` (SQLite-backed LRU eviction index for cached images/videos).

### Data Flow (Chat Request)

1. Request arrives at `/v1/chat/completions` → auth middleware → handler
2. Handler determines capability (chat/image/video/console) from model name → dispatches to specialized handler
3. Handler calls `reserveAccount(mode)` → gets a `*Lease` with SSO token
4. Builds upstream payload via `grok.BuildChatPayload()` or `grok.BuildConsolePayload()`
5. Makes TLS-fingerprinted request to upstream via `Transport`
6. Streams response through `StreamAdapter`/`ConsoleStreamAdapter` → writes OpenAI SSE frames
7. On success/failure, calls `Feedback()` to update account health; retries with different account on failure

## Configuration

Primary config: `config.defaults.toml` (shipped defaults) + `config.toml` (user overrides in data dir). Key sections: `[app]`, `[features]`, `[proxy.egress]`, `[proxy.clearance]`, `[account.refresh]`, `[account.selection]`, `[retry]`, timeouts.

## Dependencies of Note

- `bogdanfinn/fhttp` + `bogdanfinn/tls-client` — HTTP client with TLS fingerprinting (not standard `net/http`)
- `pelletier/go-toml/v2` — TOML config parsing
- `modernc.org/sqlite` — Pure-Go SQLite (no CGO required)
