/// The billing period of a purchasable subscription product.
///
/// Cases are declared, and compare, in display order: monthly, annual, lifetime.
public enum SubscriptionPeriod: Int, Equatable, Hashable, Sendable, CaseIterable, Comparable {
  case monthly
  case annual
  case lifetime

  public static func < (lhs: SubscriptionPeriod, rhs: SubscriptionPeriod) -> Bool {
    lhs.rawValue < rhs.rawValue
  }
}
