/// The user's server-side profile as returned by `GET /v1/me`.
///
/// Distinct from `UserProfile` which holds authentication identity info.
/// `ServerProfile` holds the API-managed profile with subscription tier
/// and notification preferences.
public struct ServerProfile: Equatable, Sendable {
  public let userId: String
  public let tier: SubscriptionTier
  public let pushEnabled: Bool
  public let digestDay: DayOfWeek
  public let emailDigestEnabled: Bool

  public init(
    userId: String,
    tier: SubscriptionTier,
    pushEnabled: Bool,
    digestDay: DayOfWeek,
    emailDigestEnabled: Bool
  ) {
    self.userId = userId
    self.tier = tier
    self.pushEnabled = pushEnabled
    self.digestDay = digestDay
    self.emailDigestEnabled = emailDigestEnabled
  }
}
