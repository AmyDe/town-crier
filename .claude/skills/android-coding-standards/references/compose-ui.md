# Compose UI (reference)

Read when writing any composable, screen, UiState, or ViewModel-to-UI wiring. Follow the official Compose API guidelines; the shapes below are how they land in this codebase. For every visual decision — colors, type, spacing, components — consult the `design-language` skill first; this file covers structure, not styling.

## The screen pattern: Route + Screen

Every destination splits into a thin stateful *Route* and a stateless, previewable *Screen*:

```kotlin
@Composable
fun WatchZonesRoute(
    viewModel: WatchZonesViewModel,
    onZoneSelected: (WatchZoneId) -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    WatchZonesScreen(
        state = state,
        onZoneSelected = onZoneSelected,
        onDeleteZone = viewModel::deleteZone,
        modifier = modifier,
    )
}

@Composable
internal fun WatchZonesScreen(
    state: WatchZonesUiState,
    onZoneSelected: (WatchZoneId) -> Unit,
    onDeleteZone: (WatchZoneId) -> Unit,
    modifier: Modifier = Modifier,
) { /* pure function of state */ }
```

- State flows down as immutable parameters; events flow up as lambdas. The Screen knows nothing about ViewModels, navigation, or repositories — which is exactly what makes it previewable and reusable.
- Navigation lambdas (`onZoneSelected`) are supplied by the NavHost in the navigation layer. **ViewModels never navigate** — same division of labour as the iOS Coordinators, expressed the Compose-native way.
- `collectAsStateWithLifecycle()` (not `collectAsState()`) so collection stops when the UI is backgrounded.

## Routes and screen arguments

Destinations are **type-safe routes** — `@Serializable` objects/classes (Navigation 2.8+), which dovetails with the kotlinx.serialization dependency the data layer already mandates. No hand-rolled `"zoneDetail/{id}"` route strings, no manual argument parsing:

```kotlin
@Serializable data object WatchZones
@Serializable data class ZoneDetail(val zoneId: String)

// In the NavHost (navigation layer; the composition root supplies `container`):
composable<ZoneDetail> { backStackEntry ->
    val route = backStackEntry.toRoute<ZoneDetail>()
    ZoneDetailRoute(
        viewModel = viewModel(factory = viewModelFactory {
            initializer { ZoneDetailViewModel(container.watchZoneRepository, WatchZoneId(route.zoneId)) }
        }),
        onBack = navController::popBackStack,
    )
}
```

A detail ViewModel takes its subject's ID as a plain constructor parameter, read from the route in the initializer — no `SavedStateHandle` ceremony needed for nav arguments with this wiring. Reach for `createSavedStateHandle()` inside an `initializer` only for state that must survive **process death** beyond what the route already encodes; UiState itself is reloadable (server + cache), and in-progress text input is `rememberSaveable`'s job.

## UiState modelling

One immutable `UiState` type per screen, exposed as a single `StateFlow`:

- **Flat data class** when aspects overlap (showing data *while* refreshing, error banner *over* content): `data class WatchZonesUiState(val zones: List<WatchZone> = emptyList(), val isLoading: Boolean = false, val error: UiError? = null)`.
- **Sealed interface** when states are mutually exclusive by design (force-update gate: `Checking` / `UpToDate` / `MustUpdate`). Exhaustive `when` in the Screen then renders each.
- Fields hold what the Screen renders: domain values directly where they render as-is (a `List<WatchZone>` is fine), a small presentation item type where formatting is logic (dates, distances — the shared formatters run in the ViewModel/mapper, not in composables). No `MutableState`, no lambdas, no ViewModel references inside a UiState.
- **User-facing text is a resource, not a hardcoded string.** UiState carries either a `@StringRes` id or a tiny sealed `UiText` (resource-backed vs server-supplied dynamic text) that the Screen resolves via `stringResource`. ViewModels and domain code never bake in English copy — the copy-to-outcome mapping shown in `kotlin-idiom.md` belongs in the presentation layer, keyed off the sealed outcome.

## Composable API rules (the ones that get violated)

- PascalCase noun names (`StatusBadge`, not `renderStatusBadge`); a composable either **emits UI or returns a value, never both**.
- `modifier: Modifier = Modifier` is the **first optional parameter** of every public composable that emits UI, and it is applied exactly once, to the root layout. Accepting no modifier makes a component un-composable into new layouts; applying it twice duplicates padding/click areas.
- Parameter order: required data first, then `modifier: Modifier = Modifier` leading the optionals, then the remaining optional params, then trailing content lambdas (slot APIs). Variable content is a `@Composable () -> Unit` slot, not a boolean-and-string parameter explosion.
- Hoist state: components take `value` + `onValueChange`, they don't own state internally. `remember` internal state only for genuinely internal, throwaway UI mechanics (e.g. ripple, scroll positions via `rememberLazyListState`).
- `remember(key)` and `LaunchedEffect(key)` keys must include everything the computation reads — a stale-key bug is a stale-UI bug. `rememberSaveable` for anything that must survive process death (text input drafts).
- Lazy lists always set stable `key = { it.id.value }` — without keys, deletion animations and scroll anchoring silently break.

## Theming

- Everything renders inside `TownCrierTheme(mode)`, which maps the Town Crier tokens onto a Material 3 `ColorScheme` and exposes extended tokens (status colors, amberMuted, overlay) via a `CompositionLocal`. Feature code reads `MaterialTheme.colorScheme.*` and `TownCrierTheme.colors.*` — **never a raw hex, never a hardcoded dp for a token that exists**. Token values, the M3 role mapping, and the 4-way appearance mode live in the `design-language` skill and epic #770.
- Dynamic color is off by design; don't wire `dynamicColorScheme`.

## Previews

Every significant Screen and design-system component gets previews rendering realistic sample data, in at least light + dark. Preview data lives as `private val`/functions in the preview's own file (main source set) — it *cannot* reuse the test fixtures, because previews can't see test or `testFixtures` source sets; keep preview samples small and accept the duplication rather than contorting the build for sharing:

```kotlin
@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun WatchZonesScreenPreview() {
    TownCrierTheme {
        WatchZonesScreen(
            state = WatchZonesUiState(zones = listOf(previewZone)),
            onZoneSelected = {},
            onDeleteZone = {},
        )
    }
}

private val previewZone = WatchZone(WatchZoneId("wz-1"), "Home", Coordinate(51.5074, -0.1278), Radius(500))
```

Previews are the fastest feedback loop this codebase has (no emulator round-trip) — treat a Screen without previews as unfinished. Verification of real behaviour (navigation, permissions, deep links) happens on the emulator via mobile-mcp; Compose instrumentation tests are not part of the day-1 loop.

## Performance notes (worry in this order)

1. Correct state reads: read state as late/deep as possible so recomposition scopes stay small.
2. Immutable parameters: data classes of `val`s and read-only collections. With the current Compose compiler (strong skipping, default since Kotlin 2.0.20) skippability comes from instance equality, so `List` fields are fine; don't chase `@Stable`/`@Immutable` annotations or `kotlinx-collections-immutable` until a measured problem exists.
3. `derivedStateOf` only when deriving cheaper-changing state from something that changes often (scroll position → "show scroll-to-top affordance" boolean class of problem).

Premature recomposition golf is a smell; broken-by-design state (mutable lists in state, missing keys) is the actual failure mode to prevent.
