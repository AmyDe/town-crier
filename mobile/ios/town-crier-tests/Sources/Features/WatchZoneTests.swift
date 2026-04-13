import Testing

@testable import TownCrierDomain

@Suite("WatchZone value object")
struct WatchZoneTests {
  @Test func init_validValues_createsWatchZone() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let postcode = try Postcode("CB1 2AD")
    let zone = try WatchZone(postcode: postcode, centre: centre, radiusMetres: 1000)
    #expect(zone.centre == centre)
    #expect(zone.radiusMetres == 1000)
    #expect(zone.name == postcode.value)
  }

  @Test func init_zeroRadius_throws() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let postcode = try Postcode("CB1 2AD")
    #expect(throws: DomainError.invalidWatchZoneRadius) {
      try WatchZone(postcode: postcode, centre: centre, radiusMetres: 0)
    }
  }

  @Test func init_negativeRadius_throws() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let postcode = try Postcode("CB1 2AD")
    #expect(throws: DomainError.invalidWatchZoneRadius) {
      try WatchZone(postcode: postcode, centre: centre, radiusMetres: -100)
    }
  }

  @Test func contains_coordinateInsideRadius_returnsTrue() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let nearby = try Coordinate(latitude: 52.2060, longitude: 0.1220)
    let postcode = try Postcode("CB1 2AD")
    let zone = try WatchZone(postcode: postcode, centre: centre, radiusMetres: 5000)
    #expect(zone.contains(nearby))
  }

  @Test func contains_coordinateFarAway_returnsFalse() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let farAway = try Coordinate(latitude: 51.5074, longitude: -0.1278)
    let postcode = try Postcode("CB1 2AD")
    let zone = try WatchZone(postcode: postcode, centre: centre, radiusMetres: 1000)
    #expect(!zone.contains(farAway))
  }

  @Test func init_generatesUniqueId() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let postcode = try Postcode("CB1 2AD")
    let zone1 = try WatchZone(postcode: postcode, centre: centre, radiusMetres: 1000)
    let zone2 = try WatchZone(postcode: postcode, centre: centre, radiusMetres: 1000)
    #expect(zone1.id != zone2.id)
  }

  @Test func init_storesAuthorityId() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let postcode = try Postcode("CB1 2AD")
    let zone = try WatchZone(
      postcode: postcode,
      centre: centre,
      radiusMetres: 1000,
      authorityId: 123
    )
    #expect(zone.authorityId == 123)
  }

  @Test func init_defaultAuthorityIdIsZero() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let postcode = try Postcode("CB1 2AD")
    let zone = try WatchZone(postcode: postcode, centre: centre, radiusMetres: 1000)
    #expect(zone.authorityId == 0)
  }

  // MARK: - Freeform name support (tc-y39l)

  @Test func init_acceptsFreeformName() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let zone = try WatchZone(name: "My Home Zone", centre: centre, radiusMetres: 1000)
    #expect(zone.name == "My Home Zone")
  }

  @Test func init_freeformName_nonPostcodeName_succeeds() throws {
    let centre = try Coordinate(latitude: 51.5014, longitude: -0.1419)
    let zone = try WatchZone(
      name: "Office near Westminster",
      centre: centre,
      radiusMetres: 2000,
      authorityId: 456
    )
    #expect(zone.name == "Office near Westminster")
    #expect(zone.authorityId == 456)
  }

  @Test func init_freeformName_emptyName_throws() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    #expect(throws: DomainError.invalidWatchZoneName) {
      try WatchZone(name: "", centre: centre, radiusMetres: 1000)
    }
  }

  @Test func init_freeformName_whitespaceOnlyName_throws() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    #expect(throws: DomainError.invalidWatchZoneName) {
      try WatchZone(name: "   ", centre: centre, radiusMetres: 1000)
    }
  }
}
