package uk.towncrierapp.domain.auth

import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import org.junit.jupiter.api.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * `isExpired` takes an injected [Clock] rather than calling `Instant.now()`
 * itself (android-coding-standards: no wall-clock reads in domain logic).
 */
class AuthSessionTest {
    private val expiresAt: Instant = Instant.parse("2026-07-20T14:00:00Z")

    private fun aSession(expiresAt: Instant = this.expiresAt) =
        AuthSession(
            accessToken = "access-token",
            idToken = "id-token",
            expiresAt = expiresAt,
            userProfile = UserProfile(userId = "auth0|1", email = "a@example.test", name = null),
            subscriptionTier = SubscriptionTier.FREE,
        )

    @Test
    fun `a session is not expired before its expiry instant`() {
        val clock = Clock.fixed(expiresAt.minusSeconds(1), ZoneOffset.UTC)

        assertFalse(aSession().isExpired(clock))
    }

    @Test
    fun `a session is expired exactly at or after its expiry instant`() {
        val atExpiry = Clock.fixed(expiresAt, ZoneOffset.UTC)
        val afterExpiry = Clock.fixed(expiresAt.plusSeconds(1), ZoneOffset.UTC)

        assertTrue(aSession().isExpired(atExpiry))
        assertTrue(aSession().isExpired(afterExpiry))
    }
}
