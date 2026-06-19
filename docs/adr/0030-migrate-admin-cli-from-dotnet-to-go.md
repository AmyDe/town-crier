# 0030. Rebuild the admin CLI (`tc`) from .NET to Go

Date: 2026-06-19

## Status

Accepted

Completes the language consolidation begun by [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md) (backend) and [ADR 0029](0029-migrate-infrastructure-from-dotnet-to-go.md) (infrastructure), both of which named `/cli` as the last remaining .NET component. With this change, **.NET is fully removed from the repository.**

## Context

The `tc` admin CLI is a small self-contained tool for back-office operations against the API's `/v1/admin/*` endpoints: bulk-generating offer codes, granting subscription tiers by email, and listing users. It was originally written in C#/.NET 10 with Native AOT (`/cli`, ~12 source files plus a TUnit test project).

[ADR 0028](0028-migrate-backend-from-dotnet-to-go.md) migrated the backend to Go and deleted `/api`; [ADR 0029](0029-migrate-infrastructure-from-dotnet-to-go.md) ported the Pulumi program to Go. Both explicitly left `/cli` on .NET, calling it "the sole remaining .NET component." That left a single small utility as the only reason the repo, CI, and contributors still needed the .NET toolchain:

- `pr-gate.yml` carried two .NET-only jobs (`cli-format` via `dotnet format`, `cli-build-test` via `dotnet test`) with a `setup-dotnet` step and a NuGet cache, used by nothing else.
- The CLI talks only to the Go API. The request/response contracts it depends on (the `/v1/admin/*` shapes, the `X-Admin-Key` header, the `text/plain` offer-code stream) are already exercised by Go tests in `api-go/internal/admin`. Keeping the client in a different language added a second toolchain for zero architectural benefit.
- Native AOT bought fast startup, but a statically-linked Go binary delivers the same sub-second start with `CGO_ENABLED=0`, and Go is already the team's server-side language.

## Decision

**Rebuild `tc` in Go as a feature-identical replacement (`/cli`), and delete the .NET implementation.** Tracked as bead **tc-w5x5**.

The Go CLI is a standalone module (`github.com/AmyDe/town-crier/cli`, `go 1.26`) — separate from `api-go` so it carries none of the backend's dependency tree. Layout follows the Go standards skill: one binary in `cmd/tc/`, all logic in `internal/tc/` (one cohesive package), hand-written tests in the same package.

### Feature parity

Every observable behaviour of the .NET CLI is preserved byte-for-byte where it is user-facing:

| Concern | Preserved |
|---------|-----------|
| Commands | `generate-offer-codes`, `grant-subscription`, `list-users`, `help`, `version` |
| Argument grammar | `<command> --key value …`; `help`/`-h`/`--help`/no-args → help; trailing valueless flag ignored; case-insensitive keys |
| Validation | count 1–1000, duration 1–365 (ASCII-digits only, matching `NumberStyles.None`); offer tiers `Personal\|Pro`; grant tiers `Free\|Personal\|Pro`; case-insensitive with canonical normalisation |
| Endpoints | `POST /v1/admin/offer-codes`, `PUT /v1/admin/subscriptions`, `GET /v1/admin/users` with `X-Admin-Key` |
| Output | identical stdout/stderr text, the paginated `list-users` table (24/32/10 columns, `(none)` for null email), the `Next page? [y/N]` prompt, and the streamed offer codes |
| Exit codes | `0` success, `1` usage/validation/config, `2` API/runtime |
| Config | `~/.config/tc/config.json` (`{url, apiKey}`), with `--url` / `--api-key` overriding the file |

### What changed

| Concern | Was (.NET) | Now (Go) |
|---------|-----------|----------|
| Language / runtime | C#/.NET 10, Native AOT | Go 1.26, static binary (`CGO_ENABLED=0`) |
| Layout | `cli/src/tc/` + `cli/tests/tc.tests/` (TUnit) | `cli/cmd/tc/` + `cli/internal/tc/` (`go test`) |
| Build pins | `cli/global.json`, `Directory.Build.props`, `.editorconfig` | `cli/go.mod`, `cli/.golangci.yml` |
| HTTP client | `System.Net.Http.Json` + source-generated JSON | stdlib `net/http` + `encoding/json` |
| CI jobs | `cli-format` (`dotnet format`), `cli-build-test` (`dotnet test`) | `cli-lint` (`gofmt`/`go vet`/`golangci-lint`), `cli-build-test` (`go build`/`go test -race`) |
| Install | `install.sh` via `dotnet publish` (AOT) | `install.sh` via `go build` (static) |

### What was kept

- The binary name (`tc`), command surface, config path, and `install.sh` install-to-`~/.local/bin` flow are unchanged.
- `pr-gate.yml` still keys CLI jobs on the `cli/` change category; the two job slots in the aggregating `gate` check are preserved (`cli-format` → `cli-lint`).

## Consequences

### Easier

- **.NET is gone from the repo.** No `setup-dotnet`, no NuGet cache, no SDK pin anywhere. The server-side stack is Go (backend + infra + CLI), with iOS (Swift) and web (TypeScript) as the only other ecosystems.
- **One toolchain for all Go.** Contributors and AI agents use the same `go` tooling, the `go-coding-standards` skill, and the `go-tdd-worker` for the CLI as for the API.
- **The CLI's API contracts live in one language** as the server they target, so a contract change is a single-language change.

### Harder

- **TUnit coverage was reconstructed, not carried over.** The .NET tests (`ArgParserTests`, `ConfigTests`, `GenerateOfferCodesCommandTests`) were re-expressed as Go table-driven tests, plus new `httptest`-backed integration tests for the success/error/pagination paths the .NET suite did not cover. Parity rests on those tests rather than a byte-diff against the old binary.
- **A second CLI language exists in git history.** Anyone reading pre-2026-06-19 `cli/` history sees C#; the live program is Go.

## See also

- [ADR 0028 — Migrate the backend from .NET to Go](0028-migrate-backend-from-dotnet-to-go.md)
- [ADR 0029 — Migrate the Pulumi infrastructure from .NET to Go](0029-migrate-infrastructure-from-dotnet-to-go.md) — named `/cli` as the last .NET component
- [ADR 0015 — CI/CD pipeline and deployment strategy](0015-cicd-pipeline-and-deployment-strategy.md) — the PR-gate jobs this updates
