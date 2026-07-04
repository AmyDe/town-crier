package uk.towncrierapp.domain.auth

import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/** Auth method is derived from the JWT `sub` claim prefix (port of iOS `UserProfile.authMethod`). */
class UserProfileTest {
    @Test
    fun `auth0 prefix derives EmailPassword`() {
        val profile = UserProfile(userId = "auth0|abc123", email = "a@example.test", name = null)

        assertEquals(AuthMethod.EMAIL_PASSWORD, profile.authMethod)
    }

    @Test
    fun `google-oauth2 prefix derives Google`() {
        val profile = UserProfile(userId = "google-oauth2|abc123", email = "a@example.test", name = null)

        assertEquals(AuthMethod.GOOGLE, profile.authMethod)
    }

    @Test
    fun `apple prefix derives Apple`() {
        val profile = UserProfile(userId = "apple|abc123", email = "", name = null)

        assertEquals(AuthMethod.APPLE, profile.authMethod)
    }

    @Test
    fun `an unrecognised prefix derives Unknown`() {
        val profile = UserProfile(userId = "samlp|abc123", email = "a@example.test", name = null)

        assertEquals(AuthMethod.UNKNOWN, profile.authMethod)
    }

    @Test
    fun `an empty email (Sign in with Apple, no email scope) never crashes and stays blank`() {
        val profile = UserProfile(userId = "apple|abc123", email = "", name = null)

        assertEquals("", profile.email)
    }
}
