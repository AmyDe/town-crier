/// A pre-canned notification filter for a watch zone (GH#1090/tc-w825j,
/// client-side GH#1098/tc-hogkn). Pro-tier only -- see
/// ``WatchZoneLimits/allowsWatchZoneFilter``.
///
/// This catalog must stay in sync with the server's `filterCatalog`
/// (`api-go/internal/watchzones/filter.go`) -- same duplication precedent as
/// `WatchZoneLimits`/`EntitlementMap` for tier limits. There is no
/// catalog-listing endpoint; the server only validates a submitted key
/// against its own embedded copy.
public enum WatchZoneFilterKey: String, CaseIterable, Equatable, Hashable, Sendable {
  case fewerNotifications = "fewer_notifications"
  case houseBuilder = "house_builder"
  case newHomes = "new_homes"
  case extensionsAlterations = "extensions_alterations"
  case loftExtension = "loft_extension"
  case kitchenExtension = "kitchen_extension"
  case hmoHouseShares = "hmo_house_shares"

  /// The picker row's primary label.
  public var displayName: String {
    switch self {
    case .fewerNotifications:
      return "Fewer notifications"
    case .houseBuilder:
      return "House builder"
    case .newHomes:
      return "New homes"
    case .extensionsAlterations:
      return "Extensions & alterations"
    case .loftExtension:
      return "Loft extension"
    case .kitchenExtension:
      return "Kitchen extension"
    case .hmoHouseShares:
      return "HMO / house shares"
    }
  }

  /// The picker row's secondary explanation, run through the `voice` skill.
  /// `kitchenExtension`'s description is deliberately honest about being a
  /// rear/single-storey-extension proxy, not true kitchen detection
  /// (GH#1090's explicit copy constraint).
  public var description: String {
    switch self {
    case .fewerNotifications:
      return
        "Cuts out tree works, conditions, amendments, adverts and telecoms applications, "
        + "so you get fewer notifications overall."
    case .houseBuilder:
      return "Deliberately broad: extensions, conversions, refurbishments, new builds and more."
    case .newHomes:
      return
        "New housing only: erecting, building or redeveloping dwellings, "
        + "plus new-build and self-build mentions."
    case .extensionsAlterations:
      return
        "Nearby building work: extensions, loft and garage conversions, outbuildings, "
        + "conservatories, dormers, porches, annexes and more."
    case .loftExtension:
      return "Loft conversions: loft, dormer, hip-to-gable and roof space mentions."
    case .kitchenExtension:
      return
        "We can't detect kitchen work from the planning description alone, so this shows "
        + "every rear or single-storey extension nearby instead. Most involve a kitchen."
    case .hmoHouseShares:
      return "Houses in multiple occupation, Class C4, and larger shared-house applications."
    }
  }
}
