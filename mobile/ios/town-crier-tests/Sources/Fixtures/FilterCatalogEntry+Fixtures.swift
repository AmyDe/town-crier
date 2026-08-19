import TownCrierDomain

/// Test fixtures mirroring the server's 7-entry pre-canned watch-zone filter
/// catalog (`api-go/internal/watchzones/filter.go`, GH#1090), for tests that
/// exercise a fetched ``FilterCatalogEntry`` catalog (GH#1104, tc-m8j90.2)
/// where the closed `WatchZoneFilterKey` enum's `allCases` used to be used.
extension FilterCatalogEntry {
  static let fewerNotifications = FilterCatalogEntry(
    key: "fewer_notifications",
    displayName: "Fewer notifications",
    description:
      "Cuts out tree works, conditions, amendments, adverts and telecoms applications, "
      + "so you get fewer notifications overall."
  )

  static let houseBuilder = FilterCatalogEntry(
    key: "house_builder",
    displayName: "House builder",
    description: "Deliberately broad: extensions, conversions, refurbishments, new builds and more."
  )

  static let newHomes = FilterCatalogEntry(
    key: "new_homes",
    displayName: "New homes",
    description:
      "New housing only: erecting, building or redeveloping dwellings, "
      + "plus new-build and self-build mentions."
  )

  static let extensionsAlterations = FilterCatalogEntry(
    key: "extensions_alterations",
    displayName: "Extensions & alterations",
    description:
      "Nearby building work: extensions, loft and garage conversions, outbuildings, "
      + "conservatories, dormers, porches, annexes and more."
  )

  static let loftExtension = FilterCatalogEntry(
    key: "loft_extension",
    displayName: "Loft extension",
    description: "Loft conversions: loft, dormer, hip-to-gable and roof space mentions."
  )

  static let kitchenExtension = FilterCatalogEntry(
    key: "kitchen_extension",
    displayName: "Kitchen extension",
    description:
      "We can't detect kitchen work from the planning description alone, so this shows "
      + "every rear or single-storey extension nearby instead. Most involve a kitchen."
  )

  static let hmoHouseShares = FilterCatalogEntry(
    key: "hmo_house_shares",
    displayName: "HMO / house shares",
    description: "Houses in multiple occupation, Class C4, and larger shared-house applications."
  )

  /// The full catalog, in the server's fixed declaration order.
  static let allCatalog: [FilterCatalogEntry] = [
    .fewerNotifications, .houseBuilder, .newHomes, .extensionsAlterations,
    .loftExtension, .kitchenExtension, .hmoHouseShares,
  ]
}
