# 0007. Revisiting the Backend Language Choice (.NET AOT vs Go vs Rust)

Date: 2026-05-17

## Status

Superseded by ADR [0028](../adr/0028-migrate-backend-from-dotnet-to-go.md)

> Resolved 2026-06-15. The exploration below recommended Option C — pilot Go on the polling worker, then consider the API. That played out in full: the worker pilot succeeded (Phase 2), the API was migrated (Phase 1), and the .NET runtime was decommissioned (Phase 3, epic tc-tbyp). The decision and outcome are recorded in [ADR 0028](../adr/0028-migrate-backend-from-dotnet-to-go.md); the observability slice is in [ADR 0027](../adr/0027-go-api-observability-via-aca-otel-agent.md). This memo is retained as the rationale trail (the .NET-AOT-vs-Go-vs-Rust analysis ADR 0028 deliberately does not repeat). Rust was considered and rejected here, as 0028 reflects.

## Question

The backend is .NET 10 with Native AOT (per ADR 0001), chosen for cold-start performance and because the project owner had deep .NET expertise. Two things have changed since:

1. The owner no longer reads most of the generated code — AI agents do nearly all editing.
2. AOT has forced hand-rolled REST clients in place of mainstream Azure SDKs (Cosmos DB, Service Bus), because those SDKs are not AOT-compatible.

If AI is the primary reader/writer, and AI is reportedly stronger at Go and Rust than at AOT-constrained C#, is the original language choice still correct? Should we migrate, and if so, to what?

## Analysis

### Current state of `/api`

Measured 2026-05-17:

| Metric | Value |
|---|---|
| C# files | 639 |
| Lines of C# | 45,471 |
| Test files | 168 |
| Production projects | 5 (domain / application / infrastructure / web / worker) |
| Files touching `JsonSerializable` / source-gen contexts | 52 |
| Commits to `api/` in last 6 months | 245 (of 435 repo-wide — ~56%) |

### AOT tax actually paid

The hexagonal architecture absorbed most of the AOT pain into the infrastructure layer, but the following are non-trivial workarounds that wouldn't exist outside AOT:

- **No `Microsoft.Azure.Cosmos` package.** Repository implementations talk to Cosmos via REST (`CosmosThrottleRetryHandler`, REST-based clients). `Microsoft.Azure.Cosmos` is reflection-heavy and not AOT-safe.
- **No `Azure.Messaging.ServiceBus` package.** `ServiceBusRestClient` and `ServiceBusManagementClient` are hand-rolled over HTTP.
- **No EF Core, no MediatR, no AutoMapper.** Manual CQRS dispatch, manual mapping, manual serialization with `JsonSerializerContext` source generators (52 files touched).
- Survived as official SDKs: `Azure.Identity`, `Azure.Communication.Email`, `Microsoft.AspNetCore.Authentication.JwtBearer`, OpenTelemetry exporters.

The AOT tax is mostly *paid* — the painful work of writing REST clients exists and is tested. The recurring cost is smaller: adding a new Cosmos collection or ASB queue means extending the existing REST clients, not writing new ones. The serialization plumbing is the more ongoing irritant — every new DTO needs registering in a `JsonSerializerContext`.

### Why Native AOT was chosen (ADR 0001)

Cold-start performance on Azure Container Apps (which scales to zero). AOT binaries start in ~100ms vs ~2s for JIT'd .NET. This matters because:

- Polling worker runs as a Container Apps Job — every invocation is a cold start.
- API has `min_replicas = 0` in dev and (currently) prod, per the deferred-prod-cost decision (`project_aca_min_replicas_decision`).

Any replacement runtime must preserve sub-second cold starts.

### Candidate: Go

Strengths for this workload:

- **First-class Azure SDKs.** `github.com/Azure/azure-sdk-for-go/sdk/azcosmos`, `.../azservicebus`, `.../azcommunication`, `.../azidentity` are all official, actively maintained, and compile statically. No AOT analogue needed.
- **Cold starts come free.** Single static binary, no JIT warm-up, no AOT toolchain to configure.
- **Small language surface = AI-correct on first generation.** Go's syntactic minimalism (no generics-of-generics, no LINQ, no expression trees) is well-suited to AI codegen.
- **OpenTelemetry, gRPC, HTTP server, JSON — all in the standard library or first-party.** No `Microsoft.Extensions.*` ecosystem dance.
- **Pattern fit is decent.** Hexagonal architecture maps to packages-as-bounded-contexts. CQRS dispatch is a switch over command types — trivially expressed.

Weaknesses:

- **DDD with rich domain models is awkward.** No sealed classes, no private setters, no value-object equality. You get structs + constructors + receiver methods. The compile-time guarantees are weaker — invariants depend on convention.
- **Error handling is verbose.** Every call returns `(value, error)`. AI will write the boilerplate, but reviewing it is fatiguing if you ever do read the code.
- **Lower expressiveness than C#.** No LINQ, no pattern matching, no records. For domain modelling this is a real loss, not just stylistic.
- **No `dotnet test --filter` equivalent for parameterised tests with named cases.** Table-driven tests are the norm and are uglier than TUnit's data sources.

### Candidate: Rust

Strengths:

- **Strongest type system of the three.** Algebraic data types, exhaustive matching, no nullability bugs, ownership prevents whole classes of concurrency errors.
- **Cold starts even faster than Go.** Binary sizes are smaller; no GC.
- **Memory and CPU efficiency** that neither Go nor .NET match.

Weaknesses for this workload:

- **Azure SDK story is immature.** Official SDKs exist (`azure_*` crates) but are marked "alpha" and lag features. Community alternatives (`azure-sdk-for-rust`) are workable but rougher than Go's.
- **Async story is the rough one.** `async fn` + `tokio` + `Pin`/`Box`-juggling for stored futures is the most-complex async model of any mainstream language. AI handles it but iteration is slower (compiler errors are dense).
- **Borrow checker friction is unjustified here.** Town Crier is I/O-bound — HTTP, Cosmos, ASB. There's no CPU-bound or memory-bound hotspot that exploits Rust's strengths.
- **Compile times are 5-10× Go's**, which slows the AI-driven feedback loop.

Rust is the wrong tool for an I/O-bound CRUD service with polling. Save it for a component where the constraints justify the cost (none currently in scope).

### Cost of migration

A full rewrite estimate, assuming AI does ~85% of the work with senior review:

| Component | Effort estimate |
|---|---|
| Port domain layer (entities, value objects, domain services) | 1 week |
| Port application layer (command/query handlers) | 1.5 weeks |
| Port infrastructure (Cosmos + ASB + Auth0 + ACS + APNs + PlanIt) | 2 weeks (made easier by official SDKs in Go) |
| Port web layer (controllers, DI, middleware, auth) | 1 week |
| Port tests (168 files, including 5 integration test projects) | 1.5 weeks |
| Pulumi adjustments (container images, runtime probes, env vars) | 0.5 weeks |
| Parallel-run + cutover | 1 week |
| **Total** | **~8 weeks** of "no new features" |

This is wall-clock time with a single owner reviewing. iOS (1,778 files) and web (245 files) are unaffected because they consume the API over HTTP — but the API contract has to remain byte-equal during the cut.

### What's at risk by not switching

- **Every new Azure feature requires an AOT-compatibility audit.** When (not if) we add Azure Storage Queues, Blob Storage, Event Grid, etc., we'll be checking SDK source for reflection usage.
- **The JSON serialization context becomes unwieldy at scale.** 52 files already; growth is linear in DTO count.
- **Recruitment / handoff risk.** If the project ever needs another engineer, .NET-with-AOT is a narrower skill than .NET or Go alone.

### What's at risk by switching

- **8 weeks of feature freeze** at a moment Town Crier is approaching monetisation (paid users not yet onboarded; per `project_aca_min_replicas_decision`, we're holding back infra spend until revenue arrives).
- **Behavioural regressions during cutover** despite test parity — the integration surface is large (Cosmos consistency semantics, ASB lock semantics, APNs HTTP/2 framing, Auth0 JWT validation).
- **Throwing away tested code.** The hand-rolled REST clients have absorbed real production bug fixes (e.g. ASB lock-duration handling — see `asb-lockduration-capped-at-5m` memory). Those lessons would need re-discovery in Go.

## Options Considered

### Option A: Stay on .NET AOT, accept the recurring tax

No migration. Continue paying the serialization-context tax on new DTOs and the hand-rolled-REST-client tax on new Azure services. Lowest risk, lowest disruption.

### Option B: Full migration to Go

Rewrite the entire `/api` tree in Go. ~8 weeks of feature freeze. Strongest long-term ergonomics but highest short-term cost.

### Option C: Pilot Go on a single isolated component (recommended)

Rewrite **only the polling worker** (`town-crier.worker`) in Go. The polling worker is:

- The most isolated component — communicates with the rest of the system only via ASB messages and Cosmos writes.
- The heaviest user of the hand-rolled ASB + Cosmos REST clients.
- The component that benefits most from Go's official Azure SDKs.
- Small enough to rewrite in ~2 weeks.

If the pilot ships smoothly with measurably better velocity and no regressions, we have earned evidence to consider migrating the API. If it doesn't, we've lost two weeks on a contained experiment.

Success criteria for the pilot would need to be defined before starting — provisionally: (a) feature parity with the current worker, (b) cold-start time ≤ current .NET AOT, (c) PR-gate CI runs in ≤ current runtime, (d) AI-driven implementation feedback is "noticeably better" by the owner's subjective judgement.

### Option D: Migrate to Rust

Rejected. Wrong tool for an I/O-bound service. Azure SDK maturity, async complexity, and compile times all work against AI-driven iteration.

## Recommendation

**Option C: pilot Go on the polling worker.** It's the lowest-risk way to test the hypothesis that "AI + Go" is materially better than "AI + .NET AOT" for this codebase, on the component where the AOT tax bites hardest and the isolation is cleanest. The pilot is reversible, scoped, and avoids betting the whole backend on a hunch.

If the pilot succeeds, the API migration can be planned as a follow-up. If it fails or is ambiguous, we stay on .NET with high confidence rather than nagging doubt.

If this recommendation is adopted, the next step is a bead scoping the pilot with explicit success criteria, exit conditions, and a parallel-run plan.
