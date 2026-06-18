# 0029. Migrate the Pulumi infrastructure program from C#/.NET to Go

Date: 2026-06-18

## Status

Accepted

Reverses the IaC language choice in [ADR 0001](0001-initial-tech-stack.md) (item 7) and the `/infra` internal structure in [ADR 0002](0002-monorepo-structure.md). Completes the language consolidation begun by [ADR 0028](0028-migrate-backend-from-dotnet-to-go.md), which migrated the backend to Go but explicitly left `/infra` on .NET.

## Context

[ADR 0001](0001-initial-tech-stack.md) chose **Pulumi in .NET 10 (C#)** for infrastructure-as-code, with the stated rationale that it shared "the same language and tooling (C#/.NET) as our backend," promoting code reuse and a unified developer experience.

[ADR 0028](0028-migrate-backend-from-dotnet-to-go.md) migrated the backend API and worker to Go and deleted `/api`, but deliberately kept Pulumi on .NET: *"Pulumi infrastructure stays in C#/.NET (`/infra`). It is not migrated — it provisions Azure resources and is unaffected by the API language."*

That left the original "same language as the backend" rationale inverted. After the backend went Go, .NET survived in only two places — `/infra` (Pulumi) and `/cli` (a self-contained utility) — and infra was the odd one out: the request-serving backend, the IaC, and the web bundle were three different ecosystems, with .NET kept alive largely for one Pulumi program. The cost showed up as ongoing toolchain friction:

- A separate `setup-dotnet` path and SDK pin (`infra/global.json`) had to be maintained in CI purely for infra after `/api` was deleted. That pin originally lived at `api/global.json` and broke every infra build the moment `/api` was removed — a hazard that only existed because infra was a lone .NET program in an otherwise Go/TypeScript tree.
- Contributors and AI agents needed the .NET toolchain to touch infrastructure, even though everything they were provisioning (Container Apps, Cosmos, Service Bus) was now fronted by Go services.

Pulumi has a first-class Go runtime and a maintained `pulumi-azure-native-sdk` for Go, so porting infra to Go removes .NET from the IaC path and realigns the codebase on two server-side languages (Go for backend + infra, .NET only for the CLI).

## Decision

**Port the Pulumi program from C#/.NET to Go (`/infra`).** Tracked as bead **tc-ycsn** (PR #504).

The port is a **zero-diff migration**: the Pulumi project name (`town-crier`), every logical resource name, the resulting URNs, and the azure-native provider version (**3.16.0**) are all preserved so the existing `shared`, `dev`, and `prod` stack states match exactly and **no resource is replaced**. `pulumi preview` against all three stacks showed no changes after the port.

### What changed

| Concern | Was (.NET) | Now (Go) |
|---------|-----------|----------|
| Language / runtime | C#/.NET 10, `runtime: dotnet` | Go 1.26, `runtime: go` |
| Provider SDK | `Pulumi.AzureNative` (.NET) | `github.com/pulumi/pulumi-azure-native-sdk/*/v3` @ 3.16.0 |
| Pulumi SDK | `Pulumi` NuGet | `github.com/pulumi/pulumi/sdk/v3` |
| Layout | `src/town-crier.infra/` (`SharedStack.cs`, `EnvironmentStack.cs`) + `tests/town-crier.infra.tests/` | flat Go: `main.go` dispatching to `shared.go` / `environment.go` |
| SDK pin | `infra/global.json` | `infra/go.mod` (`go 1.26`) — `global.json` removed |

`main.go` reads the `environment` config value and dispatches to `runSharedStack` (the `shared` stack) or `runEnvironmentStack` (`dev` / `prod`), mirroring the previous `SharedStack`/`EnvironmentStack` split.

### What was kept

- **Pulumi remains the IaC tool**, with the same Pulumi Cloud state backend, the same three stacks (`shared`, `dev`, `prod`), and the same OIDC-federated Azure auth in CI ([ADR 0015](0015-cicd-pipeline-and-deployment-strategy.md), [ADR 0017](0017-cd-dev-always-deploy-and-pr-gate-no-promote.md)).
- **All provisioned resources are unchanged** — the migration is a language port, not a resource change. The Cosmos containers ([ADR 0008](0008-cosmos-db-data-model.md)), Service Bus polling infra ([ADR 0024](0024-service-bus-only-polling.md)), the shared OTel agent config ([ADR 0027](0027-go-api-observability-via-aca-otel-agent.md)), ACS email ([ADR 0020](0020-email-notifications-via-acs.md)), APNs secrets ([ADR 0026](0026-apns-direct-http2.md)), and the operational dashboard ([ADR 0018](0018-opentelemetry-observability-with-azure-monitor.md)) are all still provisioned by the same logical resources.
- **`/cli` stays on .NET.** It is the sole remaining .NET component in the repo.

### Prose references in earlier ADRs

ADRs written during the .NET era refer to `infra/SharedStack.cs` and `infra/EnvironmentStack.cs` (notably [ADR 0027](0027-go-api-observability-via-aca-otel-agent.md) and [ADR 0018](0018-opentelemetry-observability-with-azure-monitor.md)). Those files are now `infra/shared.go` and `infra/environment.go` respectively. Treat the `.cs` filenames in older ADRs as historical pointers to the equivalent Go file — the resource definitions they describe are unchanged.

## Consequences

### Easier

- **.NET is confined to `/cli`.** The request-serving backend and the IaC are now both Go; only the CLI utility still needs the .NET toolchain. CI no longer runs `setup-dotnet` for infra, and the `infra/global.json` cross-directory SDK-pin hazard is gone.
- **One fewer ecosystem on the server side.** Contributors and AI agents touching infrastructure use the same Go idioms and tooling as the backend.
- **Dependabot manages infra Go modules** the same way it manages the backend's (visible as the `chore(deps)` bumps against `/infra` in git history).
- **The original ADR 0001 rationale is restored, not abandoned** — IaC again shares the backend's language; the backend's language simply changed.

### Harder

- **The .NET infra unit-test project is gone.** [ADR 0002](0002-monorepo-structure.md) provisioned `tests/town-crier.infra.tests/` for infrastructure policy/config unit tests; the Go port did not carry these over (no `*_test.go` files in `/infra`). Validation now rests on the zero-diff `pulumi preview` gate in CI rather than C# policy unit tests. Re-introduce Go-based infra tests if policy assertions become valuable again.
- **azure-native Go SDK ergonomics differ.** Some azure-native sub-modules are split into separate Go modules and have token/typing quirks the C# SDK hid; these were worked through during the port but are a per-resource cost when adding new resource types.
- **Two Pulumi-capable languages now exist in repo history.** Anyone reading pre-2026-06-18 infra history sees C#; the live program is Go.

## See also

- [ADR 0001 — Initial tech stack](0001-initial-tech-stack.md) — the Pulumi/.NET IaC choice this reverses (item 7)
- [ADR 0002 — Monorepo structure](0002-monorepo-structure.md) — the `/infra` .NET layout this replaces
- [ADR 0028 — Migrate the backend from .NET to Go](0028-migrate-backend-from-dotnet-to-go.md) — the backend migration that left infra as the last server-side .NET component
