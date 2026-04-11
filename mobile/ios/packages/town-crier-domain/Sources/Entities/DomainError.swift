/// Errors originating from the domain layer.
public enum DomainError: Error, Equatable, Sendable {
    case invalidPostcode(String)
    case invalidStatusTransition(from: ApplicationStatus, to: ApplicationStatus)
    case applicationNotFound(PlanningApplicationId)
    case invalidCoordinate
    case invalidWatchZoneRadius
    case networkUnavailable
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
        case .invalidPostcode, .invalidCoordinate, .invalidWatchZoneRadius,
             .invalidStatusTransition, .geocodingFailed,
             .notificationPermissionDenied, .unexpected:
            return "Something Went Wrong"
        }
    }

    /// User-facing message for display in error states.
    public var userMessage: String {
        switch self {
        case .networkUnavailable:
            return "Check your internet connection and try again."
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
        case .invalidPostcode, .invalidCoordinate, .invalidWatchZoneRadius,
             .invalidStatusTransition, .geocodingFailed, .unexpected:
            return "An unexpected error occurred. Please try again."
        }
    }

    /// Whether the error is transient and retrying may succeed.
    public var isRetryable: Bool {
        switch self {
        case .networkUnavailable, .unexpected, .geocodingFailed,
             .purchaseFailed, .restoreFailed, .logoutFailed:
            return true
        case .sessionExpired, .authenticationFailed, .invalidPostcode,
             .invalidCoordinate, .invalidWatchZoneRadius, .invalidStatusTransition,
             .applicationNotFound, .notificationPermissionDenied,
             .purchaseCancelled, .productNotFound, .insufficientEntitlement:
            return false
        }
    }
}
