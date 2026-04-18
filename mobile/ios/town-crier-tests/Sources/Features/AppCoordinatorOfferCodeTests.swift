import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator — Offer Codes")
@MainActor
struct AppCoordinatorOfferCodeTests {

  // MARK: - Helpers

  private func makeSUT(
    offerCodeService: OfferCodeService? = nil,
    authSession: AuthSession? = nil,
    entitlement: SubscriptionEntitlement? = nil,
    serverProfile: ServerProfile? = nil,
    tierCache: UserDefaults = UserDefaults(suiteName: UUID().uuidString) ?? .standard
  ) -> (AppCoordinator, SpyAuthenticationService, SpyUserProfileRepository) {
    let authSpy = SpyAuthenticationService()
    authSpy.currentSessionResult = authSession
    let subscriptionSpy = SpySubscriptionService()
    subscriptionSpy.currentEntitlementResult = entitlement
    let profileSpy = SpyUserProfileRepository()
    profileSpy.fetchResult = .success(serverProfile)
    let coordinator = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: authSpy,
      subscriptionService: subscriptionSpy,
      userProfileRepository: profileSpy,
      watchZoneRepository: SpyWatchZoneRepository(),
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      offerCodeService: offerCodeService,
      tierCache: tierCache
    )
    return (coordinator, authSpy, profileSpy)
  }

  private func makeServerProfile(tier: SubscriptionTier) -> ServerProfile {
    ServerProfile(
      userId: "u1",
      tier: tier,
      pushEnabled: true,
      digestDay: .monday,
      emailDigestEnabled: true
    )
  }

  // MARK: - Presentation flag

  @Test("isRedeemOfferCodePresented defaults to false")
  func isRedeemOfferCodePresented_defaultsToFalse() {
    let (sut, _, _) = makeSUT(offerCodeService: SpyOfferCodeService())

    #expect(sut.isRedeemOfferCodePresented == false)
  }

  @Test("showRedeemOfferCode flips isRedeemOfferCodePresented to true")
  func showRedeemOfferCode_presentsSheet() {
    let (sut, _, _) = makeSUT(offerCodeService: SpyOfferCodeService())

    sut.showRedeemOfferCode()

    #expect(sut.isRedeemOfferCodePresented == true)
  }

  // MARK: - Factory

  @Test("makeRedeemOfferCodeViewModel returns nil when no OfferCodeService is wired")
  func makeRedeemOfferCodeViewModel_withoutService_returnsNil() {
    let (sut, _, _) = makeSUT(offerCodeService: nil)

    #expect(sut.makeRedeemOfferCodeViewModel() == nil)
  }

  @Test("makeRedeemOfferCodeViewModel returns a viewmodel when service is wired")
  func makeRedeemOfferCodeViewModel_withService_returnsViewModel() {
    let (sut, _, _) = makeSUT(offerCodeService: SpyOfferCodeService())

    let vm = sut.makeRedeemOfferCodeViewModel()

    #expect(vm != nil)
  }

  // MARK: - Post-redemption refresh

  @Test("onRedeemed refreshes the session via the auth service")
  func onRedeemed_refreshesSession() async {
    let service = SpyOfferCodeService()
    let (sut, authSpy, _) = makeSUT(offerCodeService: service)
    guard let vm = sut.makeRedeemOfferCodeViewModel() else {
      Issue.record("expected viewmodel")
      return
    }
    vm.code = "A7KM-ZQR3-FNXP"

    await vm.redeem()
    // Let the fire-and-forget refresh Task complete.
    await sut.waitForPendingOfferCodeRefresh()

    #expect(authSpy.refreshSessionCallCount == 1)
  }

  @Test("onRedeemed re-resolves the subscription tier from the server profile")
  func onRedeemed_resolvesTierFromServer() async {
    let service = SpyOfferCodeService()
    let (sut, _, profileSpy) = makeSUT(
      offerCodeService: service,
      serverProfile: makeServerProfile(tier: .pro)
    )
    guard let vm = sut.makeRedeemOfferCodeViewModel() else {
      Issue.record("expected viewmodel")
      return
    }
    vm.code = "A7KM-ZQR3-FNXP"

    await vm.redeem()
    await sut.waitForPendingOfferCodeRefresh()

    #expect(sut.subscriptionTier == .pro)
    #expect(profileSpy.fetchCallCount >= 1)
  }

  @Test("onRedeemed dismisses the offer-code sheet")
  func onRedeemed_dismissesSheet() async {
    let service = SpyOfferCodeService()
    let (sut, _, _) = makeSUT(offerCodeService: service)
    sut.showRedeemOfferCode()
    guard let vm = sut.makeRedeemOfferCodeViewModel() else {
      Issue.record("expected viewmodel")
      return
    }
    vm.code = "A7KM-ZQR3-FNXP"

    await vm.redeem()
    await sut.waitForPendingOfferCodeRefresh()

    #expect(sut.isRedeemOfferCodePresented == false)
  }

  @Test("onRedeemed is not invoked on redemption failure")
  func onRedeemedCallback_notInvokedOnFailure() async {
    let service = SpyOfferCodeService()
    service.redeemResult = .failure(OfferCodeError.notFound)
    let (sut, authSpy, _) = makeSUT(offerCodeService: service)
    guard let vm = sut.makeRedeemOfferCodeViewModel() else {
      Issue.record("expected viewmodel")
      return
    }
    vm.code = "A7KM-ZQR3-FNXP"

    await vm.redeem()
    await sut.waitForPendingOfferCodeRefresh()

    #expect(authSpy.refreshSessionCallCount == 0)
  }
}
