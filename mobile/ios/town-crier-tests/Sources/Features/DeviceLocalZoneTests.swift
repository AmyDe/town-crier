import Testing

@testable import TownCrierDomain

@Suite("DeviceLocalZone value object")
struct DeviceLocalZoneTests {
  @Test func init_validValues_createsZone() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let zone = try DeviceLocalZone(name: "Home", centre: centre, radiusMetres: 1000)

    #expect(zone.name == "Home")
    #expect(zone.centre == centre)
    #expect(zone.radiusMetres == 1000)
  }

  @Test func init_emptyName_throws() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    #expect(throws: DomainError.invalidWatchZoneName) {
      try DeviceLocalZone(name: "", centre: centre, radiusMetres: 1000)
    }
  }

  @Test func init_whitespaceOnlyName_throws() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    #expect(throws: DomainError.invalidWatchZoneName) {
      try DeviceLocalZone(name: "   ", centre: centre, radiusMetres: 1000)
    }
  }

  @Test func init_trimsWhitespaceFromName() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    let zone = try DeviceLocalZone(name: "  Home  ", centre: centre, radiusMetres: 1000)

    #expect(zone.name == "Home")
  }

  @Test func init_generatesUniqueId() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    let zone1 = try DeviceLocalZone(name: "Home", centre: centre, radiusMetres: 1000)
    let zone2 = try DeviceLocalZone(name: "Home", centre: centre, radiusMetres: 1000)

    #expect(zone1.id != zone2.id)
  }

  // MARK: - Radius clamp [100, 5000] (matches the public near-point clamp)

  @Test func init_radiusAboveMax_clampsTo5000() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    let zone = try DeviceLocalZone(name: "Home", centre: centre, radiusMetres: 9000)

    #expect(zone.radiusMetres == 5000)
  }

  @Test func init_radiusBelowMin_clampsTo100() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    let zone = try DeviceLocalZone(name: "Home", centre: centre, radiusMetres: 10)

    #expect(zone.radiusMetres == 100)
  }

  @Test func init_radiusWithinRange_isUnchanged() throws {
    let centre = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    let zone = try DeviceLocalZone(name: "Home", centre: centre, radiusMetres: 3000)

    #expect(zone.radiusMetres == 3000)
  }

  @Test func clampRadius_matchesInitClamping() {
    #expect(DeviceLocalZone.clampRadius(9000) == 5000)
    #expect(DeviceLocalZone.clampRadius(10) == 100)
    #expect(DeviceLocalZone.clampRadius(2500) == 2500)
  }

  @Test func maxZoneCount_isOne() {
    #expect(DeviceLocalZone.maxZoneCount == 1)
  }
}
