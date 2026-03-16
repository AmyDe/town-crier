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
        userProfile: .testUser
    )

    static let expired = AuthSession(
        accessToken: "expired-access-token",
        idToken: "expired-id-token",
        expiresAt: Date.distantPast,
        userProfile: .testUser
    )
}
