package uk.towncrierapp.data.auth

import com.auth0.android.Auth0
import com.auth0.android.authentication.AuthenticationException
import com.auth0.android.provider.WebAuthProvider
import com.auth0.android.result.Credentials
import kotlinx.coroutines.CancellationException
import uk.towncrierapp.domain.auth.AuthSession
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.UserProfile
import java.time.Clock

/**
 * Auth0 Android SDK implementation of [AuthenticationService].
 *
 * [auth0] is lazy: constructing a real `com.auth0.android.Auth0` touches
 * `android.text.TextUtils`, which crashes under plain JVM unit tests.
 */
public class Auth0AuthenticationService(
    private val config: Auth0Config,
    private val credentialsStore: CredentialsStore,
    private val activityProvider: CurrentActivityProvider,
    private val auth0: Lazy<Auth0>,
    private val sessionCache: SessionCache,
    private val clock: Clock = Clock.systemUTC(),
) : AuthenticationService {
    override suspend fun login(): AuthSession {
        val activity =
            activityProvider.currentActivity()
                ?: throw DomainError.AuthenticationFailed("No active screen to present sign-in")
        try {
            val credentials =
                WebAuthProvider
                    .login(auth0.value)
                    .withScope(Auth0Config.SCOPE)
                    .withAudience(config.audience)
                    .await(activity)
            credentialsStore.saveCredentials(credentials)
            val session = credentials.toAuthSession()
            sessionCache.store(session)
            return session
        } catch (e: CancellationException) {
            throw e
        } catch (e: AuthenticationException) {
            throw DomainError.AuthenticationFailed(e.getDescription(), cause = e)
        }
    }

    override suspend fun logout() {
        try {
            // Clearing the SSO cookie needs a foreground Activity; if none is
            // available we still clear local state below rather than leaving
            // the user stuck signed-in on this device (safe degrade).
            activityProvider.currentActivity()?.let { activity -> WebAuthProvider.logout(auth0.value).await(activity) }
            credentialsStore.clearCredentials()
            sessionCache.clear()
        } catch (e: CancellationException) {
            throw e
        } catch (e: AuthenticationException) {
            throw DomainError.LogoutFailed(e.getDescription(), cause = e)
        }
    }

    @Suppress("SwallowedException")
    // The Unrecoverable case doesn't need e's contents: any unrecoverable
    // failure means "wipe and force re-login", regardless of which one it was.
    override suspend fun refreshSession(): AuthSession {
        val credentials =
            try {
                credentialsStore.credentials()
            } catch (e: CredentialsStoreException.Unrecoverable) {
                credentialsStore.clearCredentials()
                sessionCache.clear()
                throw DomainError.SessionExpired
            } catch (e: CredentialsStoreException.Transient) {
                // Network/Keystore hiccup — never wipe; surface the cause so
                // ApiClient can distinguish NetworkUnavailable from SessionExpired.
                throw e.cause ?: DomainError.SessionExpired
            }
        if (!JwtClaims.audienceMatches(credentials.accessToken, config.audience)) {
            // Stored token targets a different API audience (env flip): still
            // un-expired, but the new API would 401 it, so wipe and force a fresh login.
            credentialsStore.clearCredentials()
            sessionCache.clear()
            throw DomainError.SessionExpired
        }
        val session = credentials.toAuthSession()
        sessionCache.store(session)
        return session
    }

    override suspend fun currentSession(): AuthSession? {
        sessionCache.current(clock)?.let { return it }
        return sessionCache.currentOrLoad(clock) { loadFromStore() }
    }

    @Suppress("SwallowedException")
    private suspend fun loadFromStore(): AuthSession? {
        // hasValidCredentials() already folds in refresh-token renewability, so
        // no separate canRenew() check is needed.
        if (!credentialsStore.hasValidCredentials()) return null
        val credentials =
            try {
                credentialsStore.credentials()
            } catch (e: CredentialsStoreException) {
                return null
            }
        if (!JwtClaims.audienceMatches(credentials.accessToken, config.audience)) {
            // Fails open elsewhere (see JwtClaims.audienceMatches); a genuine
            // mismatch here means the token is unusable, so discard it.
            credentialsStore.clearCredentials()
            return null
        }
        return credentials.toAuthSession()
    }
}

private fun Credentials.toAuthSession(): AuthSession =
    AuthSession(
        accessToken = accessToken,
        idToken = idToken,
        expiresAt = expiresAt.toInstant(),
        userProfile =
            UserProfile(
                userId = JwtClaims.extractSubject(idToken) ?: JwtClaims.extractSubject(accessToken).orEmpty(),
                email = JwtClaims.extractEmail(idToken).orEmpty(),
                name = JwtClaims.extractName(idToken),
            ),
        subscriptionTier = JwtClaims.extractTier(accessToken),
    )
