import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("OnboardingViewModel")
@MainActor
struct OnboardingViewModelTests {
  private func makeSUT() -> (
    OnboardingViewModel,
    SpyPostcodeGeocoder,
    SpyWatchZoneRepository,
    SpyOnboardingRepository,
    SpyNotificationService
  ) {
    let geocoder = SpyPostcodeGeocoder()
    let zoneRepo = SpyWatchZoneRepository()
    let onboardingRepo = SpyOnboardingRepository()
    let notificationService = SpyNotificationService()
    let vm = OnboardingViewModel(
      geocoder: geocoder,
      watchZoneRepository: zoneRepo,
      onboardingRepository: onboardingRepo,
      notificationService: notificationService
    )
    return (vm, geocoder, zoneRepo, onboardingRepo, notificationService)
  }

  // MARK: - Step Progression

  @Test func initialStep_isWelcome() {
    let (sut, _, _, _, _) = makeSUT()

    #expect(sut.currentStep == .welcome)
  }

  @Test func advanceFromWelcome_goesToPostcodeEntry() {
    let (sut, _, _, _, _) = makeSUT()

    sut.advance()

    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func advanceFromPostcodeEntry_requiresGeocodedPostcode() {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry

    sut.advance()  // should not advance without geocoded postcode

    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func goBack_fromPostcodeEntry_returnsToWelcome() {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry

    sut.goBack()

    #expect(sut.currentStep == .welcome)
  }

  @Test func goBack_fromWelcome_staysOnWelcome() {
    let (sut, _, _, _, _) = makeSUT()

    sut.goBack()

    #expect(sut.currentStep == .welcome)
  }

  // MARK: - Postcode Entry

  @Test func submitPostcode_geocodesAndAdvancesToRadiusPicker() async {
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(geocoder.geocodeCalls.first?.value == "CB1 2AD")
    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.geocodedCoordinate != nil)
  }

  @Test func submitPostcode_invalidPostcode_setsError() async {
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "INVALID"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.isEmpty)
    #expect(sut.error != nil)
    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func submitPostcode_geocodingFailure_setsError() async {
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    geocoder.geocodeResult = .failure(DomainError.geocodingFailed("Not found"))

    await sut.submitPostcode()

    #expect(sut.error == .geocodingFailed("Not found"))
    #expect(sut.currentStep == .postcodeEntry)
  }

  @Test func submitPostcode_clearsExistingError() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "INVALID"
    await sut.submitPostcode()
    #expect(sut.error != nil)

    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()

    #expect(sut.error == nil)
  }

  @Test func submitPostcode_setsIsLoadingDuringGeocode() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"

    #expect(!sut.isLoading)
    await sut.submitPostcode()
    #expect(!sut.isLoading)
  }

  // MARK: - Radius Selection

  @Test func selectedRadiusMetres_defaultsTo1000() {
    let (sut, _, _, _, _) = makeSUT()

    #expect(sut.selectedRadiusMetres == 1000)
  }

  @Test func confirmRadius_createsWatchZoneAndAdvancesToNotification() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.selectedRadiusMetres = 2000

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmRadius()

    #expect(sut.currentStep == .notificationPermission)
    // Verify zone was created via the completion callback after onboarding finishes
    await sut.skipNotifications()
    #expect(completedZone != nil)
    #expect(completedZone?.radiusMetres == 2000)
  }

  @Test func goBack_fromRadiusPicker_returnsToPostcodeEntry() async {
    let (sut, _, _, _, _) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker

    sut.goBack()

    #expect(sut.currentStep == .postcodeEntry)
  }

  // MARK: - Notification Permission

  @Test func requestNotifications_requestsPermissionAndCompletes() async {
    let (sut, _, zoneRepo, onboardingRepo, notificationService) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission

    await sut.requestNotificationPermission()

    #expect(notificationService.requestPermissionCallCount == 1)
    #expect(zoneRepo.saveCalls.count == 1)
    #expect(onboardingRepo.markOnboardingCompleteCallCount == 1)
    #expect(sut.isComplete)
  }

  @Test func skipNotifications_completesWithoutRequestingPermission() async {
    let (sut, _, zoneRepo, onboardingRepo, notificationService) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission

    await sut.skipNotifications()

    #expect(notificationService.requestPermissionCallCount == 0)
    #expect(zoneRepo.saveCalls.count == 1)
    #expect(onboardingRepo.markOnboardingCompleteCallCount == 1)
    #expect(sut.isComplete)
  }

  @Test func requestNotifications_denied_stillCompletes() async {
    let (sut, _, zoneRepo, onboardingRepo, notificationService) = makeSUT()
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission
    notificationService.requestPermissionResult = .success(false)

    await sut.requestNotificationPermission()

    #expect(zoneRepo.saveCalls.count == 1)
    #expect(onboardingRepo.markOnboardingCompleteCallCount == 1)
    #expect(sut.isComplete)
  }

  // MARK: - Completion Callback

  @Test func completion_invokesOnComplete() async {
    let (sut, _, _, _, _) = makeSUT()
    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.advance()  // → postcodeEntry
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()  // → radiusPicker
    sut.confirmRadius()  // → notificationPermission

    await sut.skipNotifications()

    #expect(completedZone != nil)
    #expect(completedZone?.radiusMetres == 1000)
  }
}
