package uk.towncrierapp.data.watchzones

import kotlinx.coroutines.CancellationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import uk.towncrierapp.data.api.ApiClient
import uk.towncrierapp.data.api.ApiEndpoint
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneRepository

/**
 * `WatchZoneRepository` over the Town Crier API. Port of iOS
 * `APIWatchZoneRepository`. `create`/`update`/`delete` discard their response
 * bodies via [ApiClient.requestBytes] — the domain port's return type is
 * `Unit` (parity with iOS `save`/`update`/`delete`), and the caller reloads
 * the list afterwards.
 */
public class ApiWatchZoneRepository(
    private val apiClient: ApiClient,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : WatchZoneRepository {
    override suspend fun zones(): List<WatchZone> =
        apiClient
            .request(ApiEndpoint.get("/v1/me/watch-zones"), ListWatchZonesResponseDto.serializer())
            .zones
            // A malformed individual zone (e.g. a legacy record with an
            // out-of-range coordinate) is skipped rather than failing the
            // whole list — parity with iOS's `compactMap` behaviour.
            .mapNotNull { dto -> runCatchingDomainInvariant { dto.toDomain() } }

    override suspend fun create(zone: WatchZone) {
        val body = json.encodeToString(CreateWatchZoneRequestDto.serializer(), zone.toCreateRequestDto())
        try {
            apiClient.requestBytes(ApiEndpoint.post("/v1/me/watch-zones", body = body))
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            throw e.normalisedForCreateQuota()
        }
    }

    override suspend fun update(zone: WatchZone) {
        val body = json.encodeToString(UpdateWatchZoneRequestDto.serializer(), zone.toUpdateRequestDto())
        apiClient.requestBytes(ApiEndpoint.patch("/v1/me/watch-zones/${zone.id.value}", body = body))
    }

    @Suppress("SwallowedException")
    // Idempotent delete — already gone counts as success (epic #770 contract);
    // the exception carries no further information the caller needs.
    override suspend fun delete(id: WatchZoneId) {
        try {
            apiClient.requestBytes(ApiEndpoint.delete("/v1/me/watch-zones/${id.value}"))
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError.NotFound) {
            // Idempotent delete — already gone counts as success.
        }
    }
}

/**
 * The create endpoint's only 403 is "quota exceeded" (tc-gpjk / tc-z95t bead
 * brief), so ANY 403 on create — whether the server already shaped it as an
 * entitlement body or a plain error string — normalises to
 * `insufficientEntitlement("personal")`. This is a deliberate Android
 * divergence from iOS, which preserves an already-entitlement-shaped body's
 * `required` value; here the bead brief calls for the simpler, always-
 * personal mapping since a plain create 403 is quota-only in practice.
 */
private fun DomainError.normalisedForCreateQuota(): DomainError =
    when (this) {
        is DomainError.InsufficientEntitlement -> DomainError.InsufficientEntitlement("personal")
        is DomainError.ServerError -> if (status == 403) DomainError.InsufficientEntitlement("personal") else this
        else -> this
    }

@Suppress("SwallowedException")
// A domain-invariant violation (e.g. an out-of-range legacy coordinate, or a
// malformed/invalid boundary — GeoJsonPolygonDto.toDomain() throws
// IllegalArgumentException for the latter, since WatchZoneBoundary.of()
// itself returns a sealed result rather than throwing) skips just that item
// rather than failing the whole call — routine for old data, not a
// diagnostic-worthy failure the caller needs the original cause for.
private inline fun <T> runCatchingDomainInvariant(block: () -> T): T? =
    try {
        block()
    } catch (e: IllegalArgumentException) {
        null
    }

// MARK: - DTOs

@Serializable
internal data class ListWatchZonesResponseDto(
    val zones: List<WatchZoneDto>,
)

@Serializable
internal data class WatchZoneDto(
    val id: String,
    val name: String,
    val latitude: Double,
    val longitude: Double,
    val radiusMetres: Double,
    val authorityId: Int = 0,
    // Absent on legacy zones created before these flags existed — default to
    // true so those zones keep receiving every alert (epic #770 contract).
    val pushEnabled: Boolean = true,
    val emailInstantEnabled: Boolean = true,
    // null for a circle zone, and absent on responses predating this field
    // (older API, or a legacy circle-only zone) — both hydrate to null
    // (GH#1072 Phase 2), same back-compat convention as the flags above.
    val boundary: GeoJsonPolygonDto? = null,
)

@Serializable
internal data class CreateWatchZoneRequestDto(
    val name: String,
    val latitude: Double,
    val longitude: Double,
    val radiusMetres: Double,
    val authorityId: Int? = null,
    val pushEnabled: Boolean,
    val emailInstantEnabled: Boolean,
    // Omitted from the payload when null — create has no prior value to
    // distinguish "leave alone" from, so omission and null are equivalent
    // (same convention as authorityId above).
    val boundary: GeoJsonPolygonDto? = null,
)

/**
 * No default value on [boundary]: this repository's `update()` always sends
 * every field (no partial-update support), and kotlinx.serialization omits a
 * property from the encoded JSON only when it's absent AND equal to its
 * declared default — so a defaulted `= null` here would silently vanish for
 * a circle zone instead of encoding an explicit JSON `null`. The server's
 * PATCH handler treats an absent `boundary` key as "leave untouched" and an
 * explicit `null` as "revert to circle" — this DTO must always be able to
 * say the latter. Do NOT add a default value to this field.
 */
@Serializable
internal data class UpdateWatchZoneRequestDto(
    val name: String,
    val latitude: Double,
    val longitude: Double,
    val radiusMetres: Double,
    val authorityId: Int,
    val pushEnabled: Boolean,
    val emailInstantEnabled: Boolean,
    val boundary: GeoJsonPolygonDto?,
)

/**
 * The wire shape of a custom-shape watch zone boundary: a GeoJSON Polygon
 * with a single outer ring, first vertex repeated last to close it —
 * mirrors the server's `geoJSONPolygon`
 * (`api-go/internal/watchzones/store_postgres.go`) exactly. `coordinates` is
 * `[ring][vertex][longitude, latitude]`, GeoJSON's (lon, lat) ordering. Port
 * of iOS `GeoJSONPolygon`.
 *
 * [type] deliberately has no default: this class is always constructed with
 * `type = "Polygon"` (see [toGeoJsonDto]), and with `encodeDefaults = false`
 * (this repository's shared `Json` instance), a property left at its
 * default value is omitted from the encoded JSON entirely — which would
 * silently drop `type` from every outgoing boundary.
 */
@Serializable
internal data class GeoJsonPolygonDto(
    val type: String,
    val coordinates: List<List<List<Double>>>,
)

internal fun WatchZoneDto.toDomain(): WatchZone =
    WatchZone(
        id = WatchZoneId(id),
        name = name,
        centre = Coordinate(latitude, longitude),
        radiusMetres = radiusMetres,
        authorityId = authorityId,
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
        boundary = boundary?.toDomain(),
    )

internal fun WatchZone.toCreateRequestDto(): CreateWatchZoneRequestDto =
    CreateWatchZoneRequestDto(
        name = name,
        latitude = centre.latitude,
        longitude = centre.longitude,
        radiusMetres = radiusMetres,
        // 0 means "not yet resolved" — omit so the server reverse-geocodes it.
        authorityId = authorityId.takeIf { it > 0 },
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
        boundary = boundary?.toGeoJsonDto(),
    )

internal fun WatchZone.toUpdateRequestDto(): UpdateWatchZoneRequestDto =
    UpdateWatchZoneRequestDto(
        name = name,
        latitude = centre.latitude,
        longitude = centre.longitude,
        radiusMetres = radiusMetres,
        authorityId = authorityId,
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
        boundary = boundary?.toGeoJsonDto(),
    )

/**
 * Decodes the wire GeoJSON polygon into a validated domain
 * [WatchZoneBoundary]. Defensive shape checks mirror iOS
 * `GeoJSONPolygon.toDomain()`: [GeoJsonPolygonDto.type] must be `"Polygon"`,
 * [GeoJsonPolygonDto.coordinates] must have exactly one (outer) ring, and
 * each vertex must be a `[longitude, latitude]` pair — GeoJSON is lon-first,
 * [Coordinate]'s constructor is lat-first, so the pair is swapped here.
 * Throws [IllegalArgumentException] for any shape violation, an
 * out-of-range [Coordinate], or a non-[WatchZoneBoundaryResult.Valid]
 * [WatchZoneBoundary.of] result — [WatchZoneBoundary.of] itself returns a
 * sealed result rather than throwing (it's user-input validation, not a
 * programmer-error invariant), so this function bridges that into the same
 * exception [runCatchingDomainInvariant] already catches for every other
 * malformed-legacy-data case.
 */
private fun GeoJsonPolygonDto.toDomain(): WatchZoneBoundary {
    require(type == "Polygon") { "boundary type must be \"Polygon\", was \"$type\"" }
    require(coordinates.size == 1) { "boundary must have exactly one ring, had ${coordinates.size}" }
    val vertices =
        coordinates.single().map { vertex ->
            require(vertex.size == 2) { "boundary vertex must be [longitude, latitude], was $vertex" }
            Coordinate(latitude = vertex[1], longitude = vertex[0])
        }
    return when (val result = WatchZoneBoundary.of(vertices)) {
        is WatchZoneBoundaryResult.Valid -> result.boundary
        else -> throw IllegalArgumentException("invalid watch zone boundary: $result")
    }
}

/**
 * Encodes a validated domain [WatchZoneBoundary] into the wire GeoJSON
 * shape. [WatchZoneBoundary.vertices] is already the closed ring (first
 * vertex repeated last) — not re-closed here.
 */
private fun WatchZoneBoundary.toGeoJsonDto(): GeoJsonPolygonDto =
    GeoJsonPolygonDto(
        type = "Polygon",
        coordinates = listOf(vertices.map { listOf(it.longitude, it.latitude) }),
    )
