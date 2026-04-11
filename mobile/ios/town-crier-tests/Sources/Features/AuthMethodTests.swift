import Foundation
import Testing
import TownCrierDomain

@Suite("AuthMethod derivation from UserProfile")
struct AuthMethodTests {
  @Test func emailPasswordUser_returnsEmailPassword() {
    let profile = UserProfile(userId: "auth0|abc123", email: "user@example.com")
    #expect(profile.authMethod == .emailPassword)
  }

  @Test func googleUser_returnsGoogle() {
    let profile = UserProfile(userId: "google-oauth2|112233", email: "user@gmail.com")
    #expect(profile.authMethod == .google)
  }

  @Test func appleUser_returnsApple() {
    let profile = UserProfile(userId: "apple|001122", email: "user@privaterelay.appleid.com")
    #expect(profile.authMethod == .apple)
  }

  @Test func unknownProvider_returnsUnknown() {
    let profile = UserProfile(userId: "someother|999", email: "user@example.com")
    #expect(profile.authMethod == .unknown)
  }

  @Test func authMethodDisplayName_returnsHumanReadableString() {
    #expect(AuthMethod.emailPassword.displayName == "Email & Password")
    #expect(AuthMethod.google.displayName == "Google")
    #expect(AuthMethod.apple.displayName == "Apple")
    #expect(AuthMethod.unknown.displayName == "Unknown")
  }
}
