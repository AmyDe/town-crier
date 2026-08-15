import Testing

@testable import TownCrierDomain

/// Catalog completeness for the pre-canned watch-zone filter keys (GH#1098,
/// bead tc-hogkn). Mirrors the server's 7-entry catalog
/// (`api-go/internal/watchzones/filter.go`, GH#1090).
@Suite("WatchZoneFilterKey")
struct WatchZoneFilterKeyTests {
  @Test func allCases_hasSevenEntries() {
    #expect(WatchZoneFilterKey.allCases.count == 7)
  }

  @Test func allCases_everyCaseHasNonEmptyDisplayName() {
    for filterKey in WatchZoneFilterKey.allCases {
      #expect(!filterKey.displayName.isEmpty)
    }
  }

  @Test func allCases_everyCaseHasNonEmptyDescription() {
    for filterKey in WatchZoneFilterKey.allCases {
      #expect(!filterKey.description.isEmpty)
    }
  }

  @Test func rawValues_matchServerCatalogKeys() {
    #expect(WatchZoneFilterKey.fewerNotifications.rawValue == "fewer_notifications")
    #expect(WatchZoneFilterKey.houseBuilder.rawValue == "house_builder")
    #expect(WatchZoneFilterKey.newHomes.rawValue == "new_homes")
    #expect(WatchZoneFilterKey.extensionsAlterations.rawValue == "extensions_alterations")
    #expect(WatchZoneFilterKey.loftExtension.rawValue == "loft_extension")
    #expect(WatchZoneFilterKey.kitchenExtension.rawValue == "kitchen_extension")
    #expect(WatchZoneFilterKey.hmoHouseShares.rawValue == "hmo_house_shares")
  }

  /// GH#1090's explicit copy constraint: the kitchen filter must not imply
  /// true kitchen detection, since the server can't detect kitchen work
  /// specifically from a planning description.
  @Test func kitchenExtension_descriptionDoesNotClaimTrueKitchenDetection() {
    let description = WatchZoneFilterKey.kitchenExtension.description.lowercased()
    #expect(!description.contains("detects kitchen"))
    #expect(description.contains("can't detect kitchen"))
  }

  @Test func initFromRawValue_recognisedString_returnsCase() {
    #expect(WatchZoneFilterKey(rawValue: "hmo_house_shares") == .hmoHouseShares)
  }

  @Test func initFromRawValue_unrecognisedString_returnsNil() {
    #expect(WatchZoneFilterKey(rawValue: "not_a_real_filter") == nil)
  }
}
