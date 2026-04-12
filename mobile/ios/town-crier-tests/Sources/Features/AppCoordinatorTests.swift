import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator")
@MainActor
struct AppCoordinatorTests {
  private func makeSUT() -> (AppCoordinator, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    let coordinator = AppCoordinator(
      repository: spy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    return (coordinator, spy)
  }

  // MARK: - Detail ViewModel Factory

  @Test func makeApplicationDetailViewModel_createsViewModelWithApplication() {
    let (sut, _) = makeSUT()
    let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

    #expect(vm.reference == "2026/0042")
    #expect(vm.address == "12 Mill Road, Cambridge, CB1 2AD")
  }

  @Test func makeApplicationDetailViewModel_dismissClearsDetailApplication() {
    let (sut, _) = makeSUT()
    sut.detailApplication = .pendingReview
    let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

    vm.dismiss()

    #expect(sut.detailApplication == nil)
  }

  // MARK: - Application List Factory

  @Test func makeApplicationListViewModel_createsViewModelWithAuthority() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .success([.pendingReview])
    let vm = sut.makeApplicationListViewModel(authority: .cambridge)

    await vm.loadApplications()

    #expect(spy.fetchApplicationsCalls.first?.code == "CAM")
  }

  @Test func applicationListViewModel_onApplicationSelected_fetchesAndSetsDetail() async throws {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationResult = .success(.approved)
    let vm = sut.makeApplicationListViewModel(authority: .cambridge)

    vm.onApplicationSelected?(PlanningApplicationId("APP-002"))

    try await Task.sleep(for: .milliseconds(50))

    #expect(sut.detailApplication == .approved)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
  }

  @Test func makeApplicationListViewModel_usesAuthorityRepository_whenAvailable() async {
    let appSpy = SpyPlanningApplicationRepository()
    let authoritySpy = SpyApplicationAuthorityRepository()
    authoritySpy.fetchAuthoritiesResult = .success(
      ApplicationAuthorityResult(
        authorities: [.cambridge],
        count: 1
      )
    )
    appSpy.fetchApplicationsByAuthority = ["CAM": [.pendingReview]]
    let coordinator = AppCoordinator(
      repository: appSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      authorityRepository: authoritySpy,
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    let vm = coordinator.makeApplicationListViewModel()

    await vm.loadApplications()

    #expect(authoritySpy.fetchAuthoritiesCallCount == 1)
    #expect(appSpy.fetchApplicationsCalls.map(\.code) == ["CAM"])
    #expect(vm.filteredApplications.count == 1)
  }

  @Test func isOnboardingComplete_falseByDefault() {
    let (sut, _) = makeSUT()

    #expect(!sut.isOnboardingComplete)
  }

  @Test func isOnboardingComplete_trueWhenRepositorySaysComplete() {
    let onboardingRepoSpy = SpyOnboardingRepository()
    onboardingRepoSpy.isOnboardingComplete = true
    let sut = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      onboardingRepository: onboardingRepoSpy,
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )

    #expect(sut.isOnboardingComplete)
  }

  // MARK: - Map ViewModel Factory

  @Test func makeMapViewModel_usesAuthorityRepository() async {
    let appSpy = SpyPlanningApplicationRepository()
    let authoritySpy = SpyApplicationAuthorityRepository()
    authoritySpy.fetchAuthoritiesResult = .success(
      ApplicationAuthorityResult(
        authorities: [.cambridge],
        count: 1
      )
    )
    appSpy.fetchApplicationsByAuthority = ["CAM": [.pendingReview]]
    let coordinator = AppCoordinator(
      repository: appSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      authorityRepository: authoritySpy,
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    let vm = coordinator.makeMapViewModel(watchZone: .cambridge)

    await vm.loadApplications()

    #expect(authoritySpy.fetchAuthoritiesCallCount == 1)
    #expect(appSpy.fetchApplicationsCalls.map(\.code) == ["CAM"])
    #expect(vm.annotations.count == 1)
  }

  // MARK: - Settings ViewModel Factory

  @Test func makeSettingsViewModel_withRequiredDeps_createsViewModel() {
    let (sut, _) = makeSUT()

    let vm = sut.makeSettingsViewModel()

    #expect(!vm.isLoading)
  }

  // MARK: - Force Update ViewModel Factory

  @Test func makeForceUpdateViewModel_withRequiredDeps_createsViewModel() {
    let (sut, _) = makeSUT()

    let vm = sut.makeForceUpdateViewModel()

    #expect(!vm.requiresUpdate)
  }

  // MARK: - Settings Navigation (Legal Documents)

  @Test func showPrivacyPolicy_setsPresentedLegalDocumentToPrivacyPolicy() {
    let (sut, _) = makeSUT()

    sut.showPrivacyPolicy()

    #expect(sut.presentedLegalDocument == .privacyPolicy)
  }

  @Test func showTermsOfService_setsPresentedLegalDocumentToTermsOfService() {
    let (sut, _) = makeSUT()

    sut.showTermsOfService()

    #expect(sut.presentedLegalDocument == .termsOfService)
  }

  @Test func presentedLegalDocument_isNilByDefault() {
    let (sut, _) = makeSUT()

    #expect(sut.presentedLegalDocument == nil)
  }

  // MARK: - Settings Navigation (Manage Subscription)

  @Test func showManageSubscription_setsIsManageSubscriptionPresentedToTrue() {
    let (sut, _) = makeSUT()

    sut.showManageSubscription()

    #expect(sut.isManageSubscriptionPresented)
  }

  @Test func isManageSubscriptionPresented_isFalseByDefault() {
    let (sut, _) = makeSUT()

    #expect(!sut.isManageSubscriptionPresented)
  }
}
