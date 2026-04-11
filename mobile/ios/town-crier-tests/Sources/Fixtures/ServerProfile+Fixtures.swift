import TownCrierDomain

extension ServerProfile {
  static let freeUser = ServerProfile(
    userId: "auth0|user-001",
    tier: .free,
    pushEnabled: true,
    digestDay: .monday,
    emailDigestEnabled: true
  )

  static let personalUser = ServerProfile(
    userId: "auth0|user-001",
    tier: .personal,
    pushEnabled: false,
    digestDay: .wednesday,
    emailDigestEnabled: true
  )

  static let proUser = ServerProfile(
    userId: "auth0|user-001",
    tier: .pro,
    pushEnabled: true,
    digestDay: .friday,
    emailDigestEnabled: false
  )
}
