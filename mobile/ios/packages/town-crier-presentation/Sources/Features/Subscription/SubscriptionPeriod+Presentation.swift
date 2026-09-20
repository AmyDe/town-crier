import TownCrierDomain

extension SubscriptionPeriod {
  var pickerTitle: String {
    switch self {
    case .monthly: "Monthly"
    case .annual: "Yearly"
    case .lifetime: "Lifetime"
    }
  }
}
