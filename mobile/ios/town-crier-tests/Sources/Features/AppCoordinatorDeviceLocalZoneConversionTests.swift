import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 5: post-wizard "Add your other areas" sheet presentation,
/// quota-breach paywall routing, and reopening from the authed Zones tab.
@Suite("AppCoordinator -- Device-Local Zone Conversion (GH#879 Phase 5)")
@MainActor
struct AppCoordinatorDeviceLocalZoneConversionTests {
  private func makeSUT(
    deviceLocalZoneRepository: SpyDeviceLocalZoneRepository? = nil,
    watchZoneRepository: WatchZoneRepository = SpyWatchZoneRepository()
  ) -> AppCoordinator {
    AppCoordinator(
      repository: SpyPlanningApplicationRepository(),
      authService: SpyAuthenticationService(),
      subscriptionService: SpySubscriptionService(),
      userProfileRepository: SpyUserProfileRepository(),
      watchZoneRepository: watchZoneRepository,
      geocoder: SpyPostcodeGeocoder(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      appVersionProvider: SpyAppVersionProvider(),
      versionConfigService: SpyVersionConfigService(),
      deviceLocalZoneRepository: deviceLocalZoneRepository
    )
  }

  private func makeZone(id: String, name: String) throws -> DeviceLocalZone {
    try DeviceLocalZone(id: DeviceLocalZoneId(id), name: name, centre: .cambridge, radiusMetres: 1000)
  }

  // MARK: - Post-wizard sheet presentation

  @Test func completeOnboarding_withUnconvertedZonesRemaining_presentsConversionSheet() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let active = try makeZone(id: "a", name: "Home")
    let other = try makeZone(id: "b", name: "Work")
    localRepo.loadAllResult = [active, other]
    localRepo.activeZoneIdResult = active.id
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)
    _ = sut.makeOnboardingViewModel()

    sut.completeOnboarding()

    #expect(sut.isDeviceLocalZoneConversionPresented)
  }

  @Test func completeOnboarding_withNoZonesRemaining_doesNotPresentConversionSheet() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let active = try makeZone(id: "a", name: "Home")
    localRepo.loadAllResult = [active]
    localRepo.activeZoneIdResult = active.id
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)
    _ = sut.makeOnboardingViewModel()
    // Simulate the wizard's own conversion actually removing the zone before
    // the repository is next consulted.
    localRepo.loadAllResult = []

    sut.completeOnboarding()

    #expect(!sut.isDeviceLocalZoneConversionPresented)
  }

  @Test func completeOnboarding_noDeviceLocalZoneRepository_doesNotPresentConversionSheet() {
    let sut = makeSUT()

    sut.completeOnboarding()

    #expect(!sut.isDeviceLocalZoneConversionPresented)
  }

  @Test func isDeviceLocalZoneConversionPresented_isFalseByDefault() {
    let sut = makeSUT()

    #expect(!sut.isDeviceLocalZoneConversionPresented)
  }

  // MARK: - makeDeviceLocalZoneConversionViewModel

  @Test func makeDeviceLocalZoneConversionViewModel_noRepository_returnsNil() {
    let sut = makeSUT()

    #expect(sut.makeDeviceLocalZoneConversionViewModel() == nil)
  }

  @Test func makeDeviceLocalZoneConversionViewModel_buildsFromCurrentLocalZones() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeZone(id: "b", name: "Work")
    localRepo.loadAllResult = [zone]
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)

    let vm = sut.makeDeviceLocalZoneConversionViewModel()

    #expect(vm?.zones == [zone])
  }

  // MARK: - Quota-breach routes to the paywall

  @Test func conversionViewModel_onInsufficientEntitlement_dismissesSheetAndPresentsPaywall() async throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeZone(id: "b", name: "Work")
    localRepo.loadAllResult = [zone]
    let watchZoneRepo = SpyWatchZoneRepository()
    watchZoneRepo.saveResult = .failure(DomainError.insufficientEntitlement(required: "personal"))
    let sut = makeSUT(deviceLocalZoneRepository: localRepo, watchZoneRepository: watchZoneRepo)
    sut.isDeviceLocalZoneConversionPresented = true
    let vm = try #require(sut.makeDeviceLocalZoneConversionViewModel())

    await vm.convertAll()

    #expect(!sut.isDeviceLocalZoneConversionPresented)
    #expect(sut.isSubscriptionPresented)
  }

  // MARK: - Reopening from the Zones tab row

  @Test func reopenDeviceLocalZoneConversion_presentsSheet() {
    let sut = makeSUT()

    sut.reopenDeviceLocalZoneConversion()

    #expect(sut.isDeviceLocalZoneConversionPresented)
  }

  // MARK: - Zones-tab wiring

  @Test func makeWatchZoneListViewModel_populatesUnconvertedLocalZones() async throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeZone(id: "b", name: "Work")
    localRepo.loadAllResult = [zone]
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)

    let vm = sut.makeWatchZoneListViewModel()
    await vm.load()

    #expect(vm.unconvertedLocalZones == [zone])
  }

  @Test func makeWatchZoneListViewModel_onConvertLocalZones_reopensConversionSheet() {
    let sut = makeSUT()
    let vm = sut.makeWatchZoneListViewModel()

    vm.convertLocalZones()

    #expect(sut.isDeviceLocalZoneConversionPresented)
  }
}
