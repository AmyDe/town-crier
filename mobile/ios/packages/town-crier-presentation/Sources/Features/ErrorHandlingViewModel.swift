import TownCrierDomain

/// A protocol for ViewModels that handle errors by setting a `DomainError?` property.
///
/// Provides a shared `handleError(_:fallback:)` method that eliminates the
/// repeated catch boilerplate across ViewModels:
///
/// ```swift
/// } catch {
///     handleError(error)
/// }
/// ```
///
/// When the caught error is already a `DomainError`, it is assigned directly.
/// Otherwise, the `fallback` closure wraps the localised description into
/// the appropriate `DomainError` case (defaulting to `.unexpected`).
@MainActor
protocol ErrorHandlingViewModel: AnyObject {
    var error: DomainError? { get set }
}

extension ErrorHandlingViewModel {
    func handleError(
        _ error: Error,
        fallback: (String) -> DomainError = { .unexpected($0) }
    ) {
        if let domainError = error as? DomainError {
            self.error = domainError
        } else {
            self.error = fallback(error.localizedDescription)
        }
    }
}
