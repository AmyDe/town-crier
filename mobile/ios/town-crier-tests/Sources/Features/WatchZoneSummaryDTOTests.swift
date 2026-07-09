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
      authorityId: 123,
      pushEnabled: true,
      emailInstantEnabled: true
    )

    let zone = try dto.toDomain()
    #expect(zone.id == WatchZoneId("zone-ok"))
    #expect(zone.name == "CB1 2AD")
    #expect(zone.radiusMetres == 2000)
    #expect(zone.authorityId == 123)
  }

  // MARK: - Per-zone notification flags (tc-kh1s)

  @Test("toDomain carries pushEnabled and emailInstantEnabled to domain model")
  func toDomain_carriesNotificationFlags() throws {
    let dto = WatchZoneSummaryDTO(
      id: "zone-ok",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 123,
      pushEnabled: false,
      emailInstantEnabled: true
    )

    let zone = try dto.toDomain()
    #expect(!zone.pushEnabled)
    #expect(zone.emailInstantEnabled)
  }

  @Test("decoding DTO without pushEnabled defaults to true")
  func decoding_missingPushEnabled_defaultsToTrue() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "authorityId": 123,
          "emailInstantEnabled": true
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.pushEnabled)
  }

  @Test("decoding DTO without emailInstantEnabled defaults to true")
  func decoding_missingEmailInstantEnabled_defaultsToTrue() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "authorityId": 123,
          "pushEnabled": true
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.emailInstantEnabled)
  }

  @Test("decoding DTO with explicit false flags preserves them")
  func decoding_explicitFalseFlags_arePreserved() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "authorityId": 123,
          "pushEnabled": false,
          "emailInstantEnabled": false
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(!dto.pushEnabled)
    #expect(!dto.emailInstantEnabled)
  }

  // MARK: - Paused zones (GH#889 P2)

  @Test("toDomain carries paused to domain model")
  func toDomain_carriesPausedFlag() throws {
    let dto = WatchZoneSummaryDTO(
      id: "zone-ok",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 123,
      paused: true
    )

    let zone = try dto.toDomain()
    #expect(zone.paused)
  }

  @Test("decoding DTO without paused defaults to false (back-compat)")
  func decoding_missingPaused_defaultsToFalse() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "authorityId": 123
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(!dto.paused)
  }

  @Test("decoding DTO with explicit paused: true preserves it")
  func decoding_explicitPausedTrue_isPreserved() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "authorityId": 123,
          "paused": true
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.paused)
  }
}
