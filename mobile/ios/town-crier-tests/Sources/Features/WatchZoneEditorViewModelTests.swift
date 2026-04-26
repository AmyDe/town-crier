import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("WatchZoneEditorViewModel — create mode")
struct WatchZoneEditorCreateTests {
  private var spyGeocoder: SpyPostcodeGeocoder!
  private var spyRepository: SpyWatchZoneRepository!
  private var sut: WatchZoneEditorViewModel!

  init() {
    spyGeocoder = SpyPostcodeGeocoder()
    spyRepository = SpyWatchZoneRepository()
    sut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .personal
    )
  }

  @Test func initialState_isCreate() {
    #expect(!sut.isEditing)
    #expect(sut.postcodeInput.isEmpty)
    #expect(sut.nameInput.isEmpty)
    #expect(sut.selectedRadiusMetres == 1000)
    #expect(sut.geocodedCoordinate == nil)
  }

  @Test func submitPostcode_geocodesAndSetsCoordinate() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)

    await sut.submitPostcode()

    #expect(sut.geocodedCoordinate == .cambridge)
    #expect(spyGeocoder.geocodeCalls.count == 1)
    #expect(!sut.isLoading)
  }

  @Test func submitPostcode_invalidPostcode_setsError() async {
    sut.postcodeInput = "INVALID"

    await sut.submitPostcode()

    #expect(sut.error != nil)
    #expect(sut.geocodedCoordinate == nil)
  }

  @Test func submitPostcode_geocodingFails_setsError() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .failure(DomainError.geocodingFailed("not found"))

    await sut.submitPostcode()

    #expect(sut.error == .geocodingFailed("not found"))
  }

  @Test func radiusOptions_reflectTierLimits() {
    let limits = WatchZoneLimits(tier: .personal)
    #expect(sut.availableRadiusOptions == limits.availableRadiusOptions)
  }

  @Test func radiusOptions_freeTier_capped() {
    let freeSut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .free
    )
    let limits = WatchZoneLimits(tier: .free)
    #expect(freeSut.availableRadiusOptions == limits.availableRadiusOptions)
  }

  @Test func isPostcodeFieldVisible_inCreateMode_isTrue() {
    #expect(sut.isPostcodeFieldVisible)
  }

  @Test func maxRadiusMetres_freeTier_is2000() {
    let freeSut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .free
    )
    #expect(freeSut.maxRadiusMetres == 2000)
  }

  @Test func maxRadiusMetres_personalTier_is5000() {
    #expect(sut.maxRadiusMetres == 5000)
  }

  @Test func maxRadiusMetres_proTier_is10000() {
    let proSut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .pro
    )
    #expect(proSut.maxRadiusMetres == 10000)
  }

  @Test func submitPostcode_autoFillsNameFromPostcode_whenNameEmpty() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)

    await sut.submitPostcode()

    #expect(sut.nameInput == "CB1 2AD")
  }

  @Test func submitPostcode_doesNotOverwriteName_whenAlreadySet() async {
    sut.nameInput = "My Home"
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)

    await sut.submitPostcode()

    #expect(sut.nameInput == "My Home")
  }

  @Test func save_usesNameInput_notPostcode() async {
    sut.postcodeInput = "CB1 2AD"
    sut.nameInput = "My Home Zone"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
    sut.selectedRadiusMetres = 2000

    await sut.save()

    #expect(spyRepository.saveCalls.count == 1)
    let saved = spyRepository.saveCalls.first
    #expect(saved?.name == "My Home Zone")
    #expect(saved?.centre == .cambridge)
    #expect(saved?.radiusMetres == 2000)
  }

  @Test func save_createsZoneAndCallsRepository() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
    sut.selectedRadiusMetres = 2000

    await sut.save()

    #expect(spyRepository.saveCalls.count == 1)
    let saved = spyRepository.saveCalls.first
    #expect(saved?.name == "CB1 2AD")
    #expect(saved?.centre == .cambridge)
    #expect(saved?.radiusMetres == 2000)
  }

  @Test func save_invokesOnSaveCallback() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
    var savedZone: WatchZone?
    sut.onSave = { savedZone = $0 }

    await sut.save()

    #expect(savedZone != nil)
  }

  @Test func save_withoutGeocoding_doesNothing() async {
    await sut.save()

    #expect(spyRepository.saveCalls.isEmpty)
  }

  @Test func save_repositoryFails_setsError() async {
    sut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await sut.submitPostcode()
    spyRepository.saveResult = .failure(DomainError.networkUnavailable)

    await sut.save()

    #expect(sut.error == .networkUnavailable)
  }

  @Test func save_clampsRadiusToTierMax() async {
    let freeSut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .free
    )
    freeSut.postcodeInput = "CB1 2AD"
    spyGeocoder.geocodeResult = .success(.cambridge)
    await freeSut.submitPostcode()
    freeSut.selectedRadiusMetres = 5000

    await freeSut.save()

    let saved = spyRepository.saveCalls.first
    #expect(saved?.radiusMetres == 2000)
  }
}

@MainActor
@Suite("WatchZoneEditorViewModel — edit mode")
struct WatchZoneEditorEditTests {
  private var spyGeocoder: SpyPostcodeGeocoder!
  private var spyRepository: SpyWatchZoneRepository!
  private var sut: WatchZoneEditorViewModel!

  init() {
    spyGeocoder = SpyPostcodeGeocoder()
    spyRepository = SpyWatchZoneRepository()
    sut = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .personal,
      editing: .cambridge
    )
  }

  @Test func initialState_populatedFromExistingZone() {
    #expect(sut.isEditing)
    #expect(sut.nameInput == "CB1 2AD")
    #expect(sut.postcodeInput.isEmpty)
    #expect(sut.selectedRadiusMetres == 2000)
    #expect(sut.geocodedCoordinate == .cambridge)
  }

  @Test func initialState_freeformName_populatesNameInput() throws {
    let freeformZone = try WatchZone(
      id: WatchZoneId("zone-003"),
      name: "My Home Zone",
      centre: .cambridge,
      radiusMetres: 1500
    )
    let vm = WatchZoneEditorViewModel(
      geocoder: spyGeocoder,
      repository: spyRepository,
      tier: .personal,
      editing: freeformZone
    )
    #expect(vm.nameInput == "My Home Zone")
    #expect(vm.postcodeInput.isEmpty)
    #expect(vm.geocodedCoordinate == .cambridge)
  }

  @Test func isPostcodeFieldVisible_inEditMode_isFalse() {
    #expect(!sut.isPostcodeFieldVisible)
  }

  @Test func save_preservesExistingId() async {
    await sut.save()

    let updated = spyRepository.updateCalls.first
    #expect(updated?.id == WatchZoneId("zone-001"))
  }

  @Test func save_callsRepositoryUpdate_notSave() async {
    await sut.save()

    #expect(spyRepository.updateCalls.count == 1)
    #expect(spyRepository.saveCalls.isEmpty)
    let updated = spyRepository.updateCalls.first
    #expect(updated?.id == WatchZoneId("zone-001"))
  }

  @Test func save_repositoryUpdateFails_setsError() async {
    spyRepository.updateResult = .failure(DomainError.networkUnavailable)

    await sut.save()

    #expect(sut.error == .networkUnavailable)
  }

  @Test func submitPostcode_updatesCoordinateForNewPostcode() async throws {
    sut.postcodeInput = "SW1A 1AA"
    let london = try Coordinate(latitude: 51.5014, longitude: -0.1419)
    spyGeocoder.geocodeResult = .success(london)

    await sut.submitPostcode()

    #expect(sut.geocodedCoordinate == london)
  }
}
