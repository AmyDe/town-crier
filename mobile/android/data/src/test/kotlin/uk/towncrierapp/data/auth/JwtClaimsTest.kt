package uk.towncrierapp.data.auth

import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.util.Base64
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `JwtClaims` decodes the JWT payload segment (base64url JSON, no signature
 * verification — the server already validated the token; this is purely for
 * reading claims client-side) and exposes the individual claims
 * `Auth0AuthenticationService` needs. Port of iOS
 * `JWTSubscriptionTierExtractor`.
 */
class JwtClaimsTest {
    @Test
    fun `extractSubject reads the sub claim`() {
        val token = fakeJwt("""{"sub":"auth0|abc123"}""")

        assertEquals("auth0|abc123", JwtClaims.extractSubject(token))
    }

    @Test
    fun `extractSubject returns null for a malformed token`() {
        assertNull(JwtClaims.extractSubject("not-a-jwt"))
        assertNull(JwtClaims.extractSubject(""))
    }

    @Test
    fun `extractAudiences accepts a single string aud claim`() {
        val token = fakeJwt("""{"aud":"https://api-dev.towncrierapp.uk"}""")

        assertEquals(listOf("https://api-dev.towncrierapp.uk"), JwtClaims.extractAudiences(token))
    }

    @Test
    fun `extractAudiences accepts a JSON array aud claim`() {
        val token = fakeJwt("""{"aud":["https://api-dev.towncrierapp.uk","https://other.example"]}""")

        assertEquals(
            listOf("https://api-dev.towncrierapp.uk", "https://other.example"),
            JwtClaims.extractAudiences(token),
        )
    }

    @Test
    fun `extractAudiences returns empty for a missing claim or malformed token`() {
        assertTrue(JwtClaims.extractAudiences(fakeJwt("""{"sub":"auth0|abc"}""")).isEmpty())
        assertTrue(JwtClaims.extractAudiences("not-a-jwt").isEmpty())
    }

    @Test
    fun `extractTier reads the lowercase subscription_tier claim`() {
        val token = fakeJwt("""{"subscription_tier":"pro"}""")

        assertEquals(SubscriptionTier.PRO, JwtClaims.extractTier(token))
    }

    @Test
    fun `extractTier defaults to Free when the claim is absent, unrecognised, or the token is malformed`() {
        assertEquals(SubscriptionTier.FREE, JwtClaims.extractTier(fakeJwt("""{"sub":"auth0|abc"}""")))
        assertEquals(SubscriptionTier.FREE, JwtClaims.extractTier(fakeJwt("""{"subscription_tier":"enterprise"}""")))
        assertEquals(SubscriptionTier.FREE, JwtClaims.extractTier("not-a-jwt"))
    }

    @Test
    fun `extractEmail reads the email claim and tolerates a blank value (Sign in with Apple has no email scope)`() {
        val token = fakeJwt("""{"email":""}""")

        assertEquals("", JwtClaims.extractEmail(token))
        assertNull(JwtClaims.extractEmail("not-a-jwt"))
    }

    @Test
    fun `extractName reads the name claim`() {
        val token = fakeJwt("""{"name":"Resident"}""")

        assertEquals("Resident", JwtClaims.extractName(token))
    }

    @Test
    fun `audienceMatches fails open when the aud claim is missing or the token is undecodable`() {
        assertTrue(JwtClaims.audienceMatches(fakeJwt("""{"sub":"auth0|abc"}"""), "https://api-dev.towncrierapp.uk"))
        assertTrue(JwtClaims.audienceMatches("not-a-jwt", "https://api-dev.towncrierapp.uk"))
    }

    @Test
    fun `audienceMatches is true when the expected audience is present`() {
        val token = fakeJwt("""{"aud":"https://api-dev.towncrierapp.uk"}""")

        assertTrue(JwtClaims.audienceMatches(token, "https://api-dev.towncrierapp.uk"))
    }

    @Test
    fun `audienceMatches is false when a real aud claim does not contain the expected audience`() {
        val token = fakeJwt("""{"aud":"https://api.towncrierapp.uk"}""")

        assertEquals(false, JwtClaims.audienceMatches(token, "https://api-dev.towncrierapp.uk"))
    }
}

/** Builds a JWT-shaped string (header.payload.signature) with a real base64url-encoded payload for tests. */
internal fun fakeJwt(payloadJson: String): String {
    val encoder = Base64.getUrlEncoder().withoutPadding()
    val header = encoder.encodeToString("""{"alg":"RS256"}""".toByteArray())
    val payload = encoder.encodeToString(payloadJson.toByteArray())
    return "$header.$payload.signature"
}
