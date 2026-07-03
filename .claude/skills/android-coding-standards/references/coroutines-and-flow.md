# Coroutines & Flow (reference)

Read when writing any async code. Coroutines are the concurrency model of this codebase — exclusively. The discipline that matters is *structured* concurrency: every coroutine has an owning scope whose lifecycle bounds it, so nothing leaks, and cancellation composes for free.

## Scopes and ownership

- ViewModels launch in `viewModelScope` — cancelled automatically when the screen's state holder dies.
- Repositories and data sources **do not launch coroutines**. They expose `suspend` functions and `Flow`s; the caller owns concurrency. A `launch` inside a repository is a fire-and-forget leak wearing a disguise.
- `GlobalScope` and `runBlocking` are banned outright (detekt enforces). The rare process-lifetime work (e.g. the FCM token refresh callback) gets an explicit, named, injected `CoroutineScope` created at the composition root — ownership is still explicit.
- Parallel decomposition uses `coroutineScope { }` + `async`: if one child fails, siblings cancel and the failure propagates. Use `supervisorScope` only when children are genuinely independent and you handle each failure inline.

## `suspend` vs `Flow`

- One value, on demand → `suspend fun`. Do not wrap a single value in a `Flow` to look reactive; a flow of exactly one emission is a suspend function with extra steps.
- A stream that changes over time (DataStore preference, cache invalidation ticks, UI state) → `Flow`/`StateFlow`.
- Cold flows in the data layer; hot state (`StateFlow`) only where something owns state — practically, ViewModels.

## Main-safety and dispatcher injection

Convention: **every `suspend` function is safe to call from the main thread.** Whoever does blocking work moves itself off the main thread with `withContext`, at the lowest level that actually blocks:

```kotlin
internal class JsonBodyReader(
    private val json: Json,
    private val io: CoroutineDispatcher, // wired to Dispatchers.IO at the composition root
) {
    suspend fun <T> read(body: ResponseBody, serializer: KSerializer<T>): T =
        withContext(io) { json.decodeFromString(serializer, body.string()) } // body.string() blocks
}
```

Two important non-cargo-cult notes:

- The OkHttp bridge (`Call.enqueue` via `suspendCancellableCoroutine`, see `data-access.md`) is already non-blocking — wrapping it in `withContext(Dispatchers.IO)` adds a thread hop and nothing else. DataStore is main-safe too. Add `withContext` where something *actually blocks* (large JSON decode, file IO), not as decoration.
- When a class does need a dispatcher, it takes `CoroutineDispatcher` as a constructor parameter, wired at the composition root. **ViewModels never reference `Dispatchers`** — they call main-safe suspend functions and stay dispatcher-agnostic, which is also what makes them trivially testable.

## ViewModel state: `StateFlow`, privately mutable

```kotlin
class WatchZonesViewModel(
    private val repository: WatchZoneRepository,
) : ViewModel() {

    private val _uiState = MutableStateFlow(WatchZonesUiState())
    val uiState: StateFlow<WatchZonesUiState> = _uiState.asStateFlow()

    fun refresh() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true) }
            try {
                val zones = repository.zones()
                _uiState.update { it.copy(isLoading = false, zones = zones, error = null) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: ApiException) {
                _uiState.update { it.copy(isLoading = false, error = e.toUiError()) }
            }
        }
    }
}
```

- Backing property `_uiState` private and mutable; the exposed `uiState` is read-only via `asStateFlow()`.
- Mutate with `update { it.copy(…) }` — atomic, and it keeps state transitions expressed as value transformations.
- Deriving state from an upstream flow? `flow.map { … }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), initialValue)` — the 5-second grace keeps state alive across rotation without running when nothing watches.

## Cancellation is sacred

Cancellation propagates as `CancellationException` through suspend calls. Anything that catches broadly will eat it and leave a zombie coroutine "running" after its scope died. The rules:

- Catch **specific** exception types. A specific catch (like the `ApiException` one above) can't swallow cancellation; the `catch (e: CancellationException) { throw e }` line is a defensive convention kept uniform so the guarantee survives someone later broadening the catch. Where a broad `catch (e: Exception)` is truly unavoidable at a boundary, that rethrow line stops being convention and becomes load-bearing.
- Never `runCatching` around suspend code (it catches everything, including cancellation).
- Long CPU loops (rare here) call `ensureActive()` or use `yield()` to stay cooperative.

## One-shot effects

Prefer modelling "effects" as state the UI reconciles (a `snackbarMessage: UiMessage?` field, cleared by an `onMessageShown()` event) over `SharedFlow` event buses — state survives recomposition, process death handling, and observer gaps; fired-and-forgotten events don't. Navigation is not an effect at all: it's a lambda passed down from the NavHost (`compose-ui.md`).
