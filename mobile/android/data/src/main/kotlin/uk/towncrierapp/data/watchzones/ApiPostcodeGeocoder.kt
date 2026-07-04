package uk.towncrierapp.data.watchzones

import kotlinx.serialization.Serializable
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.PostcodeGeocoder

/**
 * `PostcodeGeocoder` over the Town Crier API's `GET /v1/geocode/{postcode}`.
 * Port of iOS `APIPostcodeGeocoder`. The postcode is passed through as a
 * single path segment (it may contain an internal space, e.g. "SW1A 1AA") —
 * [ApiEndpoint]/`ApiClient` percent-encodes it via OkHttp's `addPathSegment`.
 */
public class ApiPostcodeGeocoder(
    private val apiClient: ApiClient,
) : PostcodeGeocoder {
    override suspend fun geocode(postcode: String): Coordinate {
        val response =
            apiClient.request(ApiEndpoint.get("/v1/geocode/${postcode.trim()}"), GeocodeResponseDto.serializer())
        return Coordinate(response.coordinates.latitude, response.coordinates.longitude)
    }
}

@Serializable
internal data class GeocodeResponseDto(
    val coordinates: CoordinatesDto,
)

@Serializable
internal data class CoordinatesDto(
    val latitude: Double,
    val longitude: Double,
)
