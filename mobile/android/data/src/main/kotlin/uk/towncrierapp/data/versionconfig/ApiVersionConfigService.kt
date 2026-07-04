package uk.towncrierapp.data.versionconfig

import kotlinx.coroutines.CancellationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.Request
import uk.towncrierapp.data.api.HttpTransport
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.versionconfig.AppVersion
import uk.towncrierapp.domain.versionconfig.VersionConfigService
import java.io.IOException

/**
 * Fetches the minimum supported app version — the one endpoint that must
 * work before the user is authenticated, so it goes over the raw
 * [HttpTransport] directly (no `Authorization` header, no session check)
 * rather than through [uk.towncrierapp.data.api.ApiClient].
 */
public class ApiVersionConfigService(
    private val baseUrl: String,
    private val transport: HttpTransport,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : VersionConfigService {
    override suspend fun fetchMinimumVersion(): AppVersion {
        val url =
            baseUrl
                .toHttpUrl()
                .newBuilder()
                .addPathSegment("v1")
                .addPathSegment("version-config")
                .build()
        val request =
            Request
                .Builder()
                .url(url)
                .header("Accept", "application/json")
                .get()
                .build()

        val response =
            try {
                transport.execute(request)
            } catch (e: CancellationException) {
                throw e
            } catch (e: IOException) {
                throw DomainError.NetworkUnavailable
            }

        val bodyText = response.body?.string().orEmpty()
        if (response.code !in 200..299) {
            throw DomainError.ServerError(response.code, bodyText.ifBlank { null })
        }

        val dto =
            try {
                json.decodeFromString(VersionConfigDto.serializer(), bodyText)
            } catch (e: SerializationException) {
                throw DomainError.Unexpected("Invalid version config response")
            }

        return AppVersion.parse(dto.minimumVersion)
            ?: throw DomainError.Unexpected("Invalid version string: ${dto.minimumVersion}")
    }
}

@Serializable
internal data class VersionConfigDto(
    val minimumVersion: String,
)
