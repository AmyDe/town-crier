import Foundation
import Testing

@testable import TownCrierDomain

@Suite("AuthSession")
struct AuthSessionTests {
  @Test func isExpired_returnsFalse_whenExpiresAtIsInFuture() {
    let session = AuthSession.valid
    #expect(!session.isExpired)
  }

  @Test func isExpired_returnsTrue_whenExpiresAtIsInPast() {
    let session = AuthSession.expired
    #expect(session.isExpired)
  }

  // MARK: - subscriptionTier

  @Test func subscriptionTier_storesProvidedTier() {
    let session = AuthSession(
      accessToken: "token",
      idToken: "id",
      expiresAt: Date.distantFuture,
      userProfile: .testUser,
      subscriptionTier: .pro
    )
    #expect(session.subscriptionTier == .pro)
  }

  @Test func subscriptionTier_defaultsToFree_whenOmitted() {
    let session = AuthSession(
      accessToken: "token",
      idToken: "id",
      expiresAt: Date.distantFuture,
      userProfile: .testUser
    )
    #expect(session.subscriptionTier == .free)
  }

  @Test func equality_considersTier() {
    let free = AuthSession(
      accessToken: "token",
      idToken: "id",
      expiresAt: Date.distantFuture,
      userProfile: .testUser,
      subscriptionTier: .free
    )
    let pro = AuthSession(
      accessToken: "token",
      idToken: "id",
      expiresAt: Date.distantFuture,
      userProfile: .testUser,
      subscriptionTier: .pro
    )
    #expect(free != pro)
  }
}
