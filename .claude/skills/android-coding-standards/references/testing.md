# Testing (reference)

Read when writing any test, fake, or fixture. TDD Red-Green-Refactor: the test exists before the implementation, fails for the right reason, then passes. ViewModels, use cases, and domain rules are the primary units; everything they need is reachable through constructor-injected interfaces, which is why no mocking library is needed — or allowed.

## The stack

| Concern | Tool |
|---|---|
| Runner | JUnit 5 (Jupiter) on the JVM. `:domain` (`kotlin("jvm")`) just sets `useJUnitPlatform()`. **AGP modules do not run Jupiter tests by default** — without `testOptions { unitTests.all { it.useJUnitPlatform() } }` in the module's build script, the build goes green with zero tests executed. That wiring is mandatory in every Android module with tests. |
| Assertions | `kotlin.test` (`assertEquals`, `assertIs<T>`, `assertFailsWith`) — first-party, no DSL |
| Coroutines | `kotlinx-coroutines-test`: `runTest`, `TestDispatcher`s |
| Flow assertions | Turbine (`flow.test { awaitItem() … }`) |
| Android framework | **Nothing.** No Robolectric, no instrumentation in the unit loop. If a type can't be tested without Android, put the Android bit behind a domain interface and fake it. Real-device flows are verified on the emulator via mobile-mcp. |

Test names are backtick sentences describing behaviour, from the caller's point of view:

```kotlin
@Test
fun `redeeming a lapsed grant reactivates the tier`() = runTest { … }
```

Keep suites small and cut by behavioural concern — either one file per concern (`WatchZonesViewModelRefreshTest`) or one file per subject with JUnit 5 `@Nested inner class Refresh { … }` groupings; both are acceptable, an 800-line flat test class is not.

## Fakes: hand-written, state-based first

A fake implements the domain port with a real (in-memory) behaviour, so most assertions read from **state**, not interactions:

```kotlin
class FakeWatchZoneRepository : WatchZoneRepository {
    val stored = mutableListOf<WatchZone>()
    var failWith: ApiException? = null

    val deleteCalls = mutableListOf<WatchZoneId>()

    override suspend fun zones(): List<WatchZone> {
        failWith?.let { throw it }
        return stored.toList()
    }

    override suspend fun delete(id: WatchZoneId) {
        failWith?.let { throw it }
        deleteCalls += id
        stored.removeAll { it.id == id }
    }
}
```

- Assert on outcomes (`assertEquals(emptyList(), fake.stored)`) wherever possible; keep `<method>Calls` recording lists only for interactions that *are* the behaviour under test (e.g. "does not call delete when confirmation is dismissed").
- `failWith` (or per-method variants) makes failure paths one line to arrange. Throwing the real sealed `ApiException` types keeps error-path tests honest.
- Fakes live in the test source set beside their tests. When a second module needs the same fake (presentation tests faking a domain port), promote it to `:domain`'s `testFixtures` source set via the `java-test-fixtures` plugin — the Gradle-native way to share test code without a "testutils" module. (That plugin is JVM-only; if an *Android* module ever needs to export fixtures, that's AGP's `android.testFixtures.enable` — rare here, since shared ports live in `:domain`.)

## Fixtures: factory functions with default arguments

```kotlin
fun aWatchZone(
    id: WatchZoneId = WatchZoneId("wz-1"),
    name: String = "Home",
    centre: Coordinate = Coordinate(51.5074, -0.1278),
    radius: Radius = Radius(500),
) = WatchZone(id, name, centre, radius)
```

Each test names only what it cares about: `aWatchZone(radius = Radius(5_000))`. This is the Kotlin-native replacement for builder classes and static fixture catalogues — do not write a `WatchZoneBuilder`. Fixtures follow the same placement rule as fakes (beside tests; `testFixtures` when shared), and previews reuse them.

## Testing ViewModels

ViewModels use `viewModelScope`, which targets `Dispatchers.Main` — swapped in tests by the shared JUnit 5 extension:

```kotlin
class MainDispatcherExtension : BeforeEachCallback, AfterEachCallback {
    override fun beforeEach(context: ExtensionContext) =
        Dispatchers.setMain(UnconfinedTestDispatcher())

    override fun afterEach(context: ExtensionContext) = Dispatchers.resetMain()
}
```

`UnconfinedTestDispatcher` executes eagerly, which suits state-machine tests (act, then assert the settled state) — but the eagerness trades away ordering fidelity: intermediate states are overwritten before anything can observe them, and launch-ordering bugs stay invisible. Any test asserting *intermediate* states or ordering must switch to `StandardTestDispatcher` + `advanceUntilIdle()` locally.

```kotlin
@ExtendWith(MainDispatcherExtension::class)
class WatchZonesViewModelRefreshTest {

    @Test
    fun `refresh replaces zones and clears a previous error`() = runTest {
        val repository = FakeWatchZoneRepository().apply { zones += aWatchZone() }
        val viewModel = WatchZonesViewModel(repository)

        viewModel.refresh()

        val state = viewModel.uiState.value
        assertEquals(listOf(aWatchZone()), state.zones)
        assertNull(state.error)
    }

    @Test
    fun `refresh failure surfaces a retryable error and keeps stale zones`() = runTest {
        val repository = FakeWatchZoneRepository().apply { failWith = ApiException.Network(IOException()) }
        …
    }
}
```

Use Turbine when the *sequence* of emissions is the contract (loading → loaded), `uiState.value` when only the settled state matters — asserting intermediate states everywhere makes tests brittle against harmless refactors. Caveat for sequence tests: `StateFlow` **conflates** — under the eager Unconfined main dispatcher the `isLoading = true` emission is typically overwritten before any collector sees it. Observing a loading→loaded sequence needs `StandardTestDispatcher`, collection started (via Turbine) *before* the act, and stepping the scheduler; if an intermediate state still gets conflated away, assert the settled state instead of fighting the conflation.

## What not to test

- Compose layout/rendering — previews + emulator verification carry that; there is no unit-level substitute worth its maintenance cost day-1.
- The Android SDK, OkHttp, or kotlinx.serialization themselves — test *your* mappers, parsers, and policies (`DotNetTimeParser` round-trips, error-envelope normalisation, TTL expiry with a fixed `Clock`).
- Trivial data classes and delegation-only code. A test that cannot fail for a real reason is noise.
