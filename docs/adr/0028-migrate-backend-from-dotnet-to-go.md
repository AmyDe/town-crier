# 0028. Migrate the backend API and worker from .NET to Go (and decommission .NET)

Date: 2026-06-15

## Status

Accepted

Reverses the backend-technology choice in [ADR 0001](0001-initial-tech-stack.md) and the `/api` internal structure in [ADR 0002](0002-monorepo-structure.md). Completes the migration whose observability slice was recorded in [ADR 0027](0027-go-api-observability-via-aca-otel-agent.md).

## Context

[ADR 0001](0001-initial-tech-stack.md) selected **.NET 10 (ASP.NET Core) with Native AOT** for the backend API, hosted on Azure Container Apps. [ADR 0002](0002-monorepo-structure.md) gave that backend a hexagonal `src/town-crier.{domain,application,infrastructure,web}` layout under `/api`, with a sibling `town-crier.worker` console app for polling and digests. That stack shipped the product through to production.

Over the life of the .NET backend, the Native AOT constraint imposed steady friction that ADR 0001 anticipated but underweighted:

- **Reflection-free everything.** Every dependency had to be AOT-clean. This forced bespoke choices repeatedly — a hand-rolled `CosmosRestClient` instead of the Cosmos SDK ([ADR 0001](0001-initial-tech-stack.md) 2026-03-31 amendment), manual OpenTelemetry SDK wiring because the Azure Monitor distro uses reflection ([ADR 0018](0018-opentelemetry-observability-with-azure-monitor.md)), and `[JsonSerializable]` source-generated contexts on every payload type.
- **A documented correctness cost.** The .NET request-binding path bound DTOs case-sensitively (PascalCase), so the camelCase bodies sent by the iOS app and Apple's App Store notifications silently failed to bind — breaking subscription verify and webhook handling until worked around.
- **Cold starts and cost.** Scale-to-zero was the cost story in ADR 0001, but the polling `BackgroundService` forced `MinReplicas=1` until polling was extracted ([ADR 0019](0019-extract-polling-to-container-apps-job.md)), and Native AOT cold-start wins did not fully materialise for the API's actual traffic shape.

Go (`net/http`, `log/slog`, the official `azcosmos`/`azservicebus` SDKs) removes the AOT tax entirely: no source generators, no reflection bans, first-class Azure SDKs, fast cold starts and small images by default, and a smaller dependency tree than the .NET app had grown. The `go-coding-standards` skill already codified the target idioms (flat feature-sliced `internal/`, consumer-side interfaces, hand-written fakes, hardened HTTP server profile).

The migration ran as epic **tc-7g3i** (GH #418) in three phases:

1. **Phase 1** — build a contract-identical Go API (`api-go/`), deploy it scale-to-zero alongside the live .NET app, and verify parity with a contract-diff suite.
2. **Cutover** — flip Cloudflare DNS for `api.towncrierapp.uk` and `api-dev.towncrierapp.uk` to the Go app (2026-06-14), then restore the warm-instance `MinReplicas` policy on the Go prod app.
3. **Phase 3 decommission** (epic **tc-tbyp**, 2026-06-15) — remove the deployed .NET API and worker, delete the `/api` source tree, and make Go own the `api` domain unconditionally in infra and CI.

[ADR 0027](0027-go-api-observability-via-aca-otel-agent.md) recorded only the observability mechanism for the parallel-run window and explicitly described the migration as in-flight. The migration decision and the decommission themselves were never given an ADR. This ADR closes that gap.

## Decision

**The backend API and background worker are written in Go (`api-go/`). The .NET API and worker are decommissioned and deleted.** Go owns the `api` and `api-dev` domains unconditionally.

### What changed

| Concern | Was (.NET) | Now (Go) |
|---------|-----------|----------|
| Language / runtime | .NET 10, ASP.NET Core, Native AOT | Go 1.26 |
| HTTP | ASP.NET Core minimal API + middleware pipeline | stdlib `net/http`, hand-wired middleware |
| Logging | `ILogger` → Azure Monitor exporter | `log/slog` (JSON stdout + OTel bridge) |
| Cosmos access | bespoke `CosmosRestClient` over the REST API | official `azcosmos` SDK (v1.4.2) |
| Service Bus | `Azure.Messaging.ServiceBus` | official `azservicebus` SDK (v1.10.0) |
| JSON | `System.Text.Json` source-generated contexts | stdlib `encoding/json` (reflection is fine in Go) |
| Layout | hexagonal `src/town-crier.{domain,application,infrastructure,web}` under `/api` | flat, feature-sliced `internal/<feature>` under `/api-go` |
| Worker | `town-crier.worker` console app, `WORKER_MODE` switch | `api-go/cmd/worker`, same `WORKER_MODE` contract (`poll-sb`, `poll-bootstrap`, `digest`, `hourly-digest`, `dormant-cleanup`) |
| Observability | in-process Azure Monitor OTel exporter | OTLP → ACA managed-environment agent ([ADR 0027](0027-go-api-observability-via-aca-otel-agent.md)) |

The Go `internal/` packages reimplement every feature the .NET backend served: `auth`, `applications`, `authorities`, `designations`, `watchzones`, `notifications`, `notificationstate`, `notifydispatch`, `polling`, `servicebus`, `digest`, `dormant`, `erasure`, `subscriptions`, `offercodes`, `devicetokens`, `apns`, `acsemail`, `geocoding`, `profiles`, `savedapplications`, `demoaccount`, `legal`, `admin`, `health`, `versionconfig`.

### What was kept

The migration is a **backend-language change only**. The following decisions are unchanged and remain in force:

- **Azure Container Apps** hosting, **Cosmos DB (Serverless)** as the data store ([ADR 0008](0008-cosmos-db-data-model.md)), **Auth0** for identity ([ADR 0007](0007-auth0-authentication.md)), **GitHub Actions** CI/CD ([ADR 0015](0015-cicd-pipeline-and-deployment-strategy.md), [0017](0017-cd-dev-always-deploy-and-pr-gate-no-promote.md)).
- **Pulumi infrastructure stays in C#/.NET** (`/infra`). It is not migrated — it provisions Azure resources and is unaffected by the API language. After `/api` was deleted, infra gained its own `infra/global.json` (the SDK pin previously lived at `api/global.json`).
- **The `/cli` .NET tool stays.** It is a separate self-contained utility, not part of the request-serving backend, and was out of scope for the migration.
- **iOS (Swift)** and **web (React/TypeScript)** clients are untouched — the Go API is contract-identical, so clients did not change.

### What the migration reimplements without re-deciding

The API behaviours documented in the following ADRs were **reimplemented in Go with identical external contracts**. Their *decisions* still hold; only the implementation language and the class/handler names referenced in their prose are historical (.NET):

- [0006](0006-planit-primary-data-provider.md) PlanIt polling • [0009](0009-notification-delivery-architecture.md) notification dispatch • [0010](0010-subscription-entitlement-flow.md) App Store entitlement flow • [0013](0013-govuk-planning-data-provider.md) Gov.uk designations • [0016](0016-test-infrastructure-strategy.md) in-memory test fakes • [0019](0019-extract-polling-to-container-apps-job.md) worker job model • [0020](0020-email-notifications-via-acs.md) ACS email • [0021](0021-resumable-pagination-cursor-for-planit-polling.md) resumable poll cursor • [0022](0022-offer-codes-for-subscription-grants.md) offer codes • [0023](0023-dormant-account-cleanup-for-uk-gdpr.md) dormant cleanup • [0024](0024-service-bus-only-polling.md) Service Bus-only polling • [0026](0026-apns-direct-http2.md) direct APNs HTTP/2.

These are **not** individually re-amended — treat this ADR as the umbrella note that their `.NET`-named internals now live in the corresponding `api-go/internal/<feature>` package. The polling algorithm, entitlement model, container/partition design, and delivery transports are unchanged.

### Legal copy canonical source

The Privacy Policy / Terms JSON canonical source moved from the deleted .NET API to `api-go/internal/legal/resources/{privacy,terms}.json`. The iOS bundle mirror and `scripts/check-legal-drift.sh` are unchanged.

## Consequences

### Easier

- **No Native AOT tax.** Reflection-based JSON, the official Azure SDKs, and ordinary library wiring are all available. The bespoke `CosmosRestClient` and the manual-vs-distro OpenTelemetry dilemma are gone.
- **Correct request binding.** Go's `encoding/json` is case-insensitive by default, so the PascalCase-only binding bug that broke camelCase iOS/Apple bodies on the .NET API cannot recur.
- **Smaller images, faster cold starts, lower memory.** Go's static binaries suit scale-to-zero ACA better than the .NET AOT image did in practice.
- **One backend language, fewer moving parts.** API and worker share one module and one idiom set; the `go-tdd-worker` and `go-coding-standards` skill own the whole backend.
- **Dead code removed.** Deleting `/api` and the .NET ContainerApp removed two container images' worth of build/deploy surface from CI and infra.

### Harder

- **Two backend languages in the repo are now three ecosystems total to keep current** — Go (api), C#/.NET (infra + cli), and TypeScript (web). Contributors need Go in addition to .NET and Swift. (Net simpler than before for the *serving* path, which is now single-language.)
- **History reads against the wrong language.** ADRs 0006–0026 describe .NET handlers/classes that no longer exist by those names. This ADR is the pointer that redirects them to `api-go/internal/<feature>`; readers must follow it rather than grepping for the old type names.
- **Pulumi/.NET and cli/.NET remain**, so the .NET toolchain is not fully retired from the repo — only from the request-serving backend.
- **Reflection is now permitted in the backend**, which removes a guardrail that kept the .NET dependency tree minimal. `go-coding-standards` substitutes its own minimal-dependency discipline.

## See also

- [ADR 0001 — Initial tech stack](0001-initial-tech-stack.md) — the .NET backend choice this reverses
- [ADR 0002 — Monorepo structure](0002-monorepo-structure.md) — the `/api` layout this replaces with `/api-go`
- [ADR 0018 — OpenTelemetry observability](0018-opentelemetry-observability-with-azure-monitor.md) — the .NET in-process telemetry decommissioned here
- [ADR 0027 — Go API observability via the ACA OTel agent](0027-go-api-observability-via-aca-otel-agent.md) — the observability slice of this migration
