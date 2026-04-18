import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AppCoordinator -- Watch Zone Factories")
@MainActor
struct AppCoordinatorWatchZoneTests {
  private func makeSUT() -> AppCoordinator {
    AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: SpyWatchZoneRepository(),
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
  }

  // MARK: - Watch Zone List Factory

  @Test func makeWatchZoneListViewModel_createsViewModel() {
    let sut = makeSUT()

    let vm = sut.makeWatchZoneListViewModel()

    #expect(vm.zones.isEmpty)
    #expect(!vm.isLoading)
  }

  @Test func makeWatchZoneListViewModel_onAddZone_setsIsAddingWatchZone() {
    let sut = makeSUT()
    let vm = sut.makeWatchZoneListViewModel()

    vm.addZone()

    #expect(sut.isAddingWatchZone)
  }

  @Test func makeWatchZoneListViewModel_onEditZone_setsEditingWatchZone() {
    let sut = makeSUT()
    let vm = sut.makeWatchZoneListViewModel()

    vm.editZone(.cambridge)

    #expect(sut.editingWatchZone == .cambridge)
  }

  @Test func isAddingWatchZone_isFalseByDefault() {
    let sut = makeSUT()

    #expect(!sut.isAddingWatchZone)
  }

  @Test func editingWatchZone_isNilByDefault() {
    let sut = makeSUT()

    #expect(sut.editingWatchZone == nil)
  }

  // MARK: - Watch Zone Editor Factory

  @Test func makeWatchZoneEditorViewModel_forAdd_createsNonEditingViewModel() {
    let sut = makeSUT()

    let vm = sut.makeWatchZoneEditorViewModel()

    #expect(!vm.isEditing)
  }

  @Test func makeWatchZoneEditorViewModel_forEdit_createsEditingViewModel() {
    let sut = makeSUT()

    let vm = sut.makeWatchZoneEditorViewModel(editing: .cambridge)

    #expect(vm.isEditing)
    #expect(vm.nameInput == "CB1 2AD")
    #expect(vm.postcodeInput.isEmpty)
  }

  @Test func makeWatchZoneEditorViewModel_onSave_dismissesEditor() {
    let sut = makeSUT()
    sut.isAddingWatchZone = true
    let vm = sut.makeWatchZoneEditorViewModel()

    vm.onSave?(.cambridge)

    #expect(!sut.isAddingWatchZone)
    #expect(sut.editingWatchZone == nil)
  }

  @Test func makeWatchZoneEditorViewModel_forEdit_onSave_dismissesEditor() {
    let sut = makeSUT()
    sut.editingWatchZone = .cambridge
    let vm = sut.makeWatchZoneEditorViewModel(editing: .cambridge)

    vm.onSave?(.cambridge)

    #expect(sut.editingWatchZone == nil)
    #expect(!sut.isAddingWatchZone)
  }

  @Test func makeWatchZoneEditorViewModel_onSave_reloadsZones() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let sut = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: watchZoneSpy,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    let listVM = sut.makeWatchZoneListViewModel()
    _ = listVM
    sut.isAddingWatchZone = true
    let vm = sut.makeWatchZoneEditorViewModel()

    vm.onSave?(.cambridge)

    // Give the async loadAll task a moment to run
    try await Task.sleep(for: .milliseconds(200))

    #expect(watchZoneSpy.loadAllCallCount >= 1)
  }

  @Test func makeWatchZoneEditorViewModel_forEdit_onSave_reloadsZones() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let sut = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: watchZoneSpy,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )
    let listVM = sut.makeWatchZoneListViewModel()
    _ = listVM
    sut.editingWatchZone = .cambridge
    let vm = sut.makeWatchZoneEditorViewModel(editing: .cambridge)

    vm.onSave?(.cambridge)

    try await Task.sleep(for: .milliseconds(200))

    #expect(watchZoneSpy.loadAllCallCount >= 1)
    #expect(sut.editingWatchZone == nil)
  }

  @Test func makeWatchZoneEditorViewModel_onSave_refreshesListViewModelZones() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let sut = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: watchZoneSpy,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )

    let listVM = sut.makeWatchZoneListViewModel()
    await listVM.load()
    #expect(listVM.zones == [.cambridge])

    let renamed = try WatchZone(
      id: WatchZone.cambridge.id,
      name: "My New Name",
      centre: WatchZone.cambridge.centre,
      radiusMetres: WatchZone.cambridge.radiusMetres,
      authorityId: WatchZone.cambridge.authorityId
    )
    watchZoneSpy.loadAllResult = .success([renamed])

    sut.editingWatchZone = .cambridge
    let editorVM = sut.makeWatchZoneEditorViewModel(editing: .cambridge)
    editorVM.onSave?(renamed)

    try await Task.sleep(for: .milliseconds(200))

    #expect(listVM.zones == [renamed])
  }

  /// Reproduces the real-world bug where SwiftUI re-evaluates the view
  /// hierarchy (e.g. when the coordinator publishes a sheet state change),
  /// calling ``makeWatchZoneListViewModel()`` again before the editor's
  /// `onSave` fires. The original list VM must still be refreshed rather
  /// than a short-lived, unretained replacement.
  @Test func editorOnSave_refreshesListViewModel_afterReRender() async throws {
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let sut = AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: watchZoneSpy,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService()
    )

    // First render: the View retains this VM via @StateObject.
    let listVM = sut.makeWatchZoneListViewModel()
    await listVM.load()
    #expect(listVM.zones == [.cambridge])

    // Simulate SwiftUI re-rendering the view hierarchy during sheet
    // presentation. Each call constructs a fresh VM that is *not* retained
    // by any caller (matching what happens when @StateObject ignores the
    // parameter on re-renders).
    _ = sut.makeWatchZoneListViewModel()
    _ = sut.makeWatchZoneListViewModel()

    let renamed = try WatchZone(
      id: WatchZone.cambridge.id,
      name: "My New Name",
      centre: WatchZone.cambridge.centre,
      radiusMetres: WatchZone.cambridge.radiusMetres,
      authorityId: WatchZone.cambridge.authorityId
    )
    watchZoneSpy.loadAllResult = .success([renamed])

    sut.editingWatchZone = .cambridge
    let editorVM = sut.makeWatchZoneEditorViewModel(editing: .cambridge)
    editorVM.onSave?(renamed)

    try await Task.sleep(for: .milliseconds(200))

    #expect(listVM.zones == [renamed])
  }

  /// ``makeWatchZoneListViewModel()`` must return a stable instance so that
  /// SwiftUI's `@StateObject` binding and the coordinator's refresh path
  /// both converge on the same VM.
  @Test func makeWatchZoneListViewModel_returnsStableInstanceAcrossCalls() {
    let sut = makeSUT()

    let first = sut.makeWatchZoneListViewModel()
    let second = sut.makeWatchZoneListViewModel()

    #expect(first === second)
  }

  // MARK: - Watch Zone Upsell

  @Test func makeWatchZoneListViewModel_onViewPlans_setsIsSubscriptionPresented() {
    let sut = makeSUT()
    let vm = sut.makeWatchZoneListViewModel()

    vm.viewPlans()

    #expect(sut.isSubscriptionPresented)
  }

  @Test func isSubscriptionPresented_isFalseByDefault() {
    let sut = makeSUT()

    #expect(!sut.isSubscriptionPresented)
  }
}
