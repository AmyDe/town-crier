import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("Large zone warning")
struct LargeZoneWarningTests {

  // MARK: - Threshold logic

  @Test func warningIsHidden_whenRadiusBelowThreshold_inEditor() {
    let vm = makeEditorViewModel()
    vm.selectedRadiusMetres = 1900

    #expect(vm.showsLargeZoneWarning == false)
  }

  @Test func warningIsShown_atThreshold_inEditor() {
    let vm = makeEditorViewModel()
    vm.selectedRadiusMetres = 2100

    #expect(vm.showsLargeZoneWarning == true)
  }

  @Test func warningIsHidden_atFreeTierMaxRadius_inEditor() {
    let vm = makeEditorViewModel(tier: .free)
    vm.selectedRadiusMetres = 2000

    #expect(vm.showsLargeZoneWarning == false)
  }

  @Test func warningIsShown_aboveThreshold_inEditor() {
    let vm = makeEditorViewModel(tier: .pro)
    vm.selectedRadiusMetres = 5000

    #expect(vm.showsLargeZoneWarning == true)
  }

  @Test func warningIsHidden_atSmallCityRadius_inEditor() {
    let vm = makeEditorViewModel()
    vm.selectedRadiusMetres = 500

    #expect(vm.showsLargeZoneWarning == false)
  }

  @Test func warningIsHidden_whenRadiusBelowThreshold_inOnboarding() {
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 1000

    #expect(vm.showsLargeZoneWarning == false)
  }

  @Test func warningIsShown_atThreshold_inOnboarding() {
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 2100

    #expect(vm.showsLargeZoneWarning == true)
  }

  @Test func warningIsHidden_atFreeTierMaxRadius_inOnboarding() {
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 2000

    #expect(vm.showsLargeZoneWarning == false)
  }

  @Test func warningIsShown_atLargestOnboardingOption_inOnboarding() {
    let vm = makeOnboardingViewModel()
    vm.selectedRadiusMetres = 5000

    #expect(vm.showsLargeZoneWarning == true)
  }

  @Test func warningIsShown_justOver2km_inEditor() {
    let vm = makeEditorViewModel(tier: .pro)
    vm.selectedRadiusMetres = 2001

    #expect(vm.showsLargeZoneWarning == true)
  }

  // MARK: - Custom mode

  @Test func warningIsHidden_inCustomMode_withFewerThanThreeVertices() throws {
    let vm = makeEditorViewModel(tier: .pro)
    vm.selectShapeMode(.custom)
    try vm.addVertex(Coordinate(latitude: 51.95, longitude: -0.05))
    try vm.addVertex(Coordinate(latitude: 52.05, longitude: 0.05))

    #expect(vm.showsLargeZoneWarning == false)
  }

  @Test func warningIsShown_inCustomMode_forLargePolygon() throws {
    let vm = makeEditorViewModel(tier: .pro)
    vm.selectedRadiusMetres = 100
    vm.selectShapeMode(.custom)
    try drawSquare(on: vm, half: 0.05)

    #expect(vm.showsLargeZoneWarning == true)
  }

  @Test func warningIsHidden_inCustomMode_forSmallPolygon_evenWithLargeRadiusSlider() throws {
    let vm = makeEditorViewModel(tier: .pro)
    vm.selectedRadiusMetres = 5000
    vm.selectShapeMode(.custom)
    try drawSquare(on: vm, half: 0.002)

    #expect(vm.showsLargeZoneWarning == false)
  }

  // MARK: - Copy

  @Test func warningCopy_matchesSpecExactly() {
    #expect(LargeZoneCopy.warningTitle == "Large zones are for notifications, not browsing")
    #expect(
      LargeZoneCopy.warningSpeedParagraph
        == "Good if you want to hear about everything in a wide area - tradespeople looking for leads, for example. Your notifications work as normal, but the map and list can take up to a minute to load the first time."
    )
    #expect(
      LargeZoneCopy.warningAdviceParagraph
        == "Just want to keep an eye on your street? Stick to 100-500 m in built-up areas, and under 2 km everywhere else."
    )
    #expect(
      LargeZoneCopy.notice
        == "Large zone: built for notifications. This can take up to a minute to load the first time."
    )
    #expect(
      LargeZoneCopy.loadingMessage
        == "Loading a large zone. This can take up to a minute the first time.")
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
    #expect(vm.showsLargeZoneWarning == false)
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
    #expect(vm.showsLargeZoneWarning == true)
  }

  @Test func warningView_renders() {
    let sut = LargeZoneWarningView()
    _ = sut.body
  }

  @Test func noticeView_renders() {
    let sut = LargeZoneNoticeView()
    _ = sut.body
  }

  // MARK: - Helpers

  private func drawSquare(on vm: WatchZoneEditorViewModel, half: Double) throws {
    for (lat, lon) in [(-half, -half), (-half, half), (half, half), (half, -half)] {
      try vm.addVertex(Coordinate(latitude: 52.0 + lat, longitude: 0.0 + lon))
    }
  }

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
