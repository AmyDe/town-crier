import TownCrierDomain

/// Navigation targets reachable via deep links or notification taps.
public enum DeepLink: Equatable, Sendable {
  case applicationDetail(PlanningApplicationId)
  /// The Applications tab root — used when a Universal Link points at
  /// `/applications` exactly (e.g. the digest email's bottom CTA).
  case applicationsList
}
