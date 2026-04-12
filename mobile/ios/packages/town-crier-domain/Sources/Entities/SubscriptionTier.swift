/// Subscription level determining feature access.
///
/// Cases are ordered from lowest to highest entitlement so that
/// `Comparable` picks the richer tier when merging multiple sources
/// (e.g. server profile vs StoreKit).
public enum SubscriptionTier: String, Equatable, Hashable, Sendable, Comparable {
  case free
  case personal
  case pro

  // MARK: - Comparable

  private var rank: Int {
    switch self {
    case .free:
      0
    case .personal:
      1
    case .pro:
      2
    }
  }

  public static func < (lhs: SubscriptionTier, rhs: SubscriptionTier) -> Bool {
    lhs.rank < rhs.rank
  }
}
