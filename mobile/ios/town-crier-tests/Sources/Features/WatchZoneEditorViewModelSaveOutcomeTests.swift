import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

/// Save outcome + quota routing (tc-gpjk). `save()` reports success so the View
/// can dismiss only on success. When the repository reports the watch-zone
/// quota is exceeded (`.insufficientEntitlement`), the editor routes to the
/// upgrade paywall via `onUpgradeRequired` and leaves `error` unset (the inline
/// error section is for other, retryable-ish failures only).
@MainActor
@Suite("WatchZoneEditorViewModel — save outcome")
struct WatchZoneEditorSaveOutcomeTests {
  private var spyGeocoder: SpyPostcodeGeocoder!
  private var spyRepository: SpyWatchZoneRepository!
  private var sut: WatchZoneEditorViewModel!

  init() {
    spyGeocoder = SpyPostcodeGeocoder()
    spyRepository = SpyWatchZoneRepository()
    sut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .free
    )
  }

  private func geocode() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
  }

  @Test func save_onSuccess_returnsTrueAndFiresOnSave() async {
    await geocode()
    var savedZone: WatchZone?
    sut.onSave = { savedZone = $0 }

    let didSave = await sut.save()

    #expect(didSave)
    #expect(savedZone != nil)
    #expect(sut.error == nil)
  }

  @Test func save_quotaExceeded_routesToUpgrade_withoutSettingError() async {
    await geocode()
    spyRepository.saveResult = .failure(
      DomainError.insufficientEntitlement(required: "personal"))
    var upgradeRequested = false
    sut.onUpgradeRequired = { upgradeRequested = true }

    let didSave = await sut.save()

    #expect(!didSave)
    #expect(upgradeRequested)
    #expect(sut.error == nil)
  }

  @Test func save_otherFailure_setsError_andDoesNotRouteToUpgrade() async {
    await geocode()
    spyRepository.saveResult = .failure(DomainError.networkUnavailable)
    var upgradeRequested = false
    sut.onUpgradeRequired = { upgradeRequested = true }

    let didSave = await sut.save()

    #expect(!didSave)
    #expect(!upgradeRequested)
    #expect(sut.error == .networkUnavailable)
  }

  @Test func save_withoutGeocoding_returnsFalse() async {
    let didSave = await sut.save()

    #expect(!didSave)
    #expect(spyRepository.saveCalls.isEmpty)
  }
}
