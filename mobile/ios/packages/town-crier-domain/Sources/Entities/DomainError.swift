/// Errors originating from the domain layer.
public enum DomainError: Error, Equatable, Sendable {
  case invalidPostcode(String)
  case invalidStatusTransition(from: ApplicationStatus, to: ApplicationStatus)
  case applicationNotFound(PlanningApplicationId)
  case invalidCoordinate
  case invalidWatchZoneRadius
  case invalidWatchZoneName
  case networkUnavailable
  case serverError(statusCode: Int, message: String?)
  case authenticationFailed(String)
  case sessionExpired
  case logoutFailed(String)
  case geocodingFailed(String)
  case notificationPermissionDenied
  case purchaseFailed(String)
  case purchaseCancelled
  case productNotFound(String)
  case restoreFailed(String)
  case unexpected(String)
  case insufficientEntitlement(required: String)

  /// User-facing title for display in error states.
  public var userTitle: String {
    switch self {
    case .networkUnavailable:
      return "No Connection"
    case .serverError:
      return "Server Error"
    case .sessionExpired:
      return "Session Expired"
    case .authenticationFailed:
      return "Sign In Failed"
    case .logoutFailed:
      return "Sign Out Failed"
    case .purchaseFailed, .purchaseCancelled, .restoreFailed, .productNotFound:
      return "Purchase Error"
    case .applicationNotFound:
      return "Not Found"
    case .insufficientEntitlement:
      return "Upgrade Required"
    case .invalidPostcode:
      return "Invalid Postcode"
    case .geocodingFailed:
      return "Postcode Not Found"
    case .invalidCoordinate, .invalidWatchZoneRadius,
      .invalidWatchZoneName, .invalidStatusTransition,
      .notificationPermissionDenied, .unexpected:
      return "Something Went Wrong"
    }
  }

  /// User-facing message for display in error states.
  public var userMessage: String {
    switch self {
    case .networkUnavailable:
      return "Check your internet connection and try again."
    case .serverError:
      return "The server encountered an error. Please try again later."
    case .sessionExpired:
      return "Your session has expired. Please sign in again."
    case .authenticationFailed:
      return "Unable to sign in. Please try again."
    case .logoutFailed:
      return "Unable to sign out. Please try again."
    case .applicationNotFound:
      return "The planning application could not be found."
    case .notificationPermissionDenied:
      return "Notification permission is required. Enable it in Settings."
    case .purchaseFailed, .purchaseCancelled, .restoreFailed, .productNotFound:
      return "There was a problem with your purchase. Please try again."
    case .insufficientEntitlement:
      return "This feature requires a higher subscription tier. Upgrade to unlock it."
    case .invalidPostcode(let raw):
      return "The postcode '\(raw)' doesn't look right. Please enter a valid UK postcode."
    case .geocodingFailed:
      return "We couldn't find the location for that postcode. Please check and try again."
    case .invalidCoordinate, .invalidWatchZoneRadius,
      .invalidWatchZoneName, .invalidStatusTransition, .unexpected:
      return "An unexpected error occurred. Please try again."
    }
  }

  /// Whether the error is transient and retrying may succeed.
  public var isRetryable: Bool {
    switch self {
    case .networkUnavailable, .serverError, .unexpected, .geocodingFailed,
      .purchaseFailed, .restoreFailed, .logoutFailed, .sessionExpired:
      return true
    case .authenticationFailed, .invalidPostcode,
      .invalidCoordinate, .invalidWatchZoneRadius, .invalidWatchZoneName,
      .invalidStatusTransition, .applicationNotFound, .notificationPermissionDenied,
      .purchaseCancelled, .productNotFound, .insufficientEntitlement:
      return false
    }
  }
}
