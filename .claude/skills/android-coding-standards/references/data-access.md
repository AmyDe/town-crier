# Data Access (reference)

Read when the bead touches the ApiClient, DTOs/serialization, DataStore, caching, or a repository implementation. The wire contract itself (endpoints, error shapes, pagination, retry policy) is specced in epic #770 and its child issues — this file covers *how* that contract is expressed in Kotlin.

## The ApiClient: hand-rolled, one seam

There is no Retrofit. The `ApiClient` in `:data` is a plain class over OkHttp that owns, in one visible place: base URL + standard headers, bearer-token attachment, the single 401-refresh-retry, `X-Next-Cursor` pagination, and 403 entitlement sniffing. The decision is deliberate (#770): this behaviour must be explicit and steppable, not scattered across annotations and a reflective proxy.

- The seam is OkHttp's own `Call.Factory` — production passes the real `OkHttpClient`; transport-level tests substitute a fake that returns canned `Response`s. Don't invent a bespoke transport interface when OkHttp already ships the right one.
- OkHttp also ships `Interceptor` and `Authenticator`, and using an `Interceptor` for the static headers is fine. The **401-refresh-retry stays explicit in `ApiClient`**, though — a deliberate deviation from the `Authenticator` idiom, because the retry-once policy mirrors the iOS `URLSessionAPIClient` line-for-line and must stay visible and steppable. Don't "fix" it into an `Authenticator`.
- Bridge OkHttp's callback API to coroutines once, with cancellation wired through. (If the pinned OkHttp version ships the official `okhttp-coroutines` artifact, prefer its `Call.executeAsync()` — it is exactly this bridge, maintained upstream.)

```kotlin
internal suspend fun Call.await(): Response = suspendCancellableCoroutine { cont ->
    enqueue(object : Callback {
        override fun onResponse(call: Call, response: Response) =
            // The onCancellation lambda closes the response if the coroutine was
            // cancelled between enqueue completing and resumption — without it,
            // that race leaks the connection.
            cont.resume(response) { _, resp, _ -> resp.close() }

        override fun onFailure(call: Call, e: IOException) = cont.resumeWithException(e)
    })
    cont.invokeOnCancellation { cancel() }
}
```

The cancellation wiring is the point: when the owning scope dies (user leaves the screen), the HTTP call is torn down and nothing leaks. This bridge is already non-blocking — no `withContext(Dispatchers.IO)` around it.

- Transport failures surface as a small sealed hierarchy the ViewModel layer can `catch` specifically — e.g. `ApiException.Unauthorized`, `ApiException.InsufficientEntitlement(required: Tier)` (drives paywall routing), `ApiException.NotFound`, `ApiException.Network(cause)`. Type the failure once; everything above stays exhaustive. **The sealed `ApiException` hierarchy is declared in `:domain`** (plain Kotlin, and it references domain types like `Tier`) — `:data` throws it, `:presentation` catches it; that's the only way the module dependency rule allows all three to see it.

## DTOs and kotlinx.serialization

- DTOs are `@Serializable`, `internal` to `:data`, named `*Dto`, and shaped exactly like the wire — the domain never sees one. Each DTO file carries its explicit mapper: `internal fun WatchZoneDto.toDomain(): WatchZone`. Mapping is where wire-shape weirdness (PascalCase legacy error bodies, explicit nulls) is absorbed.
- One shared `Json` instance from the composition root: `Json { ignoreUnknownKeys = true }`. Server adding a field must never crash a shipped client.
- **Dates cross the wire as `String`** and are parsed per-field by the shared `DotNetTimeParser` port inside the mappers — never a global serializer date strategy, which would mask the backend's fractional-seconds-only-when-non-zero behaviour. Date-only fields (`startDate`, `decidedDate`) are bare `yyyy-MM-dd` → `LocalDate`. There is exactly ONE parser; do not let a feature hand-roll its own (iOS made that mistake once — don't port it).
- Error bodies come in two shapes (PascalCase backfill and lowercase handler-written) — parse defensively for both inside the ApiClient; the sealed `ApiException` is the single normalised output.

## Repositories

- Interface in `:domain`, speaking domain types (`WatchZone`, `Cursor`, sealed outcomes). Implementation in `:data`, `internal` where wiring allows, named for what distinguishes it (`HttpWatchZoneRepository`).
- Repository functions are main-safe `suspend` functions; they do not launch coroutines and do not cache unless caching *is* the type's job:

```kotlin
internal class OfflineAwareApplicationRepository(
    private val remote: ApplicationRepository,
    private val cache: InMemoryApplicationCache,
) : ApplicationRepository {
    override suspend fun detail(id: ApplicationId): PlanningApplication =
        try {
            remote.detail(id).also(cache::put)
        } catch (e: CancellationException) {
            throw e
        } catch (e: ApiException.Network) {
            cache.get(id) ?: throw e   // stale-if-offline, matching iOS semantics
        }
}
```

## Local persistence: DataStore + in-memory cache (no Room)

- Persistent state is deliberately tiny (iOS-parity, verified): the device latches — onboarding-complete, review-prompt state, cached tier, appearance mode (`appearanceMode` key, same string values as iOS). Each lives behind a small domain port (`OnboardingStateStore`), implemented in `:data` over **Preferences DataStore**. Keys are declared in one place per store; reads are `Flow`-shaped (`data.map { it[KEY] }`), writes are `suspend`.
- The in-memory TTL cache is a plain class with an injected `Clock` (TTL 900 s, paged reads bypass — semantics per #770). Injected `Clock` is what makes expiry testable without sleeping.
- If a bead seems to need more persistence than this, stop and flag it — "add Room" is an architecture change, not an implementation detail.
