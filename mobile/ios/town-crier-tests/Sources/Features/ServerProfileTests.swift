import Testing
import TownCrierDomain

@Suite("ServerProfile")
struct ServerProfileTests {

  @Test("stores all properties from GET /v1/me response")
  func storesAllProperties() {
    let profile = ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: false
    )

    #expect(profile.userId == "auth0|user-001")
    #expect(profile.tier == .personal)
    #expect(profile.pushEnabled == true)
    #expect(profile.digestDay == .monday)
    #expect(profile.emailDigestEnabled == false)
  }

  @Test("equality compares all fields")
  func equality() {
    let profile = ServerProfile(
      userId: "auth0|user-001",
      tier: .free,
      pushEnabled: true,
      digestDay: .wednesday,
      emailDigestEnabled: true
    )
    let identical = ServerProfile(
      userId: "auth0|user-001",
      tier: .free,
      pushEnabled: true,
      digestDay: .wednesday,
      emailDigestEnabled: true
    )
    let differentTier = ServerProfile(
      userId: "auth0|user-001",
      tier: .pro,
      pushEnabled: true,
      digestDay: .wednesday,
      emailDigestEnabled: true
    )

    #expect(profile == identical)
    #expect(profile != differentTier)
  }

  @Test("free tier profile has default values")
  func freeTierDefaults() {
    let profile = ServerProfile(
      userId: "auth0|free-user",
      tier: .free,
      pushEnabled: false,
      digestDay: .monday,
      emailDigestEnabled: true
    )

    #expect(profile.tier == .free)
    #expect(profile.pushEnabled == false)
  }
}
