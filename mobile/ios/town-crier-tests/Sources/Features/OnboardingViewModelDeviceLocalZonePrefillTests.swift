import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 5: seeding the wizard from an active ``DeviceLocalZone``
/// (name/centre/radius) rather than the legacy single ``AnonymousBrowseState``
/// blob. Split from `OnboardingViewModelTests` to stay under the project's
/// file-length limit.
@Suite("OnboardingViewModel -- Device-Local Zone Prefill")
@MainActor
struct OnboardingViewModelDeviceLocalZonePrefillTests {
  private func makeSUT() -> (OnboardingViewModel, SpyWatchZoneRepository) {
    let zoneRepo = SpyWatchZoneRepository()
    let vm = OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: zoneRepo,
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService()
    )
    return (vm, zoneRepo)
  }

  @Test func prefillByName_seedsNameAndCoordinate_andJumpsToRadiusPicker() throws {
    let (sut, _) = makeSUT()
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(name: "Mum's House", coordinate: coordinate, radiusMetres: 1500)

    #expect(sut.postcodeInput == "Mum's House")
    #expect(sut.geocodedCoordinate == coordinate)
    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.selectedRadiusMetres == 1500)
  }

  @Test func prefillByName_thenConfirmRadius_createsZoneWithThatName() async throws {
    let (sut, _) = makeSUT()
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    sut.prefill(name: "Mum's House", coordinate: coordinate, radiusMetres: 1500)

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmRadius()
    await sut.skipNotifications()

    #expect(completedZone != nil)
    #expect(completedZone?.name == "Mum's House")
    #expect(completedZone?.centre == coordinate)
    #expect(completedZone?.radiusMetres == 1500)
  }

  @Test func prefillByName_withRadiusAboveTierMax_clampsToMaxRadiusMetres() throws {
    let (sut, _) = makeSUT()
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(name: "Mum's House", coordinate: coordinate, radiusMetres: 5000)

    #expect(sut.selectedRadiusMetres == 2000)
  }

  @Test func prefillByName_withRadiusBelowMinimum_clampsTo100() throws {
    let (sut, _) = makeSUT()
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(name: "Mum's House", coordinate: coordinate, radiusMetres: 10)

    #expect(sut.selectedRadiusMetres == 100)
  }

  @Test func confirmRadius_withoutAnyPrefillOrPostcode_doesNothing() {
    // Guards against a stray confirmRadius() call before either prefill path
    // or submitPostcode() ever ran — should never crash or fabricate a zone.
    let (sut, _) = makeSUT()

    sut.confirmRadius()

    #expect(sut.currentStep == .welcome)
  }
}
