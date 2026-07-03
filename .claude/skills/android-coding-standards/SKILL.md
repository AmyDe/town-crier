---
name: android-coding-standards
description: "MUST consult before writing ANY Kotlin or Android code. Enforces idiomatic, modern Kotlin for the Town Crier Android app (/mobile/android): four Gradle modules (domain/data/presentation/app) with a strict dependency rule, coroutines + Flow with structured concurrency, single-activity Jetpack Compose with unidirectional data flow, manual constructor injection at a composition root (no Hilt/Koin), hand-rolled OkHttp ApiClient + kotlinx.serialization (no Retrofit), DataStore (no Room), and hand-written fakes with JUnit 5 + kotlinx-coroutines-test + Turbine (no MockK/Mockito). Trigger whenever the user asks you to write, scaffold, refactor, review, lint, or test any .kt or .kts file, Gradle build script, or version catalog under mobile/android — including ViewModels, composables, repositories, DTOs, domain entities or value classes, fakes/fixtures, coroutine/Flow code, or ktlint/detekt config. Even for a seemingly trivial Kotlin change, check this skill first — it encodes pre-resolved project decisions (epic #770) that deliberately differ from mainstream Android tutorials. Do NOT use for iOS/Swift, Go, React/web, Pulumi, GitHub Actions, or backend work."
---

# Android Coding Standards

Idiomatic, modern Kotlin for the Town Crier Android app (`/mobile/android`). Write Kotlin the way JetBrains and the Android teams write it — the kotlinx libraries, the official Kotlin coding conventions, the Compose API guidelines — not Go, Swift, or C# transliterated into Kotlin. **The single overriding rule: if a pattern would look out of place in a kotlinx library or an official Compose sample, don't use it.** This repo's culture (pure domain, hand-written fakes, manual wiring, thin dependencies) is expressed Kotlin-natively: default arguments instead of builders, sealed hierarchies instead of error enums-plus-flags, `internal` instead of ceremony. The stack choices below were pre-resolved in epic #770 with rationale — apply them, don't relitigate them per bead. Read this core first; pull the matching reference when the bead touches that area.

## Architecture (always applies)

- **Four Gradle modules under `mobile/android/`, mirroring the iOS package split.** `:domain` = pure Kotlin/JVM — entities, value classes, repository interfaces; may import only the stdlib, `kotlinx-coroutines-core`, and `java.time`. `:data` = ApiClient over OkHttp, kotlinx.serialization DTOs, DataStore, Auth0 SDK; depends on `:domain`. `:presentation` = Compose UI, ViewModels, design system, navigation; depends on `:domain` ONLY — never `:data`. `:app` = composition root + manifest; the only module that sees everything. All versions pinned in `gradle/libs.versions.toml`.
- **Package by feature inside each module** (`watchzones`, `applications`, `auth`), never by kind — no `models/`, `utils/`, `repositories/` packages. **Default to `internal`** in library modules; `public` is the module's deliberate API surface, not the compiler default you forgot to narrow.
- **Rich domain models.** Behaviour lives on the type, not in ViewModels or "service" classes. `@JvmInline value class` for IDs and domain primitives; `init { require(...) }` for programmer-error invariants; smart constructors returning a sealed result for user-input validation. Repository interfaces are declared in `:domain`, implemented in `:data`, and named for what distinguishes them (`HttpWatchZoneRepository`, `InMemoryApplicationCache`) — never `Impl`.
- **Manual constructor injection, wired once in a composition root in `:app`** — no DI framework. The graph is small, the wiring stays greppable, and a composition-root smoke test replaces framework validation.
- **Coroutines exclusively.** `suspend` for one-shot work, `Flow` for streams; structured concurrency — every coroutine has an owning scope. Suspend functions are **main-safe**: `withContext` over an injected dispatcher at the lowest level that actually blocks; ViewModels never mention `Dispatchers`.
- **Single-activity Jetpack Compose + Navigation Compose.** No Fragments, no XML. Unidirectional data flow: each screen is a ViewModel exposing one `StateFlow<UiState>` plus stateless composables; events travel up as lambdas; navigation decisions live in the NavHost layer, never in ViewModels. Consult the `design-language` skill for tokens, theming, and Material 3 mapping before writing any UI.
- **Data access:** hand-rolled `ApiClient` over OkHttp; kotlinx.serialization DTOs private to `:data`, mapped explicitly to domain types; timestamps decoded per-field through the shared `DotNetTimeParser` port. Persistence is DataStore (device latches) + an in-memory TTL cache. Time comes from an injected `java.time.Clock`.

## Test conventions (always applies)

- **JUnit 5 on the JVM** with `kotlin.test` assertions, `kotlinx-coroutines-test` (`runTest`), and Turbine for Flow assertions. TDD Red-Green-Refactor; ViewModels, use cases, and domain rules are the primary units. **No Robolectric** — if a unit needs Android to be testable, redesign it behind an interface; real-device behaviour is verified on the emulator (mobile-mcp).
- **Hand-written fakes only** (`FakeWatchZoneRepository`), living in the test source set beside their tests; promote to a `testFixtures` source set only when a second module needs them. State-based first (in-memory backing you assert against), with `<method>Calls` recording lists where the interaction itself is the behaviour, and a configurable failure per method. No mocking libraries.
- **Fixtures are top-level factory functions with default arguments** (`fun aWatchZone(name: String = "Home") = …`) — Kotlin default args make builder classes redundant.
- **Test names are backtick sentences stating behaviour** — `` fun `redeeming a lapsed grant reactivates the tier`() ``. One test file per behavioural concern. The main dispatcher is swapped via the shared JUnit 5 `MainDispatcherExtension`.

## Forbidden

- Hilt, Koin, Dagger, kotlin-inject, `javax.inject` (manual DI at the composition root).
- Retrofit; Gson/Moshi (hand-rolled ApiClient; kotlinx.serialization only).
- Room (DataStore + in-memory TTL cache is the decided persistence — verified iOS parity).
- MockK, Mockito, any mocking library; Robolectric.
- RxJava; `LiveData`; `AsyncTask` (coroutines + Flow).
- Fragments, XML layouts, view/data binding, `findViewById`.
- `!!` in production code; `lateinit var` outside framework-imposed cases.
- `GlobalScope`; `runBlocking`; `Dispatchers.*` referenced inside ViewModels.
- `runCatching` or blanket `catch (e: Exception)` around suspend calls (swallows `CancellationException`).
- Exposing mutable state: `MutableStateFlow`, `MutableList`, `var` in a public API.
- `Base*` super-classes (BaseViewModel, BaseActivity); inheritance where composition fits.
- `Util`/`Helper`/`Manager` grab-bags; static-holder classes (Kotlin has top-level functions).
- Builder classes for test data (default arguments instead).
- `java.util.Date`/`Calendar`/`SimpleDateFormat` (use `java.time`); calling `Instant.now()`/`System.currentTimeMillis()` in domain logic (inject `Clock`).
- `I` prefix on interfaces; `Impl` suffix on implementations.
- `open` classes without a documented inheritance need (Kotlin is final by default — keep it).
- Logging PII or secrets; any analytics/tracking SDK (the public stance is "we do not track").

## References (load on demand)

- `references/architecture-and-modules.md` — read when scaffolding a module or feature, wiring the composition root, modelling a domain type, or deciding visibility/placement.
- `references/kotlin-idiom.md` — read when unsure of language-level style: null-safety, sealed hierarchies, data/value classes, scope functions, error handling, collections.
- `references/coroutines-and-flow.md` — read when writing any async code: scopes, main-safety, dispatcher injection, StateFlow patterns, cancellation.
- `references/compose-ui.md` — read when writing any composable, screen, UiState, or ViewModel state: UDF, state hoisting, previews, theming hookup.
- `references/data-access.md` — read when the bead touches the ApiClient, DTOs/serialization, DataStore, caching, or a repository implementation.
- `references/testing.md` — read when writing any test, fake, or fixture (runTest, Turbine, MainDispatcherExtension, fake/fixture conventions).
- `references/workflow-and-naming.md` — read when running build/test/lint, bootstrapping ktlint/detekt from `assets/`, editing the version catalog, or naming anything.
