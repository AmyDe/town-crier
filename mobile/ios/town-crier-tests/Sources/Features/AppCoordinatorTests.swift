import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator")
@MainActor
struct AppCoordinatorTests {
  private func makeSUT(
    savedApplicationRepository: SavedApplicationRepository? = nil
  ) -> (AppCoordinator, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    let coordinator = AppCoordinator(
      repository: spy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      savedApplicationRepository: savedApplicationRepository
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

  @Test func makeApplicationDetailViewModel_passesRepository_enablesCanSave() {
    let savedSpy = SpySavedApplicationRepository()
    let (sut, _) = makeSUT(savedApplicationRepository: savedSpy)
    let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

    #expect(vm.canSave)
  }

  @Test func makeApplicationDetailViewModel_withoutRepository_canSaveIsFalse() {
    let (sut, _) = makeSUT()
    let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

    #expect(!vm.canSave)
  }

  @Test func makeApplicationDetailViewModel_dismissClearsDetailApplication() {
    let (sut, _) = makeSUT()
    sut.detailApplication = .pendingReview
    let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

    vm.dismiss()

    #expect(sut.detailApplication == nil)
  }

  @Test func makeApplicationDetailViewModel_loadSavedState_reflectsServerState() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication(
        applicationUid: PlanningApplication.pendingReview.id.value,
        savedAt: Date()
      ),
    ])
    let (sut, _) = makeSUT(savedApplicationRepository: savedSpy)
    let vm = sut.makeApplicationDetailViewModel(application: .pendingReview)

    #expect(!vm.isSaved)

    await vm.loadSavedState()

    #expect(vm.isSaved)
  }

  // MARK: - Application List Factory

  @Test func makeApplicationListViewModel_createsViewModelWithZone() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .success([.pendingReview])
    let vm = sut.makeApplicationListViewModel(zone: .cambridge)

    await vm.loadApplications()

    #expect(spy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
  }

  @Test func applicationListViewModel_onApplicationSelected_fetchesAndSetsDetail() async throws {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationResult = .success(.permitted)
    let vm = sut.makeApplicationListViewModel(zone: .cambridge)

    vm.onApplicationSelected?(PlanningApplicationId("APP-002"))

    try await Task.sleep(for: .milliseconds(200))

    #expect(sut.detailApplication == .permitted)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
  }

  @Test func makeApplicationListViewModel_noArg_resolvesZoneFromRepository() async {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let coordinator = AppCoordinator(
      repository: appSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: zoneSpy,
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    let vm = coordinator.makeApplicationListViewModel()

    await vm.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 1)
    #expect(appSpy.fetchApplicationsCalls.first?.id == WatchZone.cambridge.id)
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
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: onboardingRepoSpy,
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )

    #expect(sut.isOnboardingComplete)
  }

  // MARK: - Map ViewModel Factory

  @Test func makeMapViewModel_fetchesByZone() async {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsByZone = ["zone-001": [.pendingReview]]
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let coordinator = AppCoordinator(
      repository: appSpy,
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: watchZoneSpy,
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    let vm = coordinator.makeMapViewModel()

    await vm.loadApplications()

    #expect(appSpy.fetchApplicationsCalls.count == 1)
    #expect(appSpy.fetchApplicationsCalls.first?.id.value == "zone-001")
    #expect(vm.annotations.count == 1)
  }

  // MARK: - Saved Application List Factory

  @Test func makeSavedApplicationListViewModel_returnsConfiguredViewModel() async {
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success([
      SavedApplication.fixture(uid: "APP-A"),
    ])
    let (sut, _) = makeSUT(savedApplicationRepository: savedSpy)

    let vm = sut.makeSavedApplicationListViewModel()

    #expect(vm.selectedStatusFilter == nil)
    await vm.loadAll()
    #expect(vm.applications.count == 1)
  }

  @Test func savedApplicationListViewModel_onApplicationSelected_fetchesAndSetsDetail() async throws {
    let savedSpy = SpySavedApplicationRepository()
    let (sut, spy) = makeSUT(savedApplicationRepository: savedSpy)
    spy.fetchApplicationResult = .success(.permitted)
    let vm = sut.makeSavedApplicationListViewModel()

    vm.onApplicationSelected?(PlanningApplicationId("APP-002"))

    try await Task.sleep(for: .milliseconds(200))

    #expect(sut.detailApplication == .permitted)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
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

  // MARK: - Settings Sheet Presentation

  @Test func isSettingsPresented_isFalseByDefault() {
    let (sut, _) = makeSUT()

    #expect(!sut.isSettingsPresented)
  }

  @Test func showSettings_setsIsSettingsPresentedToTrue() {
    let (sut, _) = makeSUT()

    sut.showSettings()

    #expect(sut.isSettingsPresented)
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

  // MARK: - Settings Navigation (System Notification Settings)

  @Test func isOpeningSystemNotificationSettings_isFalseByDefault() {
    let (sut, _) = makeSUT()

    #expect(!sut.isOpeningSystemNotificationSettings)
  }

  @Test func showSystemNotificationSettings_setsFlagToTrue() {
    let (sut, _) = makeSUT()

    sut.showSystemNotificationSettings()

    #expect(sut.isOpeningSystemNotificationSettings)
  }

  // MARK: - Deterministic detail-load synchronisation

  /// Regression guard for tc-nsrh (CI flakes on `Task.sleep(...)` waits in
  /// the `showApplicationDetail` path). The Coordinator must expose a way to
  /// await the in-flight detail fetch without sleeping so deep-link and
  /// list-selection tests are deterministic.
  @Test func waitForPendingDetailLoad_awaitsShowApplicationDetail() async throws {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationResult = .success(.permitted)

    sut.handleDeepLink(.applicationDetail(PlanningApplicationId("APP-002")))

    await sut.waitForPendingDetailLoad()

    #expect(sut.detailApplication == .permitted)
    #expect(spy.fetchApplicationCalls == [PlanningApplicationId("APP-002")])
  }

}
