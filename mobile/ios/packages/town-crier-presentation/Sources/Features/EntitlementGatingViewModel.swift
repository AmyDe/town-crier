import TownCrierDomain

/// A protocol for ViewModels that intercept `DomainError.insufficientEntitlement`
/// and present the subscription upsell sheet instead of a generic error state.
///
/// Conformers must provide both `error` (from ``ErrorHandlingViewModel``) and
/// `entitlementGate`. The default `handleError` override routes entitlement
/// errors to the gate binding while forwarding all other errors normally.
///
/// Usage in a ViewModel:
/// ```swift
/// @MainActor
/// final class SearchViewModel: ObservableObject, EntitlementGatingViewModel {
///     @Published var error: DomainError?
///     @Published var entitlementGate: Entitlement?
///     ...
///     func search() async {
///         do { ... } catch { handleError(error) }
///     }
/// }
/// ```
@MainActor
protocol EntitlementGatingViewModel: ErrorHandlingViewModel {
  var entitlementGate: Entitlement? { get set }
}

extension EntitlementGatingViewModel {
  func handleError(
    _ error: Error,
    fallback: (String) -> DomainError = { .unexpected($0) }
  ) {
    if let domainError = error as? DomainError,
      case .insufficientEntitlement(let required) = domainError
    {
      if let entitlement = Entitlement(rawValue: required) {
        self.entitlementGate = entitlement
        self.error = nil
      } else {
        self.entitlementGate = nil
        self.error = domainError
      }
    } else {
      self.entitlementGate = nil
      if let domainError = error as? DomainError {
        self.error = domainError
      } else {
        self.error = fallback(error.localizedDescription)
      }
    }
  }
}
