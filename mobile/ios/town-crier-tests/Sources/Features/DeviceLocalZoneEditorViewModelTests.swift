import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 4: create/edit a single device-local zone — name, postcode
/// entry geocoded client-side, radius clamped to
/// `[DeviceLocalZone.minRadiusMetres, DeviceLocalZone.maxRadiusMetres]`.
@Suite("DeviceLocalZoneEditorViewModel")
@MainActor
struct DeviceLocalZoneEditorViewModelTests {
  private func makeSUT(
    editing zone: DeviceLocalZone? = nil
  ) -> (DeviceLocalZoneEditorViewModel, SpyPostcodeGeocoder, SpyDeviceLocalZoneRepository) {
    let geocoder = SpyPostcodeGeocoder()
    let repository = SpyDeviceLocalZoneRepository()
    let sut = DeviceLocalZoneEditorViewModel(
      geocoder: geocoder, repository: repository, editing: zone)
    return (sut, geocoder, repository)
  }

  // MARK: - New zone

  @Test func init_new_isNotEditing() {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isEditing)
    #expect(sut.isPostcodeFieldVisible)
    #expect(sut.geocodedCoordinate == nil)
  }

  @Test func init_new_defaultRadiusIsWithinRange() {
    let (sut, _, _) = makeSUT()

    #expect(sut.selectedRadiusMetres >= DeviceLocalZone.minRadiusMetres)
    #expect(sut.selectedRadiusMetres <= DeviceLocalZone.maxRadiusMetres)
  }

  @Test func radiusBounds_matchDeviceLocalZoneClamp() {
    let (sut, _, _) = makeSUT()

    #expect(sut.minRadiusMetres == DeviceLocalZone.minRadiusMetres)
    #expect(sut.maxRadiusMetres == DeviceLocalZone.maxRadiusMetres)
  }

  // MARK: - Editing an existing zone

  @Test func init_editing_seedsFieldsFromZone() throws {
    let zone = try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1500)

    let (sut, _, _) = makeSUT(editing: zone)

    #expect(sut.isEditing)
    #expect(!sut.isPostcodeFieldVisible)
    #expect(sut.nameInput == "Home")
    #expect(sut.selectedRadiusMetres == 1500)
    #expect(sut.geocodedCoordinate == .cambridge)
  }

  // MARK: - Postcode submission

  @Test func submitPostcode_validPostcode_setsGeocodedCoordinate() async throws {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.geocodedCoordinate == .cambridge)
    #expect(sut.error == nil)
  }

  @Test func submitPostcode_emptyNameInput_defaultsNameToPostcode() async throws {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.nameInput == "CB1 2AD")
  }

  @Test func submitPostcode_existingNameInput_doesNotOverwriteName() async throws {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.nameInput = "My Custom Name"
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.nameInput == "My Custom Name")
  }

  @Test func submitPostcode_invalidPostcode_setsError() async {
    let (sut, _, _) = makeSUT()
    sut.postcodeInput = "not a postcode"

    await sut.submitPostcode()

    #expect(sut.geocodedCoordinate == nil)
    #expect(sut.error != nil)
  }

  @Test func submitPostcode_geocoderFailure_setsError() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .failure(DomainError.geocodingFailed("CB1 2AD"))
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.geocodedCoordinate == nil)
    #expect(sut.error == .geocodingFailed("CB1 2AD"))
  }

  // MARK: - Save

  @Test func save_noGeocodedCoordinate_returnsFalse() async {
    let (sut, _, repository) = makeSUT()

    let result = await sut.save()

    #expect(!result)
    #expect(repository.saveCalls.isEmpty)
  }

  @Test func save_validZone_persistsAndReturnsTrue() async throws {
    let (sut, geocoder, repository) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()
    sut.nameInput = "Home"
    sut.selectedRadiusMetres = 2000

    let result = await sut.save()

    #expect(result)
    #expect(repository.saveCalls.count == 1)
    #expect(repository.saveCalls.first?.name == "Home")
    #expect(repository.saveCalls.first?.radiusMetres == 2000)
  }

  @Test func save_invokesOnSaveWithTheSavedZone() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()
    sut.nameInput = "Home"
    var saved: DeviceLocalZone?
    sut.onSave = { saved = $0 }

    await sut.save()

    #expect(saved?.name == "Home")
  }

  @Test func save_editingExistingZone_reusesTheSameId() async throws {
    let zone = try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1000)
    let (sut, _, repository) = makeSUT(editing: zone)
    sut.nameInput = "Renamed"

    await sut.save()

    #expect(repository.saveCalls.first?.id == zone.id)
    #expect(repository.saveCalls.first?.name == "Renamed")
  }

  @Test func save_repositoryThrowsLimitReached_invokesOnRequestSignUpAndReturnsFalse() async {
    let (sut, geocoder, repository) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()
    sut.nameInput = "Home"
    repository.saveError = DomainError.deviceLocalZoneLimitReached
    var requested = false
    sut.onRequestSignUp = { requested = true }

    let result = await sut.save()

    #expect(!result)
    #expect(requested)
    #expect(sut.error == nil)
  }

  @Test func save_repositoryThrowsOtherError_setsInlineErrorAndReturnsFalse() async {
    let (sut, geocoder, repository) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.submitPostcode()
    sut.nameInput = "Home"
    repository.saveError = DomainError.networkUnavailable
    var requested = false
    sut.onRequestSignUp = { requested = true }

    let result = await sut.save()

    #expect(!result)
    #expect(!requested)
    #expect(sut.error == .networkUnavailable)
  }
}
