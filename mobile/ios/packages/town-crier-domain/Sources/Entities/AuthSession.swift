import Foundation

/// Represents an authenticated user session with tokens and profile.
public struct AuthSession: Equatable, Sendable {
  public let accessToken: String
  public let idToken: String
  public let expiresAt: Date
  public let userProfile: UserProfile

  public init(
    accessToken: String,
    idToken: String,
    expiresAt: Date,
    userProfile: UserProfile
  ) {
    self.accessToken = accessToken
    self.idToken = idToken
    self.expiresAt = expiresAt
    self.userProfile = userProfile
  }

  /// Whether the session's access token has expired.
  public var isExpired: Bool {
    expiresAt <= Date.now
  }
}
