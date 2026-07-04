package uk.towncrierapp.domain.auth

/**
 * Port for authentication operations. Implementations handle Universal Login,
 * signed-in-session storage, and token refresh via Auth0. `login()`/`logout()`
 * intentionally take no Android `Context` — the `:data` implementation sources
 * the current foreground activity itself (see `CurrentActivityProvider` in
 * `:data`) so this port stays plain-Kotlin and mockable-by-fake.
 */
public interface AuthenticationService {
    /** Presents Universal Login and returns the resulting session. */
    public suspend fun login(): AuthSession

    /** Clears the current session (local credentials + remote SSO cookie). */
    public suspend fun logout()

    /**
     * Refreshes an expired (or about-to-expire) session. Throws
     * [java.io.IOException] when the failure is a transport-level network
     * problem (callers map that to [DomainError.NetworkUnavailable]); any
     * other failure means the refresh token is unusable and callers should
     * treat it as [DomainError.SessionExpired].
     */
    public suspend fun refreshSession(): AuthSession

    /** Returns the current session, or `null` if the user is not signed in. */
    public suspend fun currentSession(): AuthSession?
}
