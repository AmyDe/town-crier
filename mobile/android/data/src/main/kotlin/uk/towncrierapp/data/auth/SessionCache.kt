package uk.towncrierapp.data.auth

import uk.towncrierapp.domain.auth.AuthSession
import java.time.Clock
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.async
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * In-memory cache for the current [AuthSession] (iOS tc-3d7b). Concurrent
 * foreground bursts (a push tap arriving while the login check and the tier
 * resolver are already in flight) were each hitting
 * `SecureCredentialsManager` independently; this cache holds the most
 * recently loaded session while its access token has at least
 * [leadTimeSeconds] left before expiry, and single-flights concurrent cold
 * reads to one [loader] invocation via a shared [Deferred] so a four-way
 * burst issues at most one credentials-store read.
 *
 * [scope] is the rare case of an explicit, injected `CoroutineScope` for
 * process-lifetime work (android-coding-standards: coroutines-and-flow.md)
 * — the in-flight load must outlive whichever individual caller's coroutine
 * happens to create it, so every concurrent caller observes the same result.
 */
public class SessionCache(
    private val scope: CoroutineScope,
    private val leadTimeSeconds: Long = 60,
) {
    private val mutex = Mutex()
    private var cached: AuthSession? = null
    private var inFlight: Deferred<AuthSession?>? = null

    /** A cached session, or `null` if none is held or it's within [leadTimeSeconds] of expiry. Never triggers a load. */
    public suspend fun current(clock: Clock): AuthSession? = mutex.withLock { validOrNull(clock) }

    private fun validOrNull(clock: Clock): AuthSession? = cached?.takeIf { it.expiresAt.minusSeconds(leadTimeSeconds).isAfter(clock.instant()) }

    /** Returns the cached session if valid, otherwise runs [loader] — sharing one in-flight call across concurrent cold callers. */
    public suspend fun currentOrLoad(
        clock: Clock,
        loader: suspend () -> AuthSession?,
    ): AuthSession? {
        val deferred =
            mutex.withLock {
                validOrNull(clock)?.let { return it }
                inFlight ?: scope.async { loader() }.also { inFlight = it }
            }
        val result = deferred.await()
        mutex.withLock {
            if (inFlight === deferred) {
                cached = result
                inFlight = null
            }
        }
        return result
    }

    /** Stores [session] directly — used after a successful login or refresh round-trip. */
    public suspend fun store(session: AuthSession) {
        mutex.withLock { cached = session }
    }

    /** Drops the cached session and cancels any in-flight load — used on logout and unrecoverable refresh failures. */
    public suspend fun clear() {
        mutex.withLock {
            cached = null
            inFlight?.cancel()
            inFlight = null
        }
    }
}
