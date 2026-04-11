import Foundation

/// Represents an authenticated user session with tokens and profile.
public struct AuthSession: Equatable, Sendable {
  public let accessToken: String
  public let idToken: String
  public let expiresAt: Date
  public let userProfile: UserProfile
  /// The user's subscription tier extracted from the JWT access token.
  /// Defaults to `.free` when the claim is absent.
  public let subscriptionTier: SubscriptionTier

  public init(
    accessToken: String,
    idToken: String,
    expiresAt: Date,
    userProfile: UserProfile,
    subscriptionTier: SubscriptionTier = .free
  ) {
    self.accessToken = accessToken
    self.idToken = idToken
    self.expiresAt = expiresAt
    self.userProfile = userProfile
    self.subscriptionTier = subscriptionTier
  }

  /// Whether the session's access token has expired.
  public var isExpired: Bool {
    expiresAt <= Date.now
  }
}
