import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Behaviour for the "large watch zone produces lots of notifications" warning that
/// appears on the radius picker (onboarding) and the watch-zone editor (tc-1zb7).
@MainActor
@Suite("Large radius warning")
struct LargeRadiusWarningTests {

  // MARK: - Threshold logic

  @Test func warningIsHidden_whenRadiusBelowThreshold_inEditor() {
    let vm = makeEditorViewModel()
    vm.selectedRadiusMetres = 1900

    #expect(vm.showsLargeRadiusWarning == false)
  }

  @Test func warningIsShown_atThreshold_inEditor() {
    let vm = makeEditorViewModel()
    vm.selectedRadiusMetres = 2000

    #expect(vm.showsLargeRadiusWarning == true)
  }

  @Test func warningIsShown_aboveThreshold_inEditor() {
    let vm = makeEditorViewModel(tier: .pro)
    vm.selectedRadiusMetres = 5000

    #expect(vm.showsLargeRadiusWarning == true)
  }

  @Test func warningIsHidden_atSmallCityRadius_inEditor() {
    // Bead recommends 100–500m in cities — these must never trigger the warning.
    let vm = makeEditorViewModel()
    vm.selectedRadiusMetres = 500

    #expect(vm.showsLargeRadiusWarning == false)
  }

  @Test func warningIsHidden_whenRadiusBelowThreshold_inOnboarding() {
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 1000

    #expect(vm.showsLargeRadiusWarning == false)
  }

  @Test func warningIsShown_atThreshold_inOnboarding() {
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 2000

    #expect(vm.showsLargeRadiusWarning == true)
  }

  @Test func warningIsShown_atLargestOnboardingOption_inOnboarding() {
    // The onboarding picker offers up to 5 km; that selection must trigger the warning.
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 5000

    #expect(vm.showsLargeRadiusWarning == true)
  }

  // MARK: - View structure (smoke renders)

  @Test func editorView_renders_atSmallRadius_noWarning() throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-small"),
      name: "Home",
      centre: .cambridge,
      radiusMetres: 500
    )
    let vm = makeEditorViewModel(tier: .personal, editing: zone)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
    #expect(vm.showsLargeRadiusWarning == false)
  }

  @Test func editorView_renders_atLargeRadius_warningShown() throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-large"),
      name: "City",
      centre: .cambridge,
      radiusMetres: 3000
    )
    let vm = makeEditorViewModel(tier: .personal, editing: zone)
    let sut = WatchZoneEditorView(viewModel: vm)
    _ = sut.body
    #expect(vm.showsLargeRadiusWarning == true)
  }

  @Test func warningView_renders() {
    let sut = LargeRadiusWarningView()
    _ = sut.body
  }

  // MARK: - Helpers

  private func makeEditorViewModel(
    tier: SubscriptionTier = .personal,
    editing zone: WatchZone? = nil
  ) -> WatchZoneEditorViewModel {
    WatchZoneEditorViewModel(
      geocoder: SpyPostcodeGeocoder(),
      repository: SpyWatchZoneRepository(),
      tier: tier,
      editing: zone
    )
  }

  private func makeOnboardingViewModel() -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService()
    )
  }
}
