# Layout & Wiring (reference)

Read when scaffolding a package, shaping an entity, wiring `main()`, loading config, or checking cold-start. The core (`SKILL.md`) states the rules; this file is the full detail and examples.

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
│   ├── notifications/            # feature package: handler + store + tests, all in one dir
│   │   ├── handler.go
│   │   ├── store_cosmos.go
│   │   ├── fake_store_test.go
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
    └── e2e/                      # black-box docker-compose tests against the compiled binary ONLY
```

**Hard rules for layout**:

- **No `pkg/` directory.** This is a private API; everything goes in `internal/`. (See [Go pkg antipattern](https://sub-pop.net/post/go-pkg-antipattern/).)
- **No `domain/`, `application/`, `infrastructure/` directories.** Layered-architecture directory names fight Go's package model. Slice by feature, not by layer.
- **One feature = one package.** Handler, store, and their tests live in the same directory. Promote shared code to a sibling package only when a *second* feature actually needs it.
- **No default service layer.** Handlers call the store directly. Introduce a separate service type only when real business logic emerges that more than one entry point (e.g. handler + background worker) needs — never as a pass-through layer scaffolded "for structure".
- **Cross-cutting platform code in `internal/platform/`.** Logger, HTTP server factory, telemetry, config loading. Nothing business-specific.
- **One binary in `cmd/api/`.** If a second binary (e.g. polling worker) is added later, it goes in `cmd/worker/` as a sibling.

## 1. Plain structs, validated at construction

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

## 2. "Accept interfaces, return structs" + consumer-side interfaces

Interfaces in Go are defined where they are *used*, not where they are *implemented*. This is one of the highest-leverage Go idioms and the one most often violated by transplants from other languages:

- **Constructors return concrete `*struct`s**, never interfaces. `func NewCosmosStore(...) *CosmosStore` — not `... NotificationStore` (the interface).
- **Interfaces are declared by the consumer**, with only the methods that consumer actually uses. A handler that calls `Save` and `Get` defines:
  ```go
  type notificationStore interface {
      Save(ctx context.Context, n Notification) error
      Get(ctx context.Context, id NotificationID) (Notification, error)
  }
  ```
  Lowercase — unexported — because no other package needs to satisfy this contract by name. Go's structural typing makes `*CosmosStore` satisfy it implicitly.
- **No `I` prefix on interface names** (`Notifier`, not `INotifier`). Idiomatic Go uses `-er` suffixes for single-method interfaces (`Reader`, `Saver`, `Validator`) or a descriptive noun.
- **Say "store", not "repository".** Repository is DDD/.NET vocabulary; Go names things by what they are. `CosmosStore`, `store_cosmos.go`, `fakeNotificationStore` — not `NotificationRepository` or `cosmos_repo.go`.
- **One large `Store` interface in a shared package is an anti-pattern.** Keep interfaces small and consumer-local. Beads' fat `Storage` interface is the exception for *public extension APIs*, not internal code.

This unlocks effortless test doubles: hand-write `type fakeNotificationStore struct { ... }` with the two methods the handler test needs, and the compiler accepts it.

## 3. Errors as values

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

## 4. Context propagation

- **Every function that does I/O, blocks, or calls another function that does, takes `ctx context.Context` as its FIRST parameter.** No exceptions in handler chains, store methods, HTTP clients, or service-bus operations.
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

## 9. Concurrency

- **`golang.org/x/sync/errgroup` with `WithContext`** for fan-out with cancellation:
  ```go
  g, gctx := errgroup.WithContext(ctx)
  for _, id := range ids {
      g.Go(func() error { return process(gctx, id) })
  }
  return g.Wait()
  ```
- **`sync.Mutex`** for protecting shared maps/slices. **`atomic.*`** for hot counters.
- **Channels only when there's a real producer/consumer pipeline.** Don't use them as event buses or one-shot signals when `errgroup`/`sync.Once` would do.
- **No `go func() { ... }()` without an owner.** Every goroutine either (a) belongs to an `errgroup` that's `Wait()`ed, or (b) is a documented long-lived background goroutine started at process boot with a shutdown channel.
- **`time.NewTimer` over `time.After` in loops.** `time.After` leaks until the timer fires; `NewTimer` + explicit `Stop()` doesn't.

## 12. Config

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

## 13. Dependency injection — manual wiring in `main()`

No `wire`, no `fx`, no DI framework. `cmd/api/main.go` wires everything top-to-bottom:

```go
func main() {
    cfg, err := platform.LoadConfig()
    if err != nil { log.Fatal(err) }

    logger := platform.NewLogger(cfg.LogLevel)

    cosmosClient := platform.MustCosmosClient(cfg, logger)
    sbClient     := platform.MustServiceBusClient(cfg, logger)

    notifStore := notifications.NewCosmosStore(cosmosClient, logger)
    sbPub      := servicebus.NewPublisher(sbClient, logger)
    apnsCli    := apns.NewClient(cfg, logger)
    validator  := auth.NewAuth0Validator(cfg, logger)

    mux := http.NewServeMux()
    notifications.Routes(mux, notifStore, sbPub, apnsCli, validator, logger)

    srv := platform.NewServer(":"+cfg.Port, mw.Chain(mux, validator, logger))

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    go func() { _ = srv.ListenAndServe() }()
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = srv.Shutdown(shutdownCtx)
}
```

When this hits 200+ lines, reconsider — but until then, plain wiring beats codegen on cold-start time, debuggability, and AI-comprehension.

## 14. Cold-start checklist (Container Apps scales to zero)

A statically-linked Go binary hits sub-second cold starts out of the box, but you can still ruin it:

- **`CGO_ENABLED=0`** in build — pure-Go static binary, no glibc dance.
- **`-ldflags="-s -w" -trimpath`** — smaller binary, faster mmap.
- **`distroless` or `scratch` base image** — sub-20 MB final image.
- **No `init()` doing I/O.** All work in `main()`, behind logging.
- **No reflection-based DI startup** (this is why `fx` is banned).
- **`/healthz` returns 200 immediately** without touching dependencies. Container Apps' readiness probe gates traffic on this; any work here delays first request.
- **SDK clients constructed in `main()`**, but actual connections open lazily on first call — this is the SDK default and the right behaviour.
