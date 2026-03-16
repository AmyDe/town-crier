import Testing
@testable import TownCrierDomain

@Suite("Coordinate value object")
struct CoordinateTests {
    @Test func init_validValues_createsCoordinate() throws {
        let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        #expect(coordinate.latitude == 52.2053)
        #expect(coordinate.longitude == 0.1218)
    }

    @Test func init_latitudeTooHigh_throws() {
        #expect(throws: DomainError.invalidCoordinate) {
            try Coordinate(latitude: 91.0, longitude: 0.0)
        }
    }

    @Test func init_latitudeTooLow_throws() {
        #expect(throws: DomainError.invalidCoordinate) {
            try Coordinate(latitude: -91.0, longitude: 0.0)
        }
    }

    @Test func init_longitudeTooHigh_throws() {
        #expect(throws: DomainError.invalidCoordinate) {
            try Coordinate(latitude: 0.0, longitude: 181.0)
        }
    }

    @Test func init_longitudeTooLow_throws() {
        #expect(throws: DomainError.invalidCoordinate) {
            try Coordinate(latitude: 0.0, longitude: -181.0)
        }
    }

    @Test func equality_sameValues_areEqual() throws {
        let a = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        let b = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        #expect(a == b)
    }
}
