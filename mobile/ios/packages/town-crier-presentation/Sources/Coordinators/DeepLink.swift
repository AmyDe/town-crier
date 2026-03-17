import TownCrierDomain

/// Navigation targets reachable via deep links or notification taps.
public enum DeepLink: Equatable, Sendable {
    case applicationDetail(PlanningApplicationId)
}
