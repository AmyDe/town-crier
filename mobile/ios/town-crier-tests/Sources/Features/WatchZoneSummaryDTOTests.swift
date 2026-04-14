import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

@Suite("WatchZoneSummaryDTO mapping")
struct WatchZoneSummaryDTOTests {

  @Test("toDomain throws invalidCoordinate for out-of-range latitude")
  func toDomain_invalidLatitude_throwsInvalidCoordinate() {
    let dto = WatchZoneSummaryDTO(
      id: "zone-bad",
      name: "CB1 2AD",
      latitude: 999.0,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 123
    )

    #expect(throws: DomainError.invalidCoordinate) {
      try dto.toDomain()
    }
  }

  @Test("toDomain throws invalidWatchZoneName for empty name")
  func toDomain_emptyName_throwsInvalidWatchZoneName() {
    let dto = WatchZoneSummaryDTO(
      id: "zone-bad",
      name: "",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 123
    )

    #expect(throws: DomainError.invalidWatchZoneName) {
      try dto.toDomain()
    }
  }

  @Test("toDomain throws invalidWatchZoneRadius for zero radius")
  func toDomain_zeroRadius_throwsInvalidWatchZoneRadius() {
    let dto = WatchZoneSummaryDTO(
      id: "zone-bad",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 0,
      authorityId: 123
    )

    #expect(throws: DomainError.invalidWatchZoneRadius) {
      try dto.toDomain()
    }
  }

  @Test("toDomain succeeds for valid DTO")
  func toDomain_validDTO_returnsWatchZone() throws {
    let dto = WatchZoneSummaryDTO(
      id: "zone-ok",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 123
    )

    let zone = try dto.toDomain()
    #expect(zone.id == WatchZoneId("zone-ok"))
    #expect(zone.name == "CB1 2AD")
    #expect(zone.radiusMetres == 2000)
    #expect(zone.authorityId == 123)
  }
}
