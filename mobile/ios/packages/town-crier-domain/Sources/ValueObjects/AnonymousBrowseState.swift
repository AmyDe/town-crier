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
  public let createdAt: Date

  public init(postcode: Postcode, coordinate: Coordinate, createdAt: Date) {
    self.postcode = postcode
    self.coordinate = coordinate
    self.createdAt = createdAt
  }
}
