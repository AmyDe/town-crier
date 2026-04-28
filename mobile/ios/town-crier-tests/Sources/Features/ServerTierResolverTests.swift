import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ServerTierResolver")
struct ServerTierResolverTests {
  private func makeProfile(tier: SubscriptionTier) -> ServerProfile {
    ServerProfile(
      userId: "u1",
      tier: tier,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true
    )
  }

  @Test func ensureServerProfileTier_callsCreate_returnsProfileTier() async {
    let spy = SpyUserProfileRepository()
    spy.createResult = .success(makeProfile(tier: .pro))
    let sut = ServerTierResolver(userProfileRepository: spy)

    let tier = await sut.ensureServerProfileTier()

    #expect(tier == .pro)
    #expect(spy.createCallCount == 1)
  }

  @Test func ensureServerProfileTier_returnsNil_whenCreateFails() async {
    let spy = SpyUserProfileRepository()
    spy.createResult = .failure(DomainError.networkUnavailable)
    let sut = ServerTierResolver(userProfileRepository: spy)

    let tier = await sut.ensureServerProfileTier()

    #expect(tier == nil)
    #expect(spy.createCallCount == 1)
  }

  @Test func ensureServerProfileTier_returnsFreeTier_whenServerSaysFree() async {
    let spy = SpyUserProfileRepository()
    spy.createResult = .success(makeProfile(tier: .free))
    let sut = ServerTierResolver(userProfileRepository: spy)

    let tier = await sut.ensureServerProfileTier()

    #expect(tier == .free)
  }
}
