package uk.towncrierapp.domain.watchzones

/** Resolves a UK postcode to a [Coordinate] via `GET /v1/geocode/{postcode}`. Port of iOS `PostcodeGeocoder`. */
public interface PostcodeGeocoder {
    public suspend fun geocode(postcode: String): Coordinate
}
