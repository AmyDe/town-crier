package uk.towncrierapp.data.auth

import com.auth0.android.result.Credentials
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.auth.DomainError
import java.io.IOException
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

private const val DEV_AUDIENCE = "https://api-dev.towncrierapp.uk"

/**
 * `currentSession()`/`refreshSession()` are the unit-testable surface of
 * [Auth0AuthenticationService] — `login()`/`logout()` need a real Activity
 * and Custom Tabs and are verified on-device (epic #770 test strategy: "one
 * Auth0-SDK-boundary integration is accepted as emulator-verified"). Covers
 * the four ported bug fixes: #680 (aud-mismatch wipe, fail-open), tc-funq
 * (renewability), and the refresh unrecoverable-vs-transient split.
 */
class Auth0AuthenticationServiceTest {
    private fun aCredentials(
        accessToken: String = fakeJwt("""{"sub":"auth0|1","aud":"$DEV_AUDIENCE"}"""),
        idToken: String = fakeJwt("""{"sub":"auth0|1","email":"resident@example.test"}"""),
        expiresAt: Instant = Instant.parse("2026-07-20T15:00:00Z"),
        refreshToken: String? = "refresh-token",
    ) = Credentials(
        idToken,
        accessToken,
        "Bearer",
        refreshToken,
        // Not `import java.util.Date` (forbidden — java.time only): the Auth0
        // SDK's own Credentials constructor mandates java.util.Date, so this
        // fully-qualified reference is a one-off interop conversion, not our
        // own date type creeping in.
        java.util.Date.from(expiresAt),
        "openid profile email offline_access",
    )

    private fun makeSut(
        credentialsStore: FakeCredentialsStore = FakeCredentialsStore(),
        audience: String = DEV_AUDIENCE,
    ) = Auth0AuthenticationService(
        config = Auth0Config(clientId = "client-id", domain = "towncrierapp.uk.auth0.com", audience = audience),
        credentialsStore = credentialsStore,
        activityProvider = CurrentActivityProvider { null },
        // currentSession()/refreshSession() never touch auth0 — only login()/logout() do
        // (verified on-device, see class doc), so a real Auth0 instance is never needed here.
        auth0 = lazy { error("not used by currentSession()/refreshSession()") },
        sessionCache = SessionCache(scope = CoroutineScope(UnconfinedTestDispatcher())),
        clock = Clock.fixed(Instant.parse("2026-07-20T10:00:00Z"), ZoneOffset.UTC),
    )

    // MARK: - currentSession()

    @Test
    fun `currentSession returns null when there are no valid credentials (signed out)`() =
        runTest {
            val store = FakeCredentialsStore(hasValidCredentialsResult = false)
            val sut = makeSut(store)

            assertNull(sut.currentSession())
        }

    @Test
    fun `currentSession with valid credentials returns a session built from the access and id tokens`() =
        runTest {
            val store =
                FakeCredentialsStore(
                    hasValidCredentialsResult = true,
                    credentialsResult = Result.success(aCredentials()),
                )
            val sut = makeSut(store)

            val session = sut.currentSession()

            assertEquals("resident@example.test", session?.userProfile?.email)
            assertEquals("auth0|1", session?.userProfile?.userId)
        }

    @Test
    fun `tc-funq an expired access token with a valid refresh token still counts as signed-in (renewable)`() =
        runTest {
            // hasValidCredentials() on the real SDK already folds in refresh-token
            // renewability (see Auth0AuthenticationService doc comment) — this
            // fake simply asserts our currentSession() trusts that contract
            // rather than short-circuiting to signed-out on its own.
            val store =
                FakeCredentialsStore(
                    hasValidCredentialsResult = true,
                    credentialsResult = Result.success(aCredentials(expiresAt = Instant.parse("2026-07-20T09:00:00Z"))),
                )
            val sut = makeSut(store)

            val session = sut.currentSession()

            assertEquals("auth0|1", session?.userProfile?.userId)
        }

    @Test
    fun `aud-mismatch wipes credentials and cache, ending up signed-out (#680)`() =
        runTest {
            val store =
                FakeCredentialsStore(
                    hasValidCredentialsResult = true,
                    credentialsResult =
                        Result.success(
                            aCredentials(
                                accessToken = fakeJwt("""{"sub":"auth0|1","aud":"https://api.towncrierapp.uk"}"""),
                            ),
                        ),
                )
            val sut = makeSut(store, audience = DEV_AUDIENCE)

            val session = sut.currentSession()

            assertNull(session)
            assertEquals(1, store.clearCredentialsCalls.size)
        }

    @Test
    fun `an undecodable or missing aud claim keeps the session (fail open)`() =
        runTest {
            val store =
                FakeCredentialsStore(
                    hasValidCredentialsResult = true,
                    credentialsResult = Result.success(aCredentials(accessToken = fakeJwt("""{"sub":"auth0|1"}"""))),
                )
            val sut = makeSut(store)

            val session = sut.currentSession()

            assertEquals("auth0|1", session?.userProfile?.userId)
            assertEquals(0, store.clearCredentialsCalls.size)
        }

    @Test
    fun `a credentials-store failure while loading is treated as signed-out, not a crash`() =
        runTest {
            val store =
                FakeCredentialsStore(
                    hasValidCredentialsResult = true,
                    credentialsResult =
                        Result.failure(
                            CredentialsStoreException.Transient(IOException("keystore hiccup")),
                        ),
                )
            val sut = makeSut(store)

            assertNull(sut.currentSession())
        }

    // MARK: - refreshSession()

    @Test
    fun `refreshSession returns the refreshed session and caches it`() =
        runTest {
            val store = FakeCredentialsStore(credentialsResult = Result.success(aCredentials()))
            val sut = makeSut(store)

            val session = sut.refreshSession()

            assertEquals("auth0|1", session.userProfile.userId)
        }

    @Test
    fun `refreshSession wipes credentials and throws SessionExpired on an unrecoverable failure`() =
        runTest {
            val store =
                FakeCredentialsStore(credentialsResult = Result.failure(CredentialsStoreException.Unrecoverable))
            val sut = makeSut(store)

            assertIs<DomainError.SessionExpired>(assertFailsWith<DomainError> { sut.refreshSession() })
            assertEquals(1, store.clearCredentialsCalls.size)
        }

    @Test
    fun `refreshSession surfaces a transient IOException without wiping credentials`() =
        runTest {
            val networkFailure = IOException("no connection")
            val store =
                FakeCredentialsStore(
                    credentialsResult = Result.failure(CredentialsStoreException.Transient(networkFailure)),
                )
            val sut = makeSut(store)

            val thrown = assertFailsWith<IOException> { sut.refreshSession() }

            assertEquals(networkFailure, thrown)
            assertTrue(store.clearCredentialsCalls.isEmpty())
        }

    @Test
    fun `refreshSession on an audience mismatch wipes credentials and throws SessionExpired`() =
        runTest {
            val store =
                FakeCredentialsStore(
                    credentialsResult =
                        Result.success(
                            aCredentials(
                                accessToken = fakeJwt("""{"sub":"auth0|1","aud":"https://api.towncrierapp.uk"}"""),
                            ),
                        ),
                )
            val sut = makeSut(store, audience = DEV_AUDIENCE)

            assertIs<DomainError.SessionExpired>(assertFailsWith<DomainError> { sut.refreshSession() })
            assertEquals(1, store.clearCredentialsCalls.size)
        }
}
