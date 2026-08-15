import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Pre-canned filter (`filterKey`) DTO decode/mapping tests (GH#1098, bead
/// tc-hogkn). Split out of `WatchZoneSummaryDTOTests` to keep that file under
/// the `file_length` SwiftLint threshold, mirroring the existing split into
/// `APIWatchZoneRepositoryBoundaryTests`.
@Suite("WatchZoneSummaryDTO mapping — pre-canned filter")
struct WatchZoneSummaryDTOFilterTests {

  @Test("decoding DTO without filterKey hydrates to nil (back-compat)")
  func decoding_missingFilterKey_hydratesToNil() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.filterKey == nil)
  }

  @Test("decoding DTO with filterKey: null hydrates to nil")
  func decoding_nullFilterKey_hydratesToNil() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "filterKey": null
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.filterKey == nil)
  }

  @Test("decoding DTO with a recognised filterKey preserves it")
  func decoding_recognisedFilterKey_isPreserved() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "filterKey": "loft_extension"
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.filterKey == .loftExtension)
  }

  @Test("decoding DTO with an unrecognised filterKey string fails open to nil")
  func decoding_unrecognisedFilterKey_failsOpenToNil() throws {
    let json = """
      {
          "id": "zone-001",
          "name": "CB1 2AD",
          "latitude": 52.2053,
          "longitude": 0.1218,
          "radiusMetres": 2000,
          "filterKey": "some_future_filter_not_yet_shipped"
      }
      """
    let dto = try JSONDecoder().decode(WatchZoneSummaryDTO.self, from: Data(json.utf8))
    #expect(dto.filterKey == nil)
  }

  @Test("toDomain carries filterKey to the domain model")
  func toDomain_carriesFilterKey() throws {
    let dto = WatchZoneSummaryDTO(
      id: "zone-ok",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      filterKey: .hmoHouseShares
    )

    let zone = try dto.toDomain()
    #expect(zone.filterKey == .hmoHouseShares)
  }

  @Test("toDomain without filterKey produces an unfiltered zone")
  func toDomain_withoutFilterKey_producesUnfilteredZone() throws {
    let dto = WatchZoneSummaryDTO(
      id: "zone-ok",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000
    )

    let zone = try dto.toDomain()
    #expect(zone.filterKey == nil)
  }
}
