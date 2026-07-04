package uk.towncrierapp.data.auth

import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonPrimitive
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.util.Base64

/**
 * Decodes JWT payloads and extracts the individual claims
 * `Auth0AuthenticationService` needs — base64url decode of the payload
 * segment only, **no signature verification** (Auth0 already validated the
 * token; this is purely for reading claims client-side). Port of iOS
 * `JWTSubscriptionTierExtractor`.
 */
public object JwtClaims {
    /** Extracts the `subscription_tier` claim. Defaults to [SubscriptionTier.FREE] when absent, unrecognised, or malformed. */
    public fun extractTier(accessToken: String): SubscriptionTier {
        val claim = decodePayload(accessToken)?.get("subscription_tier")?.jsonPrimitive?.contentOrNull
        return claim?.let { SubscriptionTier.fromWireValue(it) } ?: SubscriptionTier.FREE
    }

    /** Extracts the `sub` claim, or `null` if absent or the token is malformed. */
    public fun extractSubject(token: String): String? = decodePayload(token)?.get("sub")?.jsonPrimitive?.contentOrNull

    /** Extracts the `email` claim, or `null` if absent or the token is malformed. May legitimately be blank (Sign in with Apple has no email scope). */
    public fun extractEmail(token: String): String? = decodePayload(token)?.get("email")?.jsonPrimitive?.contentOrNull

    /** Extracts the `name` claim, or `null` if absent or the token is malformed. */
    public fun extractName(token: String): String? = decodePayload(token)?.get("name")?.jsonPrimitive?.contentOrNull

    /**
     * Extracts the `aud` claim, normalised to a list. Per RFC 7519 the claim
     * may be a single string or an array of strings; both shapes return as
     * `List<String>`. Returns an empty list when the claim is absent or the
     * token is malformed — callers treat that as "unknown audience" and fail
     * open (see [audienceMatches]).
     */
    public fun extractAudiences(token: String): List<String> {
        val aud = decodePayload(token)?.get("aud") ?: return emptyList()
        return when (aud) {
            is JsonArray -> aud.mapNotNull { it.jsonPrimitive.contentOrNull }
            else -> listOfNotNull(aud.jsonPrimitive.contentOrNull)
        }
    }

    /**
     * Whether [accessToken] was issued for [expectedAudience]. **Fails
     * open**: a token whose `aud` claim is absent or undecodable is treated
     * as matching, so a parsing hiccup never signs a user out (#680 aud-
     * mismatch wipe semantics — the wipe only fires on a genuine mismatch).
     */
    public fun audienceMatches(
        accessToken: String,
        expectedAudience: String,
    ): Boolean {
        val audiences = extractAudiences(accessToken)
        if (audiences.isEmpty()) return true
        return expectedAudience in audiences
    }

    /** Decodes the base64url JSON payload segment of [token] into a [JsonObject], or `null` if malformed. */
    internal fun decodePayload(token: String): JsonObject? {
        val segments = token.split(".")
        if (segments.size < 2) return null
        return try {
            val bytes = Base64.getUrlDecoder().decode(padBase64Url(segments[1]))
            Json.parseToJsonElement(String(bytes, Charsets.UTF_8)) as? JsonObject
        } catch (e: IllegalArgumentException) {
            null
        } catch (e: SerializationException) {
            null
        }
    }

    private fun padBase64Url(segment: String): String {
        val remainder = segment.length % 4
        return if (remainder == 0) segment else segment + "=".repeat(4 - remainder)
    }
}
