import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierData

/// Pre-canned filter (`filterKey`) DTO decode/mapping tests (GH#1098/
/// tc-hogkn, GH#1104/tc-m8j90.2). Split out of `WatchZoneSummaryDTOTests` to
/// keep that file under the `file_length` SwiftLint threshold, mirroring the
/// existing split into `APIWatchZoneRepositoryBoundaryTests`. `filterKey` is
/// decoded as an opaque `String?` -- any value round-trips verbatim, there
/// is no more enum-backed "recognised vs unrecognised" distinction at
/// decode time (that distinction moved to display, see
/// `WatchZoneEditorViewModel.filterDisplayName(for:)`).
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

  @Test("decoding DTO with a filterKey preserves the exact string")
  func decoding_filterKey_isPreservedVerbatim() throws {
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
    #expect(dto.filterKey == "loft_extension")
  }

  /// GH#1104/tc-m8j90.2's core fix for tc-8z9ri: a `filterKey` this client
  /// doesn't recognise no longer fails open to `nil` at decode time (the old
  /// `WatchZoneFilterKey(rawValue:)` conversion did) -- it decodes verbatim
  /// as an opaque string, same as any other value. Only *display* falls
  /// back gracefully now, never the stored value.
  @Test("decoding DTO with an unrecognised filterKey string preserves it verbatim, not nil")
  func decoding_unrecognisedFilterKey_preservesVerbatim() throws {
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
    #expect(dto.filterKey == "some_future_filter_not_yet_shipped")
  }

  @Test("toDomain carries filterKey to the domain model")
  func toDomain_carriesFilterKey() throws {
    let dto = WatchZoneSummaryDTO(
      id: "zone-ok",
      name: "CB1 2AD",
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      filterKey: "hmo_house_shares"
    )

    let zone = try dto.toDomain()
    #expect(zone.filterKey == "hmo_house_shares")
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
