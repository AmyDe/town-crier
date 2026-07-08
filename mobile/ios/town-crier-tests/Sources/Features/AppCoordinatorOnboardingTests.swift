import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator -- Onboarding Gating")
@MainActor
struct AppCoordinatorOnboardingTests {
  private func makeSUT(
    onboardingRepository: SpyOnboardingRepository = SpyOnboardingRepository(),
    watchZoneRepository: SpyWatchZoneRepository = SpyWatchZoneRepository(),
    tierResolver: SubscriptionTierResolving? = nil,
    anonymousBrowseStateRepository: SpyAnonymousBrowseStateRepository? = nil
  ) -> AppCoordinator {
    // Isolated tier cache so a leftover `cachedSubscriptionTier` from another
    // test can't seed a non-free tier at init.
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    return AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      tierResolver: tierResolver,
      watchZoneRepository: watchZoneRepository,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: onboardingRepository,
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      tierCache: defaults,
      anonymousBrowseStateRepository: anonymousBrowseStateRepository
    )
  }

  // MARK: - Initial presentation state

  @Test func onboardingPresentation_defaultsToUndetermined_whenNotPreviouslyComplete() {
    let sut = makeSUT()

    #expect(sut.onboardingPresentation == .undetermined)
  }

  @Test func onboardingPresentation_seedsNotRequired_whenAlreadyCompletedOnDevice() {
    let onboardingRepo = SpyOnboardingRepository()
    onboardingRepo.isOnboardingComplete = true

    let sut = makeSUT(onboardingRepository: onboardingRepo)

    #expect(sut.onboardingPresentation == .notRequired)
  }

  // MARK: - determineOnboarding (account-state gate)

  @Test func determineOnboarding_withNoWatchZones_requiresOnboarding() async {
    let watchZones = SpyWatchZoneRepository()
    watchZones.loadAllResult = .success([])
    let sut = makeSUT(watchZoneRepository: watchZones)

    await sut.determineOnboarding()

    #expect(sut.onboardingPresentation == .required)
  }

  @Test func determineOnboarding_withExistingWatchZones_skipsOnboarding() async {
    let watchZones = SpyWatchZoneRepository()
    watchZones.loadAllResult = .success([.cambridge])
    let sut = makeSUT(watchZoneRepository: watchZones)

    await sut.determineOnboarding()

    #expect(sut.onboardingPresentation == .notRequired)
  }

  @Test func determineOnboarding_whenLoadFails_fallsThroughToApp() async {
    let watchZones = SpyWatchZoneRepository()
    watchZones.loadAllResult = .failure(DomainError.networkUnavailable)
    let sut = makeSUT(watchZoneRepository: watchZones)

    await sut.determineOnboarding()

    // A failed determination must not trap the user behind a loading screen
    // or in the wizard — fall through to the app.
    #expect(sut.onboardingPresentation == .notRequired)
  }

  // MARK: - Wizard factory

  @Test func makeOnboardingViewModel_injectsCurrentTier() {
    let sut = makeSUT()

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.subscriptionTier == .free)
  }

  @Test func makeOnboardingViewModel_returnsStableInstanceAcrossCalls() {
    let sut = makeSUT()

    let first = sut.makeOnboardingViewModel()
    let second = sut.makeOnboardingViewModel()

    #expect(first === second)
  }

  @Test func makeOnboardingViewModel_onComplete_marksOnboardingNotRequired() {
    let sut = makeSUT()
    let vm = sut.makeOnboardingViewModel()

    vm.onComplete?(.cambridge)

    #expect(sut.onboardingPresentation == .notRequired)
  }

  // MARK: - Live tier observation (tc-w3cb.1 / .3)

  @Test func resolveSubscriptionTier_pushesResolvedTierIntoLiveWizard() async {
    let resolver = FakeSubscriptionTierResolver()
    resolver.resolveResult = (.pro, false)
    let sut = makeSUT(tierResolver: resolver)
    let vm = sut.makeOnboardingViewModel()
    #expect(vm.subscriptionTier == .free)

    await sut.resolveSubscriptionTier()

    // The wizard observes the change in place — it is not rebuilt, so any
    // in-progress postcode/geocode survives the tier change.
    #expect(vm.subscriptionTier == .pro)
  }

  // MARK: - In-wizard radius upsell (tc-w3cb.3)

  @Test func makeOnboardingViewModel_wiresUpsellPaywallFactory() {
    let sut = makeSUT()
    let vm = sut.makeOnboardingViewModel()

    // The coordinator injects the paywall factory so the wizard can present
    // the subscription sheet over itself.
    #expect(vm.makeUpsellViewModel?() != nil)
  }

  @Test func reconcileTierAfterUpgrade_reResolvesTier_unlockingLargerRadiusLive() async {
    let resolver = FakeSubscriptionTierResolver()
    resolver.resolveResult = (.pro, false)
    let sut = makeSUT(tierResolver: resolver)
    let vm = sut.makeOnboardingViewModel()
    #expect(vm.canUnlockLargerRadius)
    #expect(vm.maxRadiusMetres == 2000)

    // Simulates the paywall dismissing after a successful purchase.
    await vm.reconcileTierAfterUpgrade()

    // Live unlock: the same wizard instance now reflects the upgraded tier.
    #expect(vm.subscriptionTier == .pro)
    #expect(vm.maxRadiusMetres == 10000)
    #expect(!vm.canUnlockLargerRadius)
  }

  // MARK: - Anonymous browse post-signup handoff (GH#868 Phase 3.5)

  @Test func makeOnboardingViewModel_withAnonymousState_prefillsAndClearsState() throws {
    let anonymousRepo = SpyAnonymousBrowseStateRepository()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    anonymousRepo.loadResult = AnonymousBrowseState(
      postcode: postcode, coordinate: coordinate, createdAt: Date())
    let sut = makeSUT(anonymousBrowseStateRepository: anonymousRepo)

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.postcodeInput == "CB1 2AD")
    #expect(vm.geocodedCoordinate == coordinate)
    #expect(vm.currentStep == .radiusPicker)
    #expect(anonymousRepo.clearCallCount == 1)
  }

  @Test func makeOnboardingViewModel_withNoAnonymousState_startsAtWelcome() {
    let anonymousRepo = SpyAnonymousBrowseStateRepository()
    let sut = makeSUT(anonymousBrowseStateRepository: anonymousRepo)

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.currentStep == .welcome)
    #expect(anonymousRepo.clearCallCount == 0)
  }

  @Test func makeOnboardingViewModel_calledAgain_doesNotReapplyPrefill() throws {
    let anonymousRepo = SpyAnonymousBrowseStateRepository()
    anonymousRepo.loadResult = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"),
      coordinate: try Coordinate(latitude: 52.2053, longitude: 0.1218),
      createdAt: Date()
    )
    let sut = makeSUT(anonymousBrowseStateRepository: anonymousRepo)

    let first = sut.makeOnboardingViewModel()
    let second = sut.makeOnboardingViewModel()

    #expect(first === second)
    // The cached-instance path never re-reads the repository, so `clear()`
    // fires exactly once across both calls.
    #expect(anonymousRepo.clearCallCount == 1)
  }
}
