import Foundation
import TownCrierDomain

final class SpyPostcodeGeocoder: PostcodeGeocoder, @unchecked Sendable {
    private(set) var geocodeCalls: [Postcode] = []
    var geocodeResult: Result<Coordinate, Error> = .success(.cambridge)

    func geocode(_ postcode: Postcode) async throws -> Coordinate {
        geocodeCalls.append(postcode)
        return try geocodeResult.get()
    }
}
