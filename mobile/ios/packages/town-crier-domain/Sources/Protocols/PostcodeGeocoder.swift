/// Converts a validated postcode into a geographic coordinate.
public protocol PostcodeGeocoder: Sendable {
    func geocode(_ postcode: Postcode) async throws -> Coordinate
}
