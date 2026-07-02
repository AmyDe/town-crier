---
name: go-coding-standards
description: MUST consult before writing ANY Go code. Enforces idiomatic, secure Go for the Town Crier API and any future Go module — flat feature-sliced layout under internal/, consumer-side interfaces, stdlib net/http + log/slog, hand-written test fakes with stdlib testing, manual main() wiring, official Azure SDK (azcosmos/azservicebus) usage, and a hardened HTTP server profile (timeouts, body limits, TLS, constant-time secret comparison). Trigger whenever the user asks you to write, scaffold, refactor, lint, or review any .go file or a Go module's go.mod, including HTTP handlers, repositories, background workers, tests, or main() wiring. Do NOT use for iOS/Swift, React/TypeScript, Pulumi, GitHub Actions, or non-Go code.
---

# Go Coding Standards

Idiomatic, secure Go for every Go module in this repo (the `api-go/` API, `/cli`, `/infra`). Write Go the way the standard library and respected OSS projects (Prometheus, Consul, Kubernetes client libs) write it. **The single overriding rule: if a pattern would feel out of place in the Go standard library, don't use it.** Read this core first; pull the matching reference below when the bead touches that area.

## Architecture (always applies)

- **Flat, feature-sliced layout under `internal/`. One feature = one package** — handler + store + tests in one directory; promote to a sibling only when a *second* feature needs it. **No `pkg/` directory.** **No `domain/`, `application/`, `infrastructure/` directories** — slice by feature, not by layer.
- **No default service layer.** Handlers call the store directly; add a separate service type only when real business logic is shared by more than one entry point — never a pass-through scaffolded "for structure".
- **Cross-cutting platform code in `internal/platform/`** (logger, server factory, telemetry, config). **One binary in `cmd/api/`**; a second binary is a sibling `cmd/worker/`.
- **Accept interfaces, return structs.** Constructors return concrete `*struct`s, never interfaces. **Interfaces are declared by the consumer** — unexported, only the methods that consumer uses. **No `I` prefix**; use `-er`/noun names. Keep interfaces small and consumer-local — one fat shared `Store` interface is an anti-pattern. Say **"store", not "repository"**.
- **Plain structs validated in constructors** (`NewX(...) (X, error)`), not rich models with private setters. Typed string IDs (`type UserID string`).
- **Errors as values** — sentinel `Err*` vars, wrap with `%w`, consume with `errors.Is`/`errors.As` (never `==` or `.Error()` string match). **Never `panic` outside `main()` startup.**
- **`ctx context.Context` is the FIRST parameter** of every function that does I/O or calls one that does; set timeouts at outbound boundaries.
- **stdlib `net/http` (Go 1.22+ ServeMux) + `log/slog`** — no web framework, no third-party logger; compose middleware by hand. **Manual DI wired top-to-bottom in `main()`** — no `wire`/`fx`. **No ORM** — go through the driver directly (`pgx` for Postgres; official Azure SDK for Cosmos/Service Bus); consumers never see SDK types.

## Test-double conventions (always applies)

- **stdlib `testing` is the default**; `testify/require`/`assert` allowed for assertions. **Forbidden**: `testify/suite`, `gomock`, `mockery`, any reflection-based mocking.
- **Hand-written fakes only**, in `_test.go` in the same package (a `fakeX` struct satisfying the consumer interface). No public fixtures package; **no builder pattern** — struct literals or a small `newTestX(t)` helper.
- **Table-driven subtests** are the default. `t.Parallel()` on every test without shared global state; `t.Helper()` in helpers; `t.Cleanup()` for teardown. **HTTP tests** use `httptest.NewServer`; outbound-client tests assert against a captured `*http.Request`.
- **Naming**: fakes `fakeX`; tests `TestSubject_Behaviour`.
- **Real-DB integration tests are mandatory for Postgres store ports.** They live in the package they exercise behind `//go:build integration`, using the `internal/platform/postgres/pgtest` harness (`pgtest.New(t)` + `Truncate`), run with `make -C api-go test-integration` (or `go test -tags=integration ./...`), and `t.Skip` when no DB is reachable. They are **additive** to the fakes, covering spatial/SQL behaviour fakes cannot honestly model; a port that only passes the untagged suite is not done. Full harness API in `references/testing-and-integration.md`.

## Forbidden

- `pkg/` directory.
- `domain/`, `application/`, `infrastructure/` directories.
- Pass-through service layers (handler → service → store where the service adds no logic).
- `Repository` naming for data access (use `Store`).
- `Get` prefix on accessor methods.
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

## References (load on demand)

- `references/layout-and-wiring.md` — read when scaffolding a package, shaping an entity/errors/context/concurrency, wiring `main()`, loading config, or checking cold-start (target-layout tree + examples).
- `references/http-hardening.md` — read when the bead touches HTTP handlers, servers, routing, timeouts, body limits, TLS, or an outbound HTTP client.
- `references/security-and-logging.md` — read when the bead touches secrets/credentials, tokens, HMAC, TLS, random IDs, Auth0 JWT, or `slog` logging.
- `references/azure-sdk.md` — read when the bead touches a store, Cosmos DB, Service Bus, ACS email, Azure auth, partition keys, or the no-ORM rule.
- `references/testing-and-integration.md` — read when writing any test, a fake, or real-DB integration tests (examples + the `pgtest` harness API).
- `references/workflow-and-naming.md` — read when running build/test/lint/format, bootstrapping `.golangci.yml`, naming, or assembling the pre-commit checklist.
