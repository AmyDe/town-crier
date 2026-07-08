import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("AnonymousBrowseView")
struct AnonymousBrowseViewTests {
  private func makeCoordinator(
    persistedState: AnonymousBrowseState? = nil
  ) -> AnonymousBrowseCoordinator {
    let stateRepository = SpyAnonymousBrowseStateRepository()
    stateRepository.loadResult = persistedState
    return AnonymousBrowseCoordinator(
      geocoder: SpyPostcodeGeocoder(),
      stateRepository: stateRepository,
      applicationsRepository: SpyAnonymousApplicationsRepository(),
      deviceLocalZoneRepository: SpyDeviceLocalZoneRepository(),
      appVersionProvider: SpyAppVersionProvider()
    )
  }

  @Test func body_renders_atWelcome() {
    let sut = AnonymousBrowseView(coordinator: makeCoordinator())
    _ = sut.body
  }

  @Test func body_renders_atPostcodeEntry() {
    let coordinator = makeCoordinator()
    coordinator.makeWelcomeViewModel().getStarted()
    let sut = AnonymousBrowseView(coordinator: coordinator)
    _ = sut.body
  }

  @Test func body_renders_atTabs() throws {
    let state = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    let sut = AnonymousBrowseView(coordinator: makeCoordinator(persistedState: state))
    _ = sut.body
  }
}
