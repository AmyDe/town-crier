import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 5: the onboarding wizard prefers an active device-local zone
/// (GH#879 Phase 4) over the legacy single ``AnonymousBrowseState`` blob, and
/// clears the converted zone only once `completeOnboarding()` has actually
/// saved it server-side — never merely on wizard construction (abandoning
/// the wizard mid-flow must not silently lose the user's data).
@Suite("AppCoordinator -- Device-Local Zone Prefill (GH#879 Phase 5)")
@MainActor
struct AppCoordinatorDeviceLocalZonePrefillTests {
  private func makeSUT(
    deviceLocalZoneRepository: SpyDeviceLocalZoneRepository? = nil,
    anonymousBrowseStateRepository: SpyAnonymousBrowseStateRepository? = nil
  ) -> AppCoordinator {
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
      versionConfigService: SpyVersionConfigService(),
      anonymousBrowseStateRepository: anonymousBrowseStateRepository,
      deviceLocalZoneRepository: deviceLocalZoneRepository
    )
  }

  private func makeZone(
    id: String = "zone-a", name: String = "Home", radiusMetres: Double = 1500
  ) throws -> DeviceLocalZone {
    try DeviceLocalZone(
      id: DeviceLocalZoneId(id), name: name, centre: .cambridge, radiusMetres: radiusMetres)
  }

  // MARK: - Prefers the active DeviceLocalZone over legacy state

  @Test func makeOnboardingViewModel_withActiveDeviceLocalZone_prefillsFromIt() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeZone(name: "Mum's House", radiusMetres: 1500)
    localRepo.loadAllResult = [zone]
    localRepo.activeZoneIdResult = zone.id
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.postcodeInput == "Mum's House")
    #expect(vm.geocodedCoordinate == .cambridge)
    #expect(vm.currentStep == .radiusPicker)
    #expect(vm.selectedRadiusMetres == 1500)
  }

  @Test func makeOnboardingViewModel_withActiveDeviceLocalZone_ignoresLegacyState() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeZone(name: "Mum's House")
    localRepo.loadAllResult = [zone]
    localRepo.activeZoneIdResult = zone.id
    let legacyRepo = SpyAnonymousBrowseStateRepository()
    legacyRepo.loadResult = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    let sut = makeSUT(deviceLocalZoneRepository: localRepo, anonymousBrowseStateRepository: legacyRepo)

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.postcodeInput == "Mum's House")
    // The legacy blob is left completely untouched when a device-local zone
    // takes priority — never read, never cleared.
    #expect(legacyRepo.loadCallCount == 0)
    #expect(legacyRepo.clearCallCount == 0)
  }

  @Test func makeOnboardingViewModel_withActiveDeviceLocalZone_doesNotDeleteAtConstructionTime() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let zone = try makeZone()
    localRepo.loadAllResult = [zone]
    localRepo.activeZoneIdResult = zone.id
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)

    _ = sut.makeOnboardingViewModel()

    #expect(localRepo.deleteCalls.isEmpty)
  }

  // MARK: - Falls back to legacy state

  @Test func makeOnboardingViewModel_noActiveDeviceLocalZone_fallsBackToLegacyState() throws {
    let localRepo = SpyDeviceLocalZoneRepository()  // no zones, no active id
    let legacyRepo = SpyAnonymousBrowseStateRepository()
    let postcode = try Postcode("CB1 2AD")
    legacyRepo.loadResult = AnonymousBrowseState(
      postcode: postcode, coordinate: .cambridge, radiusMetres: 1200, createdAt: Date())
    let sut = makeSUT(deviceLocalZoneRepository: localRepo, anonymousBrowseStateRepository: legacyRepo)

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.postcodeInput == "CB1 2AD")
    #expect(vm.selectedRadiusMetres == 1200)
    #expect(legacyRepo.clearCallCount == 1)
  }

  @Test func makeOnboardingViewModel_noDeviceLocalZoneRepositoryInjected_fallsBackToLegacyState() throws {
    let legacyRepo = SpyAnonymousBrowseStateRepository()
    legacyRepo.loadResult = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    let sut = makeSUT(anonymousBrowseStateRepository: legacyRepo)

    let vm = sut.makeOnboardingViewModel()

    #expect(vm.postcodeInput == "CB1 2AD")
    #expect(legacyRepo.clearCallCount == 1)
  }

  // MARK: - Deferred clearing on completeOnboarding()

  @Test func completeOnboarding_deletesOnlyTheConvertedActiveZone() throws {
    let localRepo = SpyDeviceLocalZoneRepository()
    let active = try makeZone(id: "zone-a", name: "Home")
    let other = try makeZone(id: "zone-b", name: "Work")
    localRepo.loadAllResult = [active, other]
    localRepo.activeZoneIdResult = active.id
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)
    _ = sut.makeOnboardingViewModel()

    sut.completeOnboarding()

    #expect(localRepo.deleteCalls == [active.id])
  }

  @Test func completeOnboarding_withNoPrefilledDeviceLocalZone_deletesNothing() {
    let localRepo = SpyDeviceLocalZoneRepository()
    let sut = makeSUT(deviceLocalZoneRepository: localRepo)
    _ = sut.makeOnboardingViewModel()

    sut.completeOnboarding()

    #expect(localRepo.deleteCalls.isEmpty)
  }
}
