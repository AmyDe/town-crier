import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AnonymousBrowseCoordinator")
@MainActor
struct AnonymousBrowseCoordinatorTests {
  private func makeSUT(
    persistedState: AnonymousBrowseState? = nil,
    appearanceStore: AppearanceStore? = nil
  ) -> (
    AnonymousBrowseCoordinator, SpyPostcodeGeocoder, SpyAnonymousBrowseStateRepository,
    SpyAnonymousApplicationsRepository, SpyDeviceLocalZoneRepository
  ) {
    let geocoder = SpyPostcodeGeocoder()
    let stateRepository = SpyAnonymousBrowseStateRepository()
    stateRepository.loadResult = persistedState
    let applicationsRepository = SpyAnonymousApplicationsRepository()
    let deviceLocalZoneRepository = SpyDeviceLocalZoneRepository()
    let sut = AnonymousBrowseCoordinator(
      geocoder: geocoder,
      stateRepository: stateRepository,
      applicationsRepository: applicationsRepository,
      deviceLocalZoneRepository: deviceLocalZoneRepository,
      appearanceStore: appearanceStore,
      appVersionProvider: SpyAppVersionProvider()
    )
    return (sut, geocoder, stateRepository, applicationsRepository, deviceLocalZoneRepository)
  }

  private var testState: AnonymousBrowseState {
    // swiftlint:disable:next force_try
    AnonymousBrowseState(postcode: try! Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
  }

  // MARK: - Initial screen

  @Test func init_withNoPersistedState_startsAtWelcome() {
    let (sut, _, _, _, _) = makeSUT()

    #expect(sut.screen == .welcome)
    #expect(sut.mapViewModel == nil)
  }

  @Test func init_withPersistedState_startsAtTabs() {
    let (sut, _, _, _, _) = makeSUT(persistedState: testState)

    #expect(sut.screen == .tabs)
    #expect(sut.mapViewModel != nil)
    #expect(sut.mapViewModel?.centreLat == Coordinate.cambridge.latitude)
  }

  // MARK: - Welcome -> postcode entry

  @Test func welcomeViewModel_getStarted_advancesToPostcodeEntry() {
    let (sut, _, _, _, _) = makeSUT()
    let welcomeVM = sut.makeWelcomeViewModel()

    welcomeVM.getStarted()

    #expect(sut.screen == .postcodeEntry)
  }

  @Test func welcomeViewModel_signIn_invokesOnRequestSignIn() {
    let (sut, _, _, _, _) = makeSUT()
    var requested = false
    sut.onRequestSignIn = { requested = true }
    let welcomeVM = sut.makeWelcomeViewModel()

    welcomeVM.signIn()

    #expect(requested)
  }

  // MARK: - Postcode entry -> tab shell / back

  @Test func postcodeEntryViewModel_onResolved_advancesToTabs() {
    let (sut, _, _, _, _) = makeSUT()
    let postcodeVM = sut.makePostcodeEntryViewModel()

    // The coordinator wires `onResolved` when it builds the view model; invoke
    // it directly to test that wiring deterministically, independent of the
    // view model's own geocode/persist behaviour (covered separately).
    postcodeVM.onResolved?(testState)

    #expect(sut.screen == .tabs)
    #expect(sut.mapViewModel != nil)
  }

  @Test func postcodeEntryViewModel_onBack_returnsToWelcome() {
    let (sut, _, _, _, _) = makeSUT()
    let welcomeVM = sut.makeWelcomeViewModel()
    welcomeVM.getStarted()
    let postcodeVM = sut.makePostcodeEntryViewModel()

    postcodeVM.goBack()

    #expect(sut.screen == .welcome)
  }

  // MARK: - Map sign-up handoff

  @Test func mapViewModel_requestSignUp_invokesOnRequestSignIn() {
    let (sut, _, _, _, _) = makeSUT(persistedState: testState)
    var requested = false
    sut.onRequestSignIn = { requested = true }

    sut.mapViewModel?.requestSignUp()

    #expect(requested)
  }

  // MARK: - View full details handoff (GH#879 Phase 2)

  @Test func mapViewModel_onShowApplicationDetail_invokesCoordinatorCallback() {
    let (sut, _, _, _, _) = makeSUT(persistedState: testState)
    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.mapViewModel?.selectApplication(.pendingReview)
    sut.mapViewModel?.requestFullDetail()
    sut.mapViewModel?.presentPendingDetailIfNeeded()

    #expect(captured == [.pendingReview])
  }

  // MARK: - Live radius picker persistence (GH#868 Phase 3 refinement)

  @Test func mapViewModel_radiusChange_persistsUpdatedStateWithSamePostcodeAndCoordinate() {
    let (sut, _, stateRepository, _, _) = makeSUT(persistedState: testState)

    sut.mapViewModel?.updateSelectedRadius(1500)

    #expect(stateRepository.saveCalls.last?.radiusMetres == 1500)
    #expect(stateRepository.saveCalls.last?.postcode == testState.postcode)
    #expect(stateRepository.saveCalls.last?.coordinate == testState.coordinate)
  }

  @Test func postcodeEntryViewModel_onResolved_thenRadiusChange_persistsAgainstResolvedState() {
    let (sut, _, stateRepository, _, _) = makeSUT()
    let postcodeVM = sut.makePostcodeEntryViewModel()
    postcodeVM.onResolved?(testState)

    sut.mapViewModel?.updateSelectedRadius(500)

    #expect(stateRepository.saveCalls.last?.radiusMetres == 500)
    #expect(stateRepository.saveCalls.last?.postcode == testState.postcode)
  }

  @Test func init_withPersistedState_seedsMapViewModelSelectedRadiusFromPersistedRadius() {
    let stateWithRadius = AnonymousBrowseState(
      postcode: testState.postcode,
      coordinate: testState.coordinate,
      radiusMetres: 1500,
      createdAt: testState.createdAt
    )

    let (sut, _, _, _, _) = makeSUT(persistedState: stateWithRadius)

    #expect(sut.mapViewModel?.selectedRadiusMetres == 1500)
  }

  // MARK: - Appearance (GH#878)

  @Test func makeWelcomeViewModel_usesTheInjectedAppearanceStore() {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let appearanceStore = AppearanceStore(defaults: defaults!)
    appearanceStore.appearanceMode = .oledDark
    let (sut, _, _, _, _) = makeSUT(appearanceStore: appearanceStore)

    let welcomeVM = sut.makeWelcomeViewModel()

    #expect(welcomeVM.appearanceMode == .oledDark)
  }

  @Test func welcomeViewModel_selectAppearanceMode_updatesTheSameInjectedStore() {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let appearanceStore = AppearanceStore(defaults: defaults!)
    let (sut, _, _, _, _) = makeSUT(appearanceStore: appearanceStore)
    let welcomeVM = sut.makeWelcomeViewModel()

    welcomeVM.selectAppearanceMode(.dark)

    #expect(appearanceStore.appearanceMode == .dark)
  }

  // MARK: - Reset (sign-out)

  @Test func reset_clearsStateAndReturnsToWelcome() {
    let (sut, _, stateRepository, _, _) = makeSUT(persistedState: testState)
    #expect(sut.screen == .tabs)

    sut.reset()

    #expect(stateRepository.clearCallCount == 1)
    #expect(sut.screen == .welcome)
    #expect(sut.mapViewModel == nil)
  }

  // MARK: - Tab shell (GH#879 Phase 3)

  @Test func selectedTab_defaultsToApplications() {
    let (sut, _, _, _, _) = makeSUT(persistedState: testState)

    #expect(sut.selectedTab == .applications)
  }

  /// The tab set is exactly Applications/Map/Zones/Settings, in that order —
  /// no Saved tab, deliberately (saving is account-bound).
  @Test func tab_allCases_isExactlyApplicationsMapZonesSettings() {
    #expect(
      AnonymousBrowseCoordinator.Tab.allCases == [.applications, .map, .zones, .settings])
  }

  @Test func makeApplicationListViewModel_afterPostcodeResolved_seedsFromCurrentState() {
    let (sut, _, _, _, _) = makeSUT(persistedState: testState)

    let viewModel = sut.makeApplicationListViewModel()

    #expect(viewModel != nil)
  }

  @Test func makeApplicationListViewModel_beforePostcodeResolved_returnsNil() {
    let (sut, _, _, _, _) = makeSUT()

    #expect(sut.makeApplicationListViewModel() == nil)
  }

  @Test func applicationListViewModel_onShowApplicationDetail_invokesCoordinatorCallback() {
    let (sut, _, _, applicationsRepository, _) = makeSUT(persistedState: testState)
    applicationsRepository.fetchNearbyResult = .success([.pendingReview])
    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }
    let listViewModel = sut.makeApplicationListViewModel()

    listViewModel?.selectApplication(.pendingReview)

    #expect(captured == [.pendingReview])
  }

  @Test func makeSettingsViewModel_usesTheInjectedAppearanceStore() {
    let defaults = UserDefaults(suiteName: UUID().uuidString)
    // swiftlint:disable:next force_unwrapping
    let appearanceStore = AppearanceStore(defaults: defaults!)
    appearanceStore.appearanceMode = .oledDark
    let (sut, _, _, _, _) = makeSUT(appearanceStore: appearanceStore)

    let settingsVM = sut.makeSettingsViewModel()

    #expect(settingsVM.appearanceMode == .oledDark)
  }

  @Test func requestSignIn_invokesOnRequestSignIn() {
    let (sut, _, _, _, _) = makeSUT()
    var requested = false
    sut.onRequestSignIn = { requested = true }

    sut.requestSignIn()

    #expect(requested)
  }

  @Test func showPrivacyPolicy_invokesOnShowPrivacyPolicy() {
    let (sut, _, _, _, _) = makeSUT()
    var invoked = false
    sut.onShowPrivacyPolicy = { invoked = true }

    sut.showPrivacyPolicy()

    #expect(invoked)
  }

  @Test func showTermsOfService_invokesOnShowTermsOfService() {
    let (sut, _, _, _, _) = makeSUT()
    var invoked = false
    sut.onShowTermsOfService = { invoked = true }

    sut.showTermsOfService()

    #expect(invoked)
  }

  @Test func requestRateApp_invokesOnRateApp() {
    let (sut, _, _, _, _) = makeSUT()
    var invoked = false
    sut.onRateApp = { invoked = true }

    sut.requestRateApp()

    #expect(invoked)
  }

  // MARK: - Zones tab (GH#879 Phase 4)

  @Test func makeDeviceLocalZoneListViewModel_requestSignUp_invokesOnRequestSignIn() {
    let (sut, _, _, _, _) = makeSUT()
    var requested = false
    sut.onRequestSignIn = { requested = true }
    let zonesVM = sut.makeDeviceLocalZoneListViewModel()

    zonesVM.requestAlertsSignUp()
    zonesVM.confirmSignUp()

    #expect(requested)
  }

  @Test func applicationListViewModel_selectZone_reCentresMapViewModel() async throws {
    let (sut, _, _, applicationsRepository, deviceLocalZoneRepository) = makeSUT(
      persistedState: testState)
    applicationsRepository.fetchNearbyResult = .success([])
    let zoneA = try DeviceLocalZone(name: "A", centre: .cambridge, radiusMetres: 1000)
    let zoneB = try DeviceLocalZone(
      name: "B",
      centre: try Coordinate(latitude: 51.5074, longitude: -0.1278),
      radiusMetres: 3000)
    deviceLocalZoneRepository.loadAllResult = [zoneA, zoneB]
    deviceLocalZoneRepository.activeZoneIdResult = zoneA.id
    let listViewModel = sut.makeApplicationListViewModel()
    await listViewModel?.loadApplications()
    let originalMapViewModel = sut.mapViewModel

    await listViewModel?.selectZone(zoneB)

    #expect(sut.mapViewModel !== originalMapViewModel)
    #expect(sut.mapViewModel?.centreLat == zoneB.centre.latitude)
    #expect(sut.mapViewModel?.radiusMetres == zoneB.radiusMetres)
  }
}
