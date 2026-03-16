import Testing
@testable import TownCrierDomain

@Suite("WatchZone value object")
struct WatchZoneTests {
    @Test func init_validValues_createsWatchZone() throws {
        let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        let zone = try WatchZone(centre: centre, radiusMetres: 1000)
        #expect(zone.centre == centre)
        #expect(zone.radiusMetres == 1000)
    }

    @Test func init_zeroRadius_throws() throws {
        let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        #expect(throws: DomainError.invalidWatchZoneRadius) {
            try WatchZone(centre: centre, radiusMetres: 0)
        }
    }

    @Test func init_negativeRadius_throws() throws {
        let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        #expect(throws: DomainError.invalidWatchZoneRadius) {
            try WatchZone(centre: centre, radiusMetres: -100)
        }
    }

    @Test func contains_coordinateInsideRadius_returnsTrue() throws {
        let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        let nearby = try Coordinate(latitude: 52.2060, longitude: 0.1220)
        let zone = try WatchZone(centre: centre, radiusMetres: 5000)
        #expect(zone.contains(nearby))
    }

    @Test func contains_coordinateFarAway_returnsFalse() throws {
        let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
        let farAway = try Coordinate(latitude: 51.5074, longitude: -0.1278)
        let zone = try WatchZone(centre: centre, radiusMetres: 1000)
        #expect(!zone.contains(farAway))
    }
}
