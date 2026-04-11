import Testing
import TownCrierDomain

@Suite("UserProfileRepository protocol")
struct UserProfileRepositoryTests {

  @Test("protocol requires create method returning ServerProfile")
  func createReturnsServerProfile() async throws {
    let spy = SpyUserProfileRepository()
    spy.createResult = .success(ServerProfile(
      userId: "auth0|user-001",
      tier: .free,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true
    ))

    let profile = try await spy.create()

    #expect(spy.createCallCount == 1)
    #expect(profile.userId == "auth0|user-001")
    #expect(profile.tier == .free)
  }

  @Test("protocol requires fetch method returning optional ServerProfile")
  func fetchReturnsOptionalServerProfile() async throws {
    let spy = SpyUserProfileRepository()
    spy.fetchResult = .success(ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: false,
      digestDay: .friday,
      emailDigestEnabled: true
    ))

    let profile = try await spy.fetch()

    #expect(spy.fetchCallCount == 1)
    #expect(profile?.tier == .personal)
  }

  @Test("fetch returns nil when profile not found")
  func fetchReturnsNilWhenNotFound() async throws {
    let spy = SpyUserProfileRepository()
    spy.fetchResult = .success(nil)

    let profile = try await spy.fetch()

    #expect(profile == nil)
  }

  @Test("protocol requires update method with mutable preferences")
  func updateSendsPreferences() async throws {
    let spy = SpyUserProfileRepository()
    spy.updateResult = .success(ServerProfile(
      userId: "auth0|user-001",
      tier: .personal,
      pushEnabled: false,
      digestDay: .wednesday,
      emailDigestEnabled: false
    ))

    let updated = try await spy.update(
      pushEnabled: false,
      digestDay: .wednesday,
      emailDigestEnabled: false
    )

    #expect(spy.updateCalls.count == 1)
    #expect(spy.updateCalls[0].pushEnabled == false)
    #expect(spy.updateCalls[0].digestDay == .wednesday)
    #expect(spy.updateCalls[0].emailDigestEnabled == false)
    #expect(updated.pushEnabled == false)
  }

  @Test("protocol requires delete method")
  func deleteCallsThrough() async throws {
    let spy = SpyUserProfileRepository()
    spy.deleteResult = .success(())

    try await spy.delete()

    #expect(spy.deleteCallCount == 1)
  }
}
