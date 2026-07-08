import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#888: edits the anonymous user's single device-local zone. There is no
/// longer a "create new zone" mode (the on-device cap dropped to 1, and the
/// Zones tab's only entry point is `editZone(_:)` on the existing zone), so
/// `editing` is a required zone and `geocodedCoordinate` is always seeded —
/// never nil. The postcode field/`submitPostcode()` stay fully reachable
/// while editing (a live-verified regression: they were previously hidden
/// behind an `isEditing` gate that made sense only when a distinct "new
/// zone" mode existed), so a mistyped onboarding postcode can still be
/// corrected.
@Suite("DeviceLocalZoneEditorViewModel")
@MainActor
struct DeviceLocalZoneEditorViewModelTests {
  private func makeSUT(
    editing zone: DeviceLocalZone
  ) -> (DeviceLocalZoneEditorViewModel, SpyPostcodeGeocoder, SpyDeviceLocalZoneRepository) {
    let geocoder = SpyPostcodeGeocoder()
    let repository = SpyDeviceLocalZoneRepository()
    let sut = DeviceLocalZoneEditorViewModel(
      geocoder: geocoder, repository: repository, editing: zone)
    return (sut, geocoder, repository)
  }

  private func makeZone(
    name: String = "Home", radiusMetres: Double = 1000
  ) throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: radiusMetres)
  }

  // MARK: - Init seeds from the zone being edited

  @Test func init_seedsFieldsFromTheZoneBeingEdited() throws {
    let zone = try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1500)

    let (sut, _, _) = makeSUT(editing: zone)

    #expect(sut.nameInput == "Home")
    #expect(sut.selectedRadiusMetres == 1500)
    #expect(sut.geocodedCoordinate == .cambridge)
  }

  @Test func radiusBounds_matchDeviceLocalZoneClamp() throws {
    let (sut, _, _) = makeSUT(editing: try makeZone())

    #expect(sut.minRadiusMetres == DeviceLocalZone.minRadiusMetres)
    #expect(sut.maxRadiusMetres == DeviceLocalZone.maxRadiusMetres)
  }

  // MARK: - Postcode submission (GH#888: reachable while editing)

  /// The regression this bead fixes: correcting the postcode of the
  /// anonymous user's single (already-geocoded) zone must actually update
  /// `geocodedCoordinate` — proving nothing gates `submitPostcode()` or its
  /// effect away just because the zone already has a coordinate.
  @Test func submitPostcode_whileEditingAnExistingZone_updatesGeocodedCoordinateToTheNewPostcode() async throws {
    let existingZone = try makeZone()
    let (sut, geocoder, _) = makeSUT(editing: existingZone)
    let newCoordinate = try Coordinate(latitude: 51.5074, longitude: -0.1278)
    geocoder.geocodeResult = .success(newCoordinate)
    sut.postcodeInput = "SW1A 1AA"

    await sut.submitPostcode()

    #expect(sut.geocodedCoordinate == newCoordinate)
    #expect(sut.geocodedCoordinate != existingZone.centre)
    #expect(sut.error == nil)
  }

  @Test func submitPostcode_emptyNameInput_defaultsNameToPostcode() async throws {
    let (sut, geocoder, _) = makeSUT(editing: try makeZone())
    sut.nameInput = ""
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.nameInput == "CB1 2AD")
  }

  @Test func submitPostcode_existingNameInput_doesNotOverwriteName() async throws {
    let (sut, geocoder, _) = makeSUT(editing: try makeZone(name: "My Custom Name"))
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.nameInput == "My Custom Name")
  }

  @Test func submitPostcode_invalidPostcode_setsError() async throws {
    let (sut, _, _) = makeSUT(editing: try makeZone())
    sut.postcodeInput = "not a postcode"

    await sut.submitPostcode()

    #expect(sut.error != nil)
  }

  @Test func submitPostcode_geocoderFailure_setsError() async throws {
    let (sut, geocoder, _) = makeSUT(editing: try makeZone())
    geocoder.geocodeResult = .failure(DomainError.geocodingFailed("CB1 2AD"))
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.error == .geocodingFailed("CB1 2AD"))
  }

  // MARK: - Save

  @Test func save_persistsAndReturnsTrue() async throws {
    let (sut, _, repository) = makeSUT(editing: try makeZone(name: "Home"))
    sut.selectedRadiusMetres = 2000

    let result = await sut.save()

    #expect(result)
    #expect(repository.saveCalls.count == 1)
    #expect(repository.saveCalls.first?.name == "Home")
    #expect(repository.saveCalls.first?.radiusMetres == 2000)
  }

  @Test func save_invokesOnSaveWithTheSavedZone() async throws {
    let (sut, _, _) = makeSUT(editing: try makeZone(name: "Home"))
    var saved: DeviceLocalZone?
    sut.onSave = { saved = $0 }

    await sut.save()

    #expect(saved?.name == "Home")
  }

  @Test func save_afterCorrectingThePostcode_persistsTheNewCoordinate() async throws {
    let (sut, geocoder, repository) = makeSUT(editing: try makeZone())
    let newCoordinate = try Coordinate(latitude: 51.5074, longitude: -0.1278)
    geocoder.geocodeResult = .success(newCoordinate)
    sut.postcodeInput = "SW1A 1AA"
    await sut.submitPostcode()

    await sut.save()

    #expect(repository.saveCalls.first?.centre == newCoordinate)
  }

  @Test func save_reusesTheZonesExistingId() async throws {
    let zone = try makeZone(name: "Home")
    let (sut, _, repository) = makeSUT(editing: zone)
    sut.nameInput = "Renamed"

    await sut.save()

    #expect(repository.saveCalls.first?.id == zone.id)
    #expect(repository.saveCalls.first?.name == "Renamed")
  }

  @Test func save_repositoryThrowsLimitReached_invokesOnRequestSignUpAndReturnsFalse() async throws {
    let (sut, _, repository) = makeSUT(editing: try makeZone())
    repository.saveError = DomainError.deviceLocalZoneLimitReached
    var requested = false
    sut.onRequestSignUp = { requested = true }

    let result = await sut.save()

    #expect(!result)
    #expect(requested)
    #expect(sut.error == nil)
  }

  @Test func save_repositoryThrowsOtherError_setsInlineErrorAndReturnsFalse() async throws {
    let (sut, _, repository) = makeSUT(editing: try makeZone())
    repository.saveError = DomainError.networkUnavailable
    var requested = false
    sut.onRequestSignUp = { requested = true }

    let result = await sut.save()

    #expect(!result)
    #expect(!requested)
    #expect(sut.error == .networkUnavailable)
  }
}
