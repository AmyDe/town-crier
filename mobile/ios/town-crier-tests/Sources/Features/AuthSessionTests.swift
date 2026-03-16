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
}
