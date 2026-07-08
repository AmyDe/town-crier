import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 3 acceptance criteria: post-postcode routing lands on the
/// tab shell with exactly Applications/Map/Settings, and the CTA banner
/// (tested directly in `AccountCTABannerTests`) is reachable from Applications
/// and Map.
@Suite("AnonymousMainTabView")
@MainActor
struct AnonymousMainTabViewTests {
  private func makeCoordinator() throws -> AnonymousBrowseCoordinator {
    let stateRepository = SpyAnonymousBrowseStateRepository()
    stateRepository.loadResult = AnonymousBrowseState(
      postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date())
    return AnonymousBrowseCoordinator(
      geocoder: SpyPostcodeGeocoder(),
      stateRepository: stateRepository,
      applicationsRepository: SpyAnonymousApplicationsRepository(),
      appVersionProvider: SpyAppVersionProvider()
    )
  }

  @Test func body_renders_onApplicationsTab() throws {
    let coordinator = try makeCoordinator()
    coordinator.selectedTab = .applications
    let sut = AnonymousMainTabView(coordinator: coordinator)

    _ = sut.body
  }

  @Test func body_renders_onMapTab() throws {
    let coordinator = try makeCoordinator()
    coordinator.selectedTab = .map
    let sut = AnonymousMainTabView(coordinator: coordinator)

    _ = sut.body
  }

  @Test func body_renders_onSettingsTab() throws {
    let coordinator = try makeCoordinator()
    coordinator.selectedTab = .settings
    let sut = AnonymousMainTabView(coordinator: coordinator)

    _ = sut.body
  }

  /// `AnonymousBrowseView` routes `.tabs` straight to this view — proven at
  /// the coordinator level (`AnonymousBrowseCoordinatorTests`); this
  /// confirms the view itself renders once the coordinator has resolved a
  /// postcode (map view model + current state both set).
  @Test func body_renders_immediatelyAfterPostcodeResolution() throws {
    let stateRepository = SpyAnonymousBrowseStateRepository()
    let coordinator = AnonymousBrowseCoordinator(
      geocoder: SpyPostcodeGeocoder(),
      stateRepository: stateRepository,
      applicationsRepository: SpyAnonymousApplicationsRepository(),
      appVersionProvider: SpyAppVersionProvider()
    )
    let postcodeVM = coordinator.makePostcodeEntryViewModel()
    postcodeVM.onResolved?(
      AnonymousBrowseState(
        postcode: try Postcode("CB1 2AD"), coordinate: .cambridge, createdAt: Date()))
    #expect(coordinator.screen == .tabs)

    let sut = AnonymousMainTabView(coordinator: coordinator)

    _ = sut.body
  }
}
