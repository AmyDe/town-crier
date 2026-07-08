import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AnonymousBrowseCoordinator")
@MainActor
struct AnonymousBrowseCoordinatorTests {
  private func makeSUT(
    persistedState: AnonymousBrowseState? = nil
  ) -> (
    AnonymousBrowseCoordinator, SpyPostcodeGeocoder, SpyAnonymousBrowseStateRepository,
    SpyAnonymousApplicationsRepository
  ) {
    let geocoder = SpyPostcodeGeocoder()
    let stateRepository = SpyAnonymousBrowseStateRepository()
    stateRepository.loadResult = persistedState
    let applicationsRepository = SpyAnonymousApplicationsRepository()
    let sut = AnonymousBrowseCoordinator(
      geocoder: geocoder,
      stateRepository: stateRepository,
      applicationsRepository: applicationsRepository
    )
    return (sut, geocoder, stateRepository, applicationsRepository)
  }

  private var testState: AnonymousBrowseState {
    // swiftlint:disable:next force_try
    AnonymousBrowseState(postcode: try! Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
  }

  // MARK: - Initial screen

  @Test func init_withNoPersistedState_startsAtWelcome() {
    let (sut, _, _, _) = makeSUT()

    #expect(sut.screen == .welcome)
    #expect(sut.mapViewModel == nil)
  }

  @Test func init_withPersistedState_startsAtMap() {
    let (sut, _, _, _) = makeSUT(persistedState: testState)

    #expect(sut.screen == .map)
    #expect(sut.mapViewModel != nil)
    #expect(sut.mapViewModel?.centreLat == Coordinate.cambridge.latitude)
  }

  // MARK: - Welcome -> postcode entry

  @Test func welcomeViewModel_getStarted_advancesToPostcodeEntry() {
    let (sut, _, _, _) = makeSUT()
    let welcomeVM = sut.makeWelcomeViewModel()

    welcomeVM.getStarted()

    #expect(sut.screen == .postcodeEntry)
  }

  @Test func welcomeViewModel_signIn_invokesOnRequestSignIn() {
    let (sut, _, _, _) = makeSUT()
    var requested = false
    sut.onRequestSignIn = { requested = true }
    let welcomeVM = sut.makeWelcomeViewModel()

    welcomeVM.signIn()

    #expect(requested)
  }

  // MARK: - Postcode entry -> map / back

  @Test func postcodeEntryViewModel_onResolved_advancesToMap() {
    let (sut, _, _, _) = makeSUT()
    let postcodeVM = sut.makePostcodeEntryViewModel()

    // The coordinator wires `onResolved` when it builds the view model; invoke
    // it directly to test that wiring deterministically, independent of the
    // view model's own geocode/persist behaviour (covered separately).
    postcodeVM.onResolved?(testState)

    #expect(sut.screen == .map)
    #expect(sut.mapViewModel != nil)
  }

  @Test func postcodeEntryViewModel_onBack_returnsToWelcome() {
    let (sut, _, _, _) = makeSUT()
    let welcomeVM = sut.makeWelcomeViewModel()
    welcomeVM.getStarted()
    let postcodeVM = sut.makePostcodeEntryViewModel()

    postcodeVM.goBack()

    #expect(sut.screen == .welcome)
  }

  // MARK: - Map sign-up handoff

  @Test func mapViewModel_requestSignUp_invokesOnRequestSignIn() {
    let (sut, _, _, _) = makeSUT(persistedState: testState)
    var requested = false
    sut.onRequestSignIn = { requested = true }

    sut.mapViewModel?.requestSignUp()

    #expect(requested)
  }

  // MARK: - Live radius picker persistence (GH#868 Phase 3 refinement)

  @Test func mapViewModel_radiusChange_persistsUpdatedStateWithSamePostcodeAndCoordinate() {
    let (sut, _, stateRepository, _) = makeSUT(persistedState: testState)

    sut.mapViewModel?.updateSelectedRadius(1500)

    #expect(stateRepository.saveCalls.last?.radiusMetres == 1500)
    #expect(stateRepository.saveCalls.last?.postcode == testState.postcode)
    #expect(stateRepository.saveCalls.last?.coordinate == testState.coordinate)
  }

  @Test func postcodeEntryViewModel_onResolved_thenRadiusChange_persistsAgainstResolvedState() {
    let (sut, _, stateRepository, _) = makeSUT()
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

    let (sut, _, _, _) = makeSUT(persistedState: stateWithRadius)

    #expect(sut.mapViewModel?.selectedRadiusMetres == 1500)
  }

  // MARK: - Reset (sign-out)

  @Test func reset_clearsStateAndReturnsToWelcome() {
    let (sut, _, stateRepository, _) = makeSUT(persistedState: testState)
    #expect(sut.screen == .map)

    sut.reset()

    #expect(stateRepository.clearCallCount == 1)
    #expect(sut.screen == .welcome)
    #expect(sut.mapViewModel == nil)
  }
}
