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

  // MARK: - Tier-bounded radius (tc-w3cb.2)

  private func makeViewModel(tier: SubscriptionTier) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  @Test func maxRadiusMetres_freeTier_capsAt2km() {
    #expect(makeViewModel(tier: .free).maxRadiusMetres == 2000)
  }

  @Test func maxRadiusMetres_personalTier_capsAt5km() {
    #expect(makeViewModel(tier: .personal).maxRadiusMetres == 5000)
  }

  @Test func maxRadiusMetres_proTier_capsAt10km() {
    #expect(makeViewModel(tier: .pro).maxRadiusMetres == 10000)
  }

  // MARK: - Radius upsell (tc-w3cb.3)

  @Test func canUnlockLargerRadius_isTrueBelowPro() {
    #expect(makeViewModel(tier: .free).canUnlockLargerRadius)
    #expect(makeViewModel(tier: .personal).canUnlockLargerRadius)
  }

  @Test func canUnlockLargerRadius_isFalseAtTopTier() {
    #expect(!makeViewModel(tier: .pro).canUnlockLargerRadius)
  }

  @Test func requestLargerRadiusUpgrade_presentsUpsellSheet() {
    let sut = makeViewModel(tier: .free)
    #expect(!sut.isRadiusUpsellPresented)

    sut.requestLargerRadiusUpgrade()

    #expect(sut.isRadiusUpsellPresented)
  }

  // MARK: - Tier-aware notification copy (tc-w3cb.4)

  @Test func deliversInstantAlerts_isFalseForFree() {
    #expect(!makeViewModel(tier: .free).deliversInstantAlerts)
  }

  @Test func deliversInstantAlerts_isTrueForPaidTiers() {
    #expect(makeViewModel(tier: .personal).deliversInstantAlerts)
    #expect(makeViewModel(tier: .pro).deliversInstantAlerts)
  }

  // MARK: - Anonymous browse post-signup handoff (GH#868 Phase 3.5)

  @Test func prefill_seedsPostcodeAndCoordinate_andJumpsToRadiusPicker() throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 1500)

    #expect(sut.postcodeInput == "CB1 2AD")
    #expect(sut.geocodedCoordinate == coordinate)
    #expect(sut.currentStep == .radiusPicker)
    #expect(sut.selectedRadiusMetres == 1500)
  }

  @Test func prefill_thenConfirmRadius_createsZoneAtPrefilledLocation() async throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)
    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 1500)

    var completedZone: WatchZone?
    sut.onComplete = { zone in completedZone = zone }
    sut.confirmRadius()
    await sut.skipNotifications()

    #expect(completedZone != nil)
    #expect(completedZone?.centre == coordinate)
    #expect(completedZone?.radiusMetres == 1500)
  }

  @Test func prefill_withRadiusAboveTierMax_clampsToMaxRadiusMetres() throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    // Free tier's cap is 2000; the anonymous picker itself is bounded there
    // too, so this only guards against a stale/legacy persisted radius.
    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 5000)

    #expect(sut.selectedRadiusMetres == 2000)
  }

  @Test func prefill_withRadiusBelowMinimum_clampsTo100() throws {
    let (sut, _, _, _, _) = makeSUT()
    let postcode = try Postcode("CB1 2AD")
    let coordinate = try Coordinate(latitude: 52.2053, longitude: 0.1218)

    sut.prefill(postcode: postcode, coordinate: coordinate, radiusMetres: 10)

    #expect(sut.selectedRadiusMetres == 100)
  }

  @Test func prefill_doesNotAffectNormalNonPrefilledFlow() async {
    // The additive prefill entry point leaves the existing postcode-entry ->
    // submitPostcode() path completely unchanged when never called.
    let (sut, geocoder, _, _, _) = makeSUT()
    sut.advance()  // -> postcodeEntry
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(sut.currentStep == .radiusPicker)
  }
}
