import Foundation

/// A snapshot of an anonymous (pre-signup) browsing session: the postcode the
/// user entered before creating an account, its resolved coordinate, and when
/// it was captured (GH#868 Phase 3).
///
/// Persisted locally via ``AnonymousBrowseStateRepository`` so a relaunch with
/// no authenticated session routes straight back to the anonymous map instead
/// of the welcome screen, and handed to the onboarding wizard post-signup so
/// its postcode step can be pre-filled rather than asked twice.
public struct AnonymousBrowseState: Equatable, Sendable {
  public let postcode: Postcode
  public let coordinate: Coordinate
  /// The live monitoring radius the user chose on the anonymous map before
  /// signing up (GH#868 Phase 3 refinement), carried into onboarding so the
  /// wizard's radius step lands pre-set rather than reset to a default.
  /// Defaults to 2000m (the free tier's cap) for the brief window between
  /// postcode resolution and the user's first radius-slider interaction.
  public let radiusMetres: Double
  public let createdAt: Date

  public init(
    postcode: Postcode, coordinate: Coordinate, radiusMetres: Double = 2000, createdAt: Date
  ) {
    self.postcode = postcode
    self.coordinate = coordinate
    self.radiusMetres = radiusMetres
    self.createdAt = createdAt
  }
}
