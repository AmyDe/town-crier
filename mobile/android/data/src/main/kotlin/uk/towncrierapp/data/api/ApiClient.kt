package uk.towncrierapp.data.api

import android.util.Log
import kotlinx.coroutines.CancellationException
import kotlinx.serialization.KSerializer
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import uk.towncrierapp.domain.auth.AuthSession
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.auth.DomainError
import java.io.IOException

/** `requestPaged`'s result: the decoded body plus the opaque `X-Next-Cursor` continuation token (`null` on the last page). */
internal data class PagedResult<T>(
    val value: T,
    val nextCursor: String?,
)

/** Marks a 401 response internally so [ApiClient] can trigger exactly one refresh-and-retry. */
private class UnauthorizedException : Exception()

/**
 * Hand-rolled API client — 1:1 port of iOS `URLSessionAPIClient` behaviour
 * (epic #770 "API contract essentials"; no Retrofit). Every request:
 * requires a session first (else [DomainError.SessionExpired], no network
 * call); attaches `Authorization: Bearer`, `Accept: application/json`
 * always, `Content-Type: application/json` only with a body; on 401,
 * refreshes the session once and retries once; 403 is sniffed for an
 * `insufficient_entitlement` body; 404 and other >=400 map to their
 * `DomainError` cases; nothing else retries. Request building and status
 * mapping are top-level functions in this file, not members — they need
 * none of this class's mutable/session state, only [baseUrl]/[json] passed
 * explicitly, which keeps this class's own responsibility to session +
 * retry orchestration.
 */
public class ApiClient(
    private val baseUrl: String,
    private val transport: HttpTransport,
    private val authService: AuthenticationService,
    private val json: Json = Json { ignoreUnknownKeys = true },
    private val enableDebugLogging: Boolean = false,
) {
    internal suspend fun <T> request(
        endpoint: ApiEndpoint,
        serializer: KSerializer<T>,
    ): T = performRequest(endpoint, serializer).value

    /** Returns the decoded body alongside the `X-Next-Cursor` header (`null` = last page). See [PagedResult]. */
    internal suspend fun <T> requestPaged(
        endpoint: ApiEndpoint,
        serializer: KSerializer<T>,
    ): PagedResult<T> = performRequest(endpoint, serializer)

    /** Returns the raw response body bytes untouched — for opaque payloads (e.g. a future GDPR export). */
    internal suspend fun requestBytes(endpoint: ApiEndpoint): ByteArray {
        val session = requireSession()
        return withRetryOn401(session.accessToken) { accessToken -> executeBytes(endpoint, accessToken) }
    }

    private suspend fun <T> performRequest(
        endpoint: ApiEndpoint,
        serializer: KSerializer<T>,
    ): PagedResult<T> {
        val session = requireSession()
        return withRetryOn401(
            session.accessToken,
        ) { accessToken -> executeAndDecode(endpoint, accessToken, serializer) }
    }

    /**
     * Runs [action] with [initialAccessToken]; on a 401, refreshes the
     * session exactly once and retries [action] exactly once with the
     * refreshed token — the epic's whole retry-policy contract ("no other
     * retries") lives in this one place so [performRequest] and
     * [requestBytes] don't each carry their own copy of it.
     */
    @Suppress("SwallowedException", "TooGenericExceptionCaught")
    // The three deliberately-generic catches below ARE the policy table this
    // function exists to centralise (epic #770: 401 -> refresh-once-retry-
    // once; IOException -> NetworkUnavailable; anything else -> SessionExpired
    // with no further retry) — narrowing them would make the policy incomplete.
    private suspend fun <T> withRetryOn401(
        initialAccessToken: String,
        action: suspend (accessToken: String) -> T,
    ): T =
        try {
            action(initialAccessToken)
        } catch (e: CancellationException) {
            throw e
        } catch (e: UnauthorizedException) {
            val refreshed = refreshOrThrow()
            try {
                action(refreshed.accessToken)
            } catch (e2: CancellationException) {
                throw e2
            } catch (e2: IOException) {
                throw DomainError.NetworkUnavailable
            } catch (e2: UnauthorizedException) {
                // The retry itself got another 401 — stop here rather than loop; the
                // session is unusable either way (epic #770: "no other retries").
                throw DomainError.SessionExpired
            }
        } catch (e: IOException) {
            throw DomainError.NetworkUnavailable
        }

    private suspend fun requireSession(): AuthSession {
        log(enableDebugLogging) { "▶ requesting session" }
        return authService.currentSession() ?: run {
            log(enableDebugLogging) { "✗ no active session" }
            throw DomainError.SessionExpired
        }
    }

    /**
     * Refreshes the session, normalising every failure per the epic's
     * retry-policy contract: [IOException] -> [DomainError.NetworkUnavailable],
     * anything else -> [DomainError.SessionExpired]. The final catch is
     * deliberately generic — it's the policy's "anything else" case, not an
     * oversight — so narrowing it further isn't possible while staying correct.
     */
    @Suppress("SwallowedException", "TooGenericExceptionCaught")
    private suspend fun refreshOrThrow(): AuthSession =
        try {
            authService.refreshSession()
        } catch (e: CancellationException) {
            throw e
        } catch (e: IOException) {
            throw DomainError.NetworkUnavailable
        } catch (e: Exception) {
            throw DomainError.SessionExpired
        }

    private suspend fun <T> executeAndDecode(
        endpoint: ApiEndpoint,
        accessToken: String,
        serializer: KSerializer<T>,
    ): PagedResult<T> {
        val response = executeRaw(endpoint, accessToken)
        val bodyText = response.body?.string().orEmpty()
        mapHttpStatus(json, response.code, bodyText)
        return PagedResult(decode(json, bodyText, serializer), response.header("X-Next-Cursor"))
    }

    private suspend fun executeBytes(
        endpoint: ApiEndpoint,
        accessToken: String,
    ): ByteArray {
        val response = executeRaw(endpoint, accessToken)
        val bytes = response.body?.bytes() ?: ByteArray(0)
        mapHttpStatus(json, response.code, String(bytes, Charsets.UTF_8))
        return bytes
    }

    private suspend fun executeRaw(
        endpoint: ApiEndpoint,
        accessToken: String,
    ): Response {
        val request = buildRequest(baseUrl, endpoint, accessToken)
        log(enableDebugLogging) { "→ ${endpoint.method} ${endpoint.path}" }
        val response = transport.execute(request)
        log(enableDebugLogging) { "← ${endpoint.method} ${endpoint.path} ${response.code}" }
        return response
    }
}

private inline fun log(
    enableDebugLogging: Boolean,
    message: () -> String,
) {
    // Debug-flavor-only request logging (method/path/status — never tokens),
    // wired from :app via BuildConfig.DEBUG. Guarded so plain JVM unit tests
    // (enableDebugLogging = false by default) never touch android.util.Log.
    if (enableDebugLogging) Log.d("ApiClient", message())
}

// No charset parameter: BridgeInterceptor rewrites the Content-Type header
// from this RequestBody's contentType() unconditionally once a real
// OkHttpClient executes the request, so the media type set here — not the
// explicit header() call below — is what actually reaches the wire. Bare
// "application/json" keeps both the wire value and the (BridgeInterceptor-
// free) FakeHttpTransport-observed value equal to the epic's exact contract.
private val jsonMediaType = "application/json".toMediaType()
private val emptyBody: RequestBody = ByteArray(0).toRequestBody(null)

private fun buildRequest(
    baseUrl: String,
    endpoint: ApiEndpoint,
    accessToken: String,
): Request {
    val urlBuilder = baseUrl.toHttpUrl().newBuilder()
    endpoint.path
        .trim('/')
        .split("/")
        .filter { it.isNotEmpty() }
        // addEncodedPathSegment (not addPathSegment): a caller that has
        // pre-percent-encoded a literal "/" it needs preserved WITHIN one
        // segment (e.g. ApiSavedApplicationRepository's "%2F"-escaped id)
        // would otherwise have that escape's "%" itself re-encoded to "%25"
        // (double-encoding it into mush). Every OTHER unsafe character still
        // gets encoded either way, so existing callers passing plain,
        // unencoded segments (UUIDs, postcodes with a space) are unaffected.
        .forEach { urlBuilder.addEncodedPathSegment(it) }
    endpoint.query.forEach { (name, value) -> urlBuilder.addQueryParameter(name, value) }

    val builder =
        Request
            .Builder()
            .url(urlBuilder.build())
            .header("Authorization", "Bearer $accessToken")
            .header("Accept", "application/json")

    val requestBody = endpoint.body?.toRequestBody(jsonMediaType)
    if (requestBody != null) {
        builder.header("Content-Type", "application/json")
    }
    when (endpoint.method) {
        "GET" -> builder.get()
        "DELETE" -> if (requestBody != null) builder.delete(requestBody) else builder.delete()
        "POST" -> builder.post(requestBody ?: emptyBody)
        "PUT" -> builder.put(requestBody ?: emptyBody)
        "PATCH" -> builder.patch(requestBody ?: emptyBody)
        else -> error("Unsupported HTTP method: ${endpoint.method}")
    }
    return builder.build()
}

// A status-code-to-error dispatcher inherently branches once per HTTP status
// bucket (epic #770's exact contract: 401/403/404/other-4xx-5xx) — that IS
// the function, not a complexity smell to fix by merging branches.
@Suppress("ThrowsCount")
private fun mapHttpStatus(
    json: Json,
    code: Int,
    body: String,
) {
    when {
        code in 200..299 -> return
        code == 401 -> throw UnauthorizedException()
        code == 403 -> throw mapForbidden(json, body)
        code == 404 -> throw DomainError.NotFound
        code >= 400 -> throw DomainError.ServerError(code, body.ifBlank { null })
    }
}

private fun mapForbidden(
    json: Json,
    body: String,
): DomainError {
    val required = insufficientEntitlementRequired(json, body)
    return if (required != null) {
        DomainError.InsufficientEntitlement(required)
    } else {
        DomainError.ServerError(403, body.ifBlank { null })
    }
}

@Suppress("SwallowedException")
// A 403 body that isn't the entitlement envelope is routine (most 403s
// aren't entitlement-gated) — falling back to a generic ServerError, not a
// diagnostic-worthy failure.
private fun insufficientEntitlementRequired(
    json: Json,
    body: String,
): String? =
    try {
        val decoded = json.decodeFromString(InsufficientEntitlementBody.serializer(), body)
        decoded.required.takeIf { decoded.error == "insufficient_entitlement" }
    } catch (e: IllegalArgumentException) {
        // SerializationException extends IllegalArgumentException.
        null
    }

private fun <T> decode(
    json: Json,
    body: String,
    serializer: KSerializer<T>,
): T =
    try {
        json.decodeFromString(serializer, body)
    } catch (e: IllegalArgumentException) {
        // SerializationException extends IllegalArgumentException.
        throw DomainError.Unexpected("Failed to decode response: ${e.message}", cause = e)
    }

@kotlinx.serialization.Serializable
private data class InsufficientEntitlementBody(
    val error: String,
    val required: String,
)
