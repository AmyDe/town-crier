/// Errors originating from the domain layer.
public enum DomainError: Error, Equatable, Sendable {
  case invalidPostcode(String)
  case invalidStatusTransition(from: ApplicationStatus, to: ApplicationStatus)
  case applicationNotFound(PlanningApplicationId)
  case invalidCoordinate
  case invalidWatchZoneRadius
  case invalidWatchZoneName
  /// A custom-shape watch zone boundary has fewer than 3 or more than 50
  /// distinct vertices. Mirrors the server's `ErrBoundaryVertexCount`
  /// (`watchzones.NewBoundary`).
  case invalidWatchZoneBoundaryVertexCount
  /// A custom-shape watch zone boundary has two non-adjacent vertices with
  /// identical coordinates. Mirrors the server's `ErrBoundaryDuplicateVertex`.
  case invalidWatchZoneBoundaryDuplicateVertex
  /// A custom-shape watch zone boundary has two non-adjacent edges that
  /// cross, or is degenerate (zero-area/collinear). Mirrors the server's
  /// `ErrBoundarySelfIntersecting`.
  case invalidWatchZoneBoundarySelfIntersecting
  /// A custom-shape watch zone boundary has a vertex outside the UK bounding
  /// box (or a non-finite coordinate). Mirrors the server's
  /// `ErrBoundaryOutOfBounds`.
  case invalidWatchZoneBoundaryOutOfBounds
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
  /// Attempted to save a NEW device-local zone while already at
  /// ``DeviceLocalZone/maxZoneCount`` (GH#888: now 1, matching the Free
  /// tier). The Zones tab UI no longer offers a create path — only edit — so
  /// this now guards only defensively (e.g. a stale client); the repository
  /// cap stays in place regardless. Distinct from `.insufficientEntitlement`:
  /// this is an on-device cap with no subscription tier behind it, so the UI
  /// routes to a sign-up CTA rather than the paywall.
  case deviceLocalZoneLimitReached

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
    case .deviceLocalZoneLimitReached:
      return "Area Limit Reached"
    case .invalidPostcode:
      return "Invalid Postcode"
    case .geocodingFailed:
      return "Postcode Not Found"
    case .invalidWatchZoneBoundaryVertexCount, .invalidWatchZoneBoundaryDuplicateVertex,
      .invalidWatchZoneBoundarySelfIntersecting, .invalidWatchZoneBoundaryOutOfBounds:
      return "Invalid Area"
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
      return "This feature is on a higher plan. Upgrade to use it."
    case .deviceLocalZoneLimitReached:
      return "You can save one area on this device. Create a free account to add more."
    case .invalidPostcode(let raw):
      return "The postcode '\(raw)' doesn't look right. Please enter a valid UK postcode."
    case .geocodingFailed:
      return "We couldn't find the location for that postcode. Please check and try again."
    case .invalidWatchZoneBoundaryVertexCount:
      return "A custom area needs between 3 and 50 points."
    case .invalidWatchZoneBoundaryDuplicateVertex:
      return "A custom area can't have two points in the same place."
    case .invalidWatchZoneBoundarySelfIntersecting:
      return "This shape crosses itself. Try drawing it again without crossing lines."
    case .invalidWatchZoneBoundaryOutOfBounds:
      return "A custom area must be drawn within the UK."
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
      .purchaseCancelled, .productNotFound, .insufficientEntitlement,
      .deviceLocalZoneLimitReached, .invalidWatchZoneBoundaryVertexCount,
      .invalidWatchZoneBoundaryDuplicateVertex, .invalidWatchZoneBoundarySelfIntersecting,
      .invalidWatchZoneBoundaryOutOfBounds:
      return false
    }
  }
}
