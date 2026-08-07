package uk.towncrierapp.presentation.features.map

import com.google.android.gms.maps.model.LatLng
import com.google.android.gms.maps.model.LatLngBounds
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.watchzones.Coordinate

/** [Coordinate] <-> `LatLng`/`LatLngBounds` bridges between the pure `:domain` map types and the Google Maps SDK's own model classes. */
internal fun Coordinate.toLatLng(): LatLng = LatLng(latitude, longitude)

internal fun LatLng.toCoordinate(): Coordinate = Coordinate(latitude, longitude)

internal fun MapViewport.toLatLngBounds(): LatLngBounds = LatLngBounds(LatLng(south, west), LatLng(north, east))

internal fun LatLngBounds.toMapViewport(): MapViewport =
    MapViewport(
        west = southwest.longitude,
        south = southwest.latitude,
        east = northeast.longitude,
        north = northeast.latitude,
    )
