---
name: go-coding-standards
description: MUST consult before writing ANY Go code. Enforces idiomatic, secure Go for the Town Crier API and any future Go module — flat feature-sliced layout under internal/, consumer-side interfaces, stdlib net/http + log/slog, hand-written test fakes with stdlib testing, manual main() wiring, official Azure SDK (azcosmos/azservicebus) usage, and a hardened HTTP server profile (timeouts, body limits, TLS, constant-time secret comparison). Trigger whenever the user asks you to write, scaffold, refactor, lint, or review any .go file or a Go module's go.mod, including HTTP handlers, repositories, background workers, tests, or main() wiring. Do NOT use for iOS/Swift, React/TypeScript, Pulumi, GitHub Actions, or non-Go code.
---

# Go Coding Standards

## Overview

This skill enforces **idiomatic, secure Go** for any Go module in this repository — initially the API pilot, possibly later a polling worker. Write Go the way Go is written: flat feature packages, consumer-side interfaces, hand-written fakes, stdlib over frameworks. The goal is code the next AI agent reading this codebase will recognise as idiomatic.

The single overriding rule: **idiomatic, secure Go**. If a pattern would feel out of place in the Go standard library or in widely-respected open-source Go projects (Prometheus, Consul, the official Kubernetes client libraries), don't use it.

## Scope and target layout

The Go module lives in `api-go/`. The skill's rules apply to every `.go` file under that tree.

```
api-go/
├── go.mod
├── go.sum
├── cmd/
│   └── api/
│       └── main.go              # the only binary; manual DI wiring
├── internal/
│   ├── notifications/            # feature package: handler + repo + service + tests, all in one dir
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── cosmos_repo.go
│   │   ├── fake_repo_test.go
│   │   └── handler_test.go
│   ├── auth/                     # Auth0 JWT validation + middleware
│   ├── planit/                   # PlanIt client
│   ├── apns/                     # APNs HTTP/2 client
│   ├── servicebus/               # ASB publisher/consumer
│   └── platform/                 # cross-cutting: logger, server, telemetry, config
│       ├── config.go
│       ├── logger.go
│       ├── server.go             # hardened http.Server factory
│       └── telemetry.go
└── tests/
    └── integration/              # docker-compose-driven end-to-end tests
```

**Hard rules for layout**:

- **No `pkg/` directory.** This is a private API; everything goes in `internal/`. (See [Go pkg antipattern](https://sub-pop.net/post/go-pkg-antipattern/).)
- **No `domain/`, `application/`, `infrastructure/` directories.** Layered-architecture directory names fight Go's package model. Slice by feature, not by layer.
- **One feature = one package.** Handler, service, repository, and their tests live in the same directory. Promote shared code to a sibling package only when a *second* feature actually needs it.
- **Cross-cutting platform code in `internal/platform/`.** Logger, HTTP server factory, telemetry, config loading. Nothing business-specific.
- **One binary in `cmd/api/`.** If a second binary (e.g. polling worker) is added later, it goes in `cmd/worker/` as a sibling.

## Core mandates

### 1. Plain structs, validated at construction

Go's idiom is plain data with validation at the boundary, not "rich" domain models with private setters and invariant-guarding methods. Resist the urge to manufacture ceremony:

- Simple `struct`s with exported fields (or unexported fields + small accessor methods only when an invariant truly needs guarding).
- Validation in **constructors** (`func NewNotification(...) (Notification, error)`), not in setters. Go has no equivalent of `private set`; rely on the constructor returning a validated value and treat the struct as immutable by convention.
- Receiver methods for behaviour, but **don't manufacture methods for the sake of "rich models"** — a free function that takes a struct is fine.
- `encoding/json` works with struct tags out of the box (`json:"reference"`); no source generators or codegen needed.

**Example — idiomatic Go entity:**
```go
type Notification struct {
    ID            NotificationID `json:"id"`
    UserID        UserID         `json:"userId"`
    AuthorityCode string         `json:"authorityCode"`
    Reference     string         `json:"reference"`
    DispatchedAt  time.Time      `json:"dispatchedAt"`
}

func NewNotification(userID UserID, authority, reference string, now time.Time) (Notification, error) {
    if authority == "" {
        return Notification{}, errors.New("authority is required")
    }
    if reference == "" {
        return Notification{}, errors.New("reference is required")
    }
    return Notification{
        ID:            NewNotificationID(),
        UserID:        userID,
        AuthorityCode: authority,
        Reference:     reference,
        DispatchedAt:  now,
    }, nil
}
```

`NotificationID` and `UserID` are typed strings (`type NotificationID string`) for compile-time safety without ceremony.

### 2. "Accept interfaces, return structs" + consumer-side interfaces

Interfaces in Go are defined where they are *used*, not where they are *implemented*. This is one of the highest-leverage Go idioms and the one most often violated by transplants from other languages:

- **Constructors return concrete `*struct`s**, never interfaces. `func NewCosmosNotificationRepo(...) *CosmosNotificationRepo` — not `... Repository`.
- **Interfaces are declared by the consumer**, with only the methods that consumer actually uses. A handler that calls `Save` and `Get` defines:
  ```go
  type notificationStore interface {
      Save(ctx context.Context, n Notification) error
      Get(ctx context.Context, id NotificationID) (Notification, error)
  }
  ```
  Lowercase — unexported — because no other package needs to satisfy this contract by name. Go's structural typing makes `*CosmosNotificationRepo` satisfy it implicitly.
- **No `I` prefix on interface names** (`Notifier`, not `INotifier`). Idiomatic Go uses `-er` suffixes for single-method interfaces (`Reader`, `Saver`, `Validator`) or a descriptive noun.
- **One large `Repository` interface in a shared package is an anti-pattern.** Keep interfaces small and consumer-local. Beads' fat `Storage` interface is the exception for *public extension APIs*, not internal code.

This unlocks effortless test doubles: hand-write `type fakeNotificationStore struct { ... }` with the two methods the handler test needs, and the compiler accepts it.

### 3. Errors as values

- **Sentinel errors at the top of the package** for known failure modes:
  ```go
  var (
      ErrNotFound       = errors.New("not found")
      ErrAlreadyClaimed = errors.New("already claimed")
  )
  ```
- **Wrap with `%w`** when adding context: `return fmt.Errorf("save notification %s: %w", id, err)`. Never `fmt.Errorf("...: %v", err)` — it discards the chain.
- **`errors.Is` / `errors.As`** at consumption sites. Never `err == ErrNotFound` (it breaks under wrapping); never `err.Error() == "..."`.
- **Typed errors for rich data** (HTTP status, retry hints):
  ```go
  type APIError struct {
      StatusCode int
      Body       string
  }
  func (e *APIError) Error() string { return fmt.Sprintf("api error: status %d: %s", e.StatusCode, e.Body) }
  ```
- **Never `panic` outside `main()` startup.** Production code returns errors. A `panic` is reserved for "this binary cannot continue at boot" (e.g. missing required config).

### 4. Context propagation

- **Every function that does I/O, blocks, or calls another function that does, takes `ctx context.Context` as its FIRST parameter.** No exceptions in handler chains, repository methods, HTTP clients, or service-bus operations.
- **`context.TODO()`** is permitted only in `main()` and one-off scripts; never in library code.
- **Set timeouts at the boundary.** Every outbound call (Cosmos, Auth0, APNs, Service Bus) wraps `ctx` with `context.WithTimeout(ctx, X)` and `defer cancel()` immediately.
- **Honour cancellation in retry/poll loops:**
  ```go
  select {
  case <-ctx.Done():
      return ctx.Err()
  case <-time.After(backoff):
  }
  ```
- **Never store `ctx` in a struct field** unless it's a deliberately scoped helper documented as such. Pass it explicitly through the call.

### 5. Testing — stdlib `testing` first, `testify/require` allowed

- **Framework**: stdlib `testing` is the default. `github.com/stretchr/testify/require` and `.../assert` are permitted for assertion ergonomics (`require.NoError(t, err)` is genuinely better than the four-line `if err != nil` form repeated 50 times).
- **Forbidden**: `testify/suite` (fights `t.Cleanup`), `gomock`, `mockery`, any reflection-based mocking library. Hand-written fakes only.
- **Table-driven tests** with subtests are the default shape:
  ```go
  func TestNotification_Validation(t *testing.T) {
      t.Parallel()
      tests := []struct {
          name      string
          authority string
          reference string
          wantErr   bool
      }{
          {"valid", "GLA", "24/0001", false},
          {"missing authority", "", "24/0001", true},
          {"missing reference", "GLA", "", true},
      }
      for _, tc := range tests {
          t.Run(tc.name, func(t *testing.T) {
              t.Parallel()
              _, err := NewNotification(UserID("u1"), tc.authority, tc.reference, time.Now())
              if (err != nil) != tc.wantErr {
                  t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
              }
          })
      }
  }
  ```
- **Hand-written fakes** live in `_test.go` files in the same package. No public test fixtures package.
  ```go
  type fakeNotificationStore struct {
      saved   map[NotificationID]Notification
      saveErr error
  }

  func newFakeNotificationStore() *fakeNotificationStore {
      return &fakeNotificationStore{saved: map[NotificationID]Notification{}}
  }

  func (f *fakeNotificationStore) Save(ctx context.Context, n Notification) error {
      if f.saveErr != nil {
          return f.saveErr
      }
      f.saved[n.ID] = n
      return nil
  }

  func (f *fakeNotificationStore) Get(ctx context.Context, id NotificationID) (Notification, error) {
      n, ok := f.saved[id]
      if !ok {
          return Notification{}, ErrNotFound
      }
      return n, nil
  }
  ```
- **HTTP integration tests** use `httptest.NewServer` with `http.HandlerFunc`. Outbound client tests assert against a captured `*http.Request`.
- **No builder pattern.** Go has struct literals and small helper constructors. `notif := Notification{ID: "n1", ...}` or `notif := newTestNotification(t)`. Builders add ceremony Go does not need.
- **`t.Parallel()`** on every test that doesn't share global state. Catches data races and keeps the suite fast.
- **`t.Helper()`** in helper functions so failures point at the caller.
- **`t.Cleanup()`** for teardown instead of `defer` when the cleanup is a fixture concern.

### 6. HTTP server — hardened defaults

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

### 7. HTTP client — secure outbound

Every outbound client (PlanIt, Auth0, Cosmos REST fallback, APNs) MUST:

- Use `http.NewRequestWithContext(ctx, ...)`. Never `http.Get` / `http.Post` (no context, no cancellation).
- Set per-request timeout via `context.WithTimeout`.
- Bound response body: `io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))` — typically 10 MiB.
- Reject non-HTTPS URLs (`url.Scheme != "https"`) except for localhost in tests.
- Use a shared `*http.Client` with `Timeout` set (e.g. 30s) and a tuned `Transport` (connection pool, MaxIdleConnsPerHost ≥ 10 for hot upstreams).
- Retry only idempotent methods (GET, HEAD, PUT) on 429/5xx, with exponential backoff + jitter. Honour `Retry-After`. POST is **not** retried by default.
- Branch 4xx → permanent typed error, 429/5xx → retry.

### 8. Logging — `log/slog` only

- **`log/slog` is the only logger.** No `zap`, no `zerolog`, no `logrus`, no stdlib `log`.
- **JSON handler in production**, text handler permitted only in local dev:
  ```go
  logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
  ```
- **Pass the logger explicitly** through constructors. Do not use `slog.Default()` in library code; only `main()` may set it.
- **Key/value pairs, typed**: `logger.Info("notification dispatched", "id", n.ID, "user", n.UserID)`. Linted via `sloglint`.
- **Never log secrets or PII**: use the `SecretString` value type for credentials (see §10) and redact email/phone fields in structured logs.

### 9. Concurrency

- **`golang.org/x/sync/errgroup` with `WithContext`** for fan-out with cancellation:
  ```go
  g, gctx := errgroup.WithContext(ctx)
  for _, id := range ids {
      id := id
      g.Go(func() error { return process(gctx, id) })
  }
  return g.Wait()
  ```
- **`sync.Mutex`** for protecting shared maps/slices. **`atomic.*`** for hot counters.
- **Channels only when there's a real producer/consumer pipeline.** Don't use them as event buses or one-shot signals when `errgroup`/`sync.Once` would do.
- **No `go func() { ... }()` without an owner.** Every goroutine either (a) belongs to an `errgroup` that's `Wait()`ed, or (b) is a documented long-lived background goroutine started at process boot with a shutdown channel.
- **`time.NewTimer` over `time.After` in loops.** `time.After` leaks until the timer fires; `NewTimer` + explicit `Stop()` doesn't.

### 10. Security primitives

- **`SecretString` value type** for any credential, redacting in `String()` and `MarshalJSON`:
  ```go
  type SecretString struct{ value string }
  func NewSecret(v string) SecretString             { return SecretString{value: v} }
  func (s SecretString) String() string             { return "[REDACTED]" }
  func (s SecretString) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
  func (s SecretString) Expose() string             { return s.value }
  ```
  Use for Auth0 client secrets, APNs auth keys, Service Bus connection strings, Cosmos primary keys. Pass `SecretString` through config; call `.Expose()` only at the boundary where the credential leaves the process.
- **`crypto/subtle.ConstantTimeCompare`** for HMAC/token equality. Never `==` on a secret.
- **`crypto/rand`** for IDs, nonces, tokens. Never `math/rand` for anything security-sensitive.
- **TLS 1.2 minimum** on any custom `tls.Config`: `MinVersion: tls.VersionTLS12`. Never `InsecureSkipVerify: true` in non-test code.
- **Auth0 JWT validation** via `github.com/auth0/go-jwt-middleware/v3`. Always validate `iss`, `aud`, `exp`. Cache JWKS with a TTL.
- **No CSRF** for native iOS clients on `Authorization: Bearer`. CSRF applies only to cookie-session browser POSTs. Don't add ceremony you don't need.

### 11. Data access — official Azure SDK

- **Cosmos DB**: `github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos`. Official Microsoft SDK, actively maintained. Do **not** use `microsoft/gocosmos` (a `database/sql` driver that loses Cosmos semantics) or the community vippsas SDK.
- **Service Bus**: `github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus`. The official SDK is the only supported path — do not hand-roll a REST client.
- **Auth**: `github.com/Azure/azure-sdk-for-go/sdk/azidentity`. Use `DefaultAzureCredential` in deployed environments; `ClientSecretCredential` only where required.
- **Communication Services (email)**: `github.com/Azure/azure-sdk-for-go/sdk/messaging/azcommunicationservices/sender` (or the `azcommunication` namespace's email package — check the current name in `go.mod` when implementing).
- **Repository struct holds the SDK client**, exposes only the methods the consumer interface declares. Map Cosmos documents ↔ domain structs at the repo boundary; consumers never see SDK types.
- **No ORM**, no `gorm`, no `sqlx`. Cosmos is not relational.
- **Partition keys** are designed around query access patterns. Document the choice per container in a comment at the top of the repo file.

### 12. Config

`internal/platform/config.go`:
```go
type Config struct {
    Port              string
    LogLevel          slog.Level
    CosmosEndpoint    string
    CosmosKey         SecretString
    Auth0Domain       string
    Auth0Audience     string
    ServiceBusFQDN    string
    APNsKeyID         string
    APNsTeamID        string
    APNsAuthKey       SecretString
}

func LoadConfig() (Config, error) {
    cfg := Config{
        Port:           getenv("PORT", "8080"),
        CosmosEndpoint: mustEnv("COSMOS_ENDPOINT"),
        CosmosKey:      NewSecret(mustEnv("COSMOS_KEY")),
        // ...
    }
    if err := cfg.validate(); err != nil {
        return Config{}, err
    }
    return cfg, nil
}
```
No `viper`, no `godotenv`. Container Apps provides env vars; load them, validate them, fail fast at startup.

### 13. Dependency injection — manual wiring in `main()`

No `wire`, no `fx`, no DI framework. `cmd/api/main.go` wires everything top-to-bottom:

```go
func main() {
    cfg, err := platform.LoadConfig()
    if err != nil { log.Fatal(err) }

    logger := platform.NewLogger(cfg.LogLevel)

    cosmosClient := platform.MustCosmosClient(cfg, logger)
    sbClient     := platform.MustServiceBusClient(cfg, logger)

    notifRepo := notifications.NewCosmosRepo(cosmosClient, logger)
    sbPub     := servicebus.NewPublisher(sbClient, logger)
    apnsCli   := apns.NewClient(cfg, logger)
    validator := auth.NewAuth0Validator(cfg, logger)

    mux := http.NewServeMux()
    notifications.Routes(mux, notifRepo, sbPub, apnsCli, validator, logger)

    srv := platform.NewServer(":"+cfg.Port, mw.Chain(mux, validator, logger))

    platform.RunWithGracefulShutdown(srv, logger)
}
```

When this hits 200+ lines, reconsider — but until then, plain wiring beats codegen on cold-start time, debuggability, and AI-comprehension.

### 14. Cold-start checklist (Container Apps scales to zero)

A statically-linked Go binary hits sub-second cold starts out of the box, but you can still ruin it:

- **`CGO_ENABLED=0`** in build — pure-Go static binary, no glibc dance.
- **`-ldflags="-s -w" -trimpath`** — smaller binary, faster mmap.
- **`distroless` or `scratch` base image** — sub-20 MB final image.
- **No `init()` doing I/O.** All work in `main()`, behind logging.
- **No reflection-based DI startup** (this is why `fx` is banned).
- **`/healthz` returns 200 immediately** without touching dependencies. Container Apps' readiness probe gates traffic on this; any work here delays first request.
- **SDK clients constructed in `main()`**, but actual connections open lazily on first call — this is the SDK default and the right behaviour.

## Forbidden

- `pkg/` directory.
- `domain/`, `application/`, `infrastructure/` directories.
- `I`-prefix on interface names.
- Constructors that return interfaces.
- `gomock`, `mockery`, `testify/suite`, any reflection-based mocking.
- Builder pattern for test data construction.
- `panic` outside `main()` startup wiring.
- `http.Get`, `http.Post`, `http.DefaultClient` (no context, no timeouts).
- `time.After` in long-running loops.
- `log.Println` / stdlib `log` in production paths (use `slog`).
- `math/rand` for any security-sensitive value.
- `InsecureSkipVerify: true` outside localhost tests.
- `==` comparison on secrets/tokens (use `subtle.ConstantTimeCompare`).
- `viper`, `godotenv`, `wire`, `fx`, `gin`, `echo`, `chi` (unless an explicit ADR justifies the dependency).
- `interface{}` / `any` in public APIs (use generics when a type parameter fits; otherwise model the type).
- Returning `(nil, nil)` from a function with `(T, error)` signature.
- Goroutines without a clear owner / shutdown path.

## Workflow

### Build

```bash
cd api-go && go build ./...
```

### Test

```bash
cd api-go && go test ./...                  # all tests
cd api-go && go test -race ./...            # with race detector (required before merge)
cd api-go && go test -run TestName ./...    # single test
```

### Format

```bash
cd api-go && gofmt -w .                     # idiomatic formatting
cd api-go && go vet ./...                   # standard correctness checks
```

### Lint

```bash
cd api-go && golangci-lint run ./...
```

The lint config lives in `api-go/.golangci.yml`. Bootstrap it from the bundled asset:

```bash
cp .claude/skills/go-coding-standards/assets/.golangci.yml api-go/.golangci.yml
```

The baseline enables `errcheck`, `govet`, `staticcheck`, `gosec`, `sloglint`, `bodyclose`, `contextcheck`, `errorlint`, `noctx`, `rowserrcheck`, `sqlclosecheck`, `copyloopvar`, `intrange`, `misspell`, `unparam`, `unconvert`. Style-opinion linters (`funlen`, `cyclop`, `wsl`, `gofumpt`) are deliberately disabled — they fight AI agents without catching real bugs.

## Naming conventions

- **Packages**: short, lowercase, single word where possible (`notifications`, `planit`, `apns`). No underscores, no camelCase, no plurals where a singular reads naturally.
- **Files**: lowercase with underscores (`cosmos_repo.go`, `handler_test.go`).
- **Exported identifiers**: PascalCase. **Unexported**: camelCase.
- **Interfaces**: noun or `-er` suffix (`Notifier`, `Validator`, `Store`). No `I` prefix.
- **Receivers**: short — one or two letters matching the type (`func (n *Notification) ...`). Never `this`/`self`.
- **Error variables**: `Err` prefix (`ErrNotFound`, `ErrAlreadyClaimed`).
- **Test functions**: `TestSubject_Behaviour` (e.g. `TestNotification_RejectsEmptyAuthority`).
- **Constants**: PascalCase if exported, camelCase if not. No `SHOUTY_CASE`.

## Pre-commit checklist (run before every PR)

```bash
cd api-go && \
  gofmt -l . | tee /dev/stderr | wc -l | xargs -I{} test {} = 0 && \
  go vet ./... && \
  golangci-lint run ./... && \
  go test -race ./...
```

A single failing step blocks the PR.
