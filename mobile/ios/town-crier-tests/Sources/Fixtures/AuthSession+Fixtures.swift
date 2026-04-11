import Foundation
import TownCrierDomain

extension UserProfile {
  static let testUser = UserProfile(
    userId: "auth0|user-001",
    email: "test@example.com",
    name: "Test User"
  )
}

extension AuthSession {
  static let valid = AuthSession(
    accessToken: "test-access-token",
    idToken: "test-id-token",
    expiresAt: Date.distantFuture,
    userProfile: .testUser,
    subscriptionTier: .free
  )

  static let expired = AuthSession(
    accessToken: "expired-access-token",
    idToken: "expired-id-token",
    expiresAt: Date.distantPast,
    userProfile: .testUser,
    subscriptionTier: .free
  )

  static let pro = AuthSession(
    accessToken: "pro-access-token",
    idToken: "pro-id-token",
    expiresAt: Date.distantFuture,
    userProfile: .testUser,
    subscriptionTier: .pro
  )

  static let personal = AuthSession(
    accessToken: "personal-access-token",
    idToken: "personal-id-token",
    expiresAt: Date.distantFuture,
    userProfile: .testUser,
    subscriptionTier: .personal
  )
}
