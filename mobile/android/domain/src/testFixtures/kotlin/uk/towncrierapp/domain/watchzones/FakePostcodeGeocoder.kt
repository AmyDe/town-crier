package uk.towncrierapp.domain.watchzones

/** Hand-written fake for [PostcodeGeocoder]. */
public class FakePostcodeGeocoder(
    public var geocodeResult: Result<Coordinate> = Result.success(aCoordinate()),
) : PostcodeGeocoder {
    public val geocodeCalls: MutableList<String> = mutableListOf()

    override suspend fun geocode(postcode: String): Coordinate {
        geocodeCalls += postcode
        return geocodeResult.getOrThrow()
    }
}
