# HTTP Hardening (reference)

Read when the bead touches HTTP handlers, the server factory, routing, timeouts, body limits, TLS, or any outbound HTTP client (PlanIt, Auth0, APNs, etc.). The core (`SKILL.md`) states the rule; this file is the full profile.

## 6. HTTP server — hardened defaults

Use stdlib `net/http` with the Go 1.22+ ServeMux. Do **not** add `chi`, `gorilla/mux`, `gin`, or `echo` unless a specific requirement (sub-router groups with shared middleware that's genuinely painful to express in stdlib) is justified. Stdlib wins on cold start and zero supply-chain risk.

**Always construct the server via `internal/platform/server.go`:**
```go
func NewServer(addr string, handler http.Handler) *http.Server {
    return &http.Server{
        Addr:              addr,
        Handler:           handler,
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       15 * time.Second,
        WriteTimeout:      15 * time.Second,
        IdleTimeout:       60 * time.Second,
        MaxHeaderBytes:    1 << 20, // 1 MiB
    }
}
```
The default zero-valued timeouts on `http.Server` allow slowloris attacks. Never accept defaults.

**Routing (Go 1.22+ syntax):**
```go
mux := http.NewServeMux()
mux.HandleFunc("GET /v1/notifications/{id}", h.getNotification)
mux.HandleFunc("POST /v1/notifications", h.createNotification)
```

**Body size limit** on every handler that reads a body:
```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
if err := json.NewDecoder(r.Body).Decode(&req); err != nil { ... }
```

**Middleware composition** is plain `func(http.Handler) http.Handler`. Build a small chain in `cmd/api/main.go`:
```go
handler := mw.Recover(mw.RequestID(mw.SLogRequest(logger)(mw.AuthRequired(validator)(mux))))
```
No third-party middleware framework. Compose by hand; it's twenty lines.

**Panic recovery** at the top of every chain — convert to 500 + structured log.

**Graceful shutdown** in `main()`:
```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
go func() { _ = srv.ListenAndServe() }()
<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_ = srv.Shutdown(shutdownCtx)
```

## 7. HTTP client — secure outbound

Every outbound client (PlanIt, Auth0, Cosmos REST fallback, APNs) MUST:

- Use `http.NewRequestWithContext(ctx, ...)`. Never `http.Get` / `http.Post` (no context, no cancellation).
- Set per-request timeout via `context.WithTimeout`.
- Bound response body: `io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))` — typically 10 MiB.
- Reject non-HTTPS URLs (`url.Scheme != "https"`) except for localhost in tests.
- Use a shared `*http.Client` with `Timeout` set (e.g. 30s) and a tuned `Transport` (connection pool, MaxIdleConnsPerHost ≥ 10 for hot upstreams).
- Retry only idempotent methods (GET, HEAD, PUT) on 429/5xx, with exponential backoff + jitter. Honour `Retry-After`. POST is **not** retried by default.
- Branch 4xx → permanent typed error, 429/5xx → retry.
