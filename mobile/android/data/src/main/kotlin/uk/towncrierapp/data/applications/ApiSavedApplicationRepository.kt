package uk.towncrierapp.data.applications

import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.data.api.DotNetTimeParser
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.SavedApplication
import uk.towncrierapp.domain.applications.SavedApplicationRepository

/**
 * `SavedApplicationRepository` over the Town Crier API. Save/unsave send
 * [PlanningApplicationId.value] with every `/` percent-encoded so the id stays
 * a single path segment.
 */
public class ApiSavedApplicationRepository(
    private val apiClient: ApiClient,
) : SavedApplicationRepository {
    override suspend fun savedApplications(): List<SavedApplication> =
        apiClient
            .request(ApiEndpoint.get("/v1/me/saved-applications"), ListSerializer(SavedApplicationDto.serializer()))
            .map { it.toDomain() }

    override suspend fun save(id: PlanningApplicationId) {
        apiClient.requestBytes(ApiEndpoint.put("/v1/me/saved-applications/${id.encodedPathSegment()}"))
    }

    override suspend fun unsave(id: PlanningApplicationId) {
        apiClient.requestBytes(ApiEndpoint.delete("/v1/me/saved-applications/${id.encodedPathSegment()}"))
    }
}

private fun PlanningApplicationId.encodedPathSegment(): String = value.replace("/", "%2F")

@Serializable
internal data class SavedApplicationDto(
    val applicationUid: String,
    val savedAt: String,
    val application: PlanningApplicationRowDto? = null,
)

internal fun SavedApplicationDto.toDomain(): SavedApplication =
    SavedApplication(
        applicationUid = PlanningApplicationId.parse(applicationUid),
        savedAt = requireNotNull(DotNetTimeParser.parse(savedAt)) { "unparseable savedAt: $savedAt" },
        application = application?.toDomain(),
    )
