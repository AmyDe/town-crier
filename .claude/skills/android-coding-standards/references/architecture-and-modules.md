# Architecture & Modules (reference)

Read when scaffolding a module or feature, wiring the composition root, modelling a domain type, or deciding visibility/placement. The core (`SKILL.md`) states the rules; this file is the rationale and the worked shapes.

## Module graph

```
mobile/android/
├── settings.gradle.kts, gradle/libs.versions.toml   # version catalog, everything pinned
├── domain/          # pure Kotlin/JVM (`kotlin("jvm")`) — zero Android dependencies
├── data/            # Android library — ApiClient, DTOs, DataStore, Auth0
├── presentation/    # Android library — Compose UI, ViewModels, design system, navigation
└── app/             # Android application — composition root, manifest, flavors
```

Allowed dependencies (anything not listed is a review flag):

| Module | May depend on |
|---|---|
| `:domain` | Kotlin stdlib, `kotlinx-coroutines-core`, `java.time` — nothing else. No `android.*`, no `androidx.*`, no serialization, no HTTP. |
| `:data` | `:domain`, OkHttp, kotlinx.serialization, DataStore, Auth0 SDK |
| `:presentation` | `:domain`, Compose/Material 3, `androidx.lifecycle` (ViewModel), Navigation Compose, maps-compose. **Never `:data`.** |
| `:app` | everything — it is the only module that may see the whole graph |

Why `:domain` is a plain `kotlin("jvm")` module and not an Android library: the compiler enforces purity for free (no `android.*` on the classpath at all), and its tests run as fast JVM tests with no Android toolchain in the loop. Repository interfaces live here and use `suspend`/`Flow` in their signatures — that is why `kotlinx-coroutines-core` is a permitted domain dependency; it is a language-level concurrency vocabulary, not a framework. `java.time` is safe everywhere because minSdk is 26 (epic #770) — no core-library desugaring needed.

Turn on `kotlin { explicitApi() }` for `:domain` and `:data` — it makes an unmarked-public declaration a compile error, which is the mechanical enforcement of the `internal`-by-default rule. `:presentation` skips it (explicit-API mode fights Compose ergonomics); there, `internal` stays a review discipline.

## Package-by-feature

Inside each module, slice by feature, not by kind:

```
domain/src/main/kotlin/uk/towncrierapp/domain/
├── watchzones/      # WatchZone, Radius, WatchZoneRepository, quota rules
├── applications/    # PlanningApplication, ApplicationStatus, browsing ports
├── subscriptions/   # Tier, EntitlementMap, tier max-merge
└── auth/            # Session, SessionStore port
```

A feature package owns its entities, its ports, and its rules. Promote a type to a shared package only when a *second* feature genuinely needs it — not pre-emptively.

**Visibility:** Kotlin's default is `public`, which is the wrong default for a library module. Declare `internal` unless the type is deliberately part of the module's API (domain entities and ports are; DTOs, parsers, and cache internals are not). `internal` is the Kotlin-native encapsulation tool — use it instead of directory conventions or naming ceremony.

## Domain modelling

Behaviour on the type; two distinct validation channels:

```kotlin
// Typed primitives: value classes, zero runtime cost.
@JvmInline
value class WatchZoneId(val value: String)

// Value classes carry invariants too. Programmer-error invariant: fail fast
// and loudly — a caller passing 0 is a bug, not an outcome to model.
@JvmInline
value class Radius(val metres: Int) {
    init {
        require(metres in 1..MAX_METRES) { "radius must be 1..$MAX_METRES m, was $metres" }
    }

    companion object {
        const val MAX_METRES = 10_000
    }
}

// Expected outcome the caller must handle: a sealed result, not an exception.
sealed interface PostcodeLookup {
    data class Found(val coordinate: Coordinate, val displayName: String) : PostcodeLookup
    data object NotFound : PostcodeLookup
}

// Rich entity: the rule lives here, not in a ViewModel.
data class WatchZone(
    val id: WatchZoneId,
    val name: String,
    val centre: Coordinate,
    val radius: Radius,
) {
    fun withRadiusCappedFor(tier: Tier): WatchZone =
        copy(radius = Radius(minOf(radius.metres, tier.maxRadiusMetres)))
}
```

The distinction matters: `require`/`check` (→ `IllegalArgumentException`/`IllegalStateException`) are for conditions that indicate a bug and should crash a debug build; sealed hierarchies are for outcomes that are part of the feature's contract (invalid postcode, quota exceeded, insufficient entitlement) and must be handled exhaustively by the caller.

## Composition root

`:app` hand-wires the graph top-to-bottom in one place — a plain class, constructed once in `Application.onCreate`. The Android-touching leaves come in **through the constructor** (as their domain interfaces), so the container itself stays a pure-JVM type:

```kotlin
class AppContainer(
    baseUrl: String,                        // BuildConfig.API_BASE_URL
    sessionStore: SessionStore,             // Auth0SessionStore(context) in prod
    latchStore: DeviceLatchStore,           // DataStore-backed in prod
    private val clock: Clock = Clock.systemUTC(),
    callFactory: Call.Factory = OkHttpClient.Builder().build(),
) {
    private val json = Json { ignoreUnknownKeys = true }
    private val apiClient = ApiClient(baseUrl, callFactory, json, sessionStore)

    val watchZoneRepository: WatchZoneRepository = HttpWatchZoneRepository(apiClient)
    val applicationRepository: ApplicationRepository = OfflineAwareApplicationRepository(
        remote = HttpApplicationRepository(apiClient),
        cache = InMemoryApplicationCache(clock),
    )
    // …one val per port the UI layer consumes
}
```

ViewModels get their dependencies through plain constructors; the NavHost supplies them with the `viewModel(factory = …)` overload (`viewModelFactory { initializer { … } }`) reading from the container. No service locator, no `@EnvironmentObject`-style ambient graph — if a screen's dependency list gets long, that is design feedback, not a reason for a framework.

**Composition-root smoke test:** because the Android leaves are constructor parameters, a plain JVM test can construct the entire graph — `AppContainer("https://api.example.test", FakeSessionStore(), FakeDeviceLatchStore())` — and touch every exposed `val`, so wiring drift fails in `./gradlew test`, not on first launch. This is the replacement for a DI framework's graph validation — keep it green.

## Where things go — quick placement table

| Thing | Module | Notes |
|---|---|---|
| Entity, value class, sealed outcome | `:domain` | behaviour on the type |
| Repository interface (port) | `:domain` | consumer vocabulary, domain types only |
| Sealed `ApiException` hierarchy | `:domain` | thrown by `:data`, caught by `:presentation` — only `:domain` is visible to both |
| Repository implementation | `:data` | named `Http*`/`DataStore*`/`InMemory*`, `internal` where possible |
| DTO + mapper | `:data` | `internal`; domain never sees a DTO |
| ViewModel, UiState, composable | `:presentation` | depends on `:domain` ports only |
| Design system component | `:presentation` | see `design-language` skill |
| Wiring, flavors, manifest, FCM service | `:app` | the only all-seeing module |
