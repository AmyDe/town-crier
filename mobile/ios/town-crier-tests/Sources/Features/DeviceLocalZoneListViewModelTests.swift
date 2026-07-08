import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#888: the anonymous Zones tab now holds exactly one editable
/// device-local zone (`DeviceLocalZone.maxZoneCount == 1`) — no add, no
/// delete. Any alert affordance (row bell or the persistent sign-up pitch)
/// routes to the sign-up CTA, and a successful edit notifies
/// `onZonesChanged` so the coordinator can propagate it live to the Map and
/// Applications tabs.
@Suite("DeviceLocalZoneListViewModel")
@MainActor
struct DeviceLocalZoneListViewModelTests {
  private func makeSUT() -> (
    DeviceLocalZoneListViewModel, SpyDeviceLocalZoneRepository, SpyPostcodeGeocoder
  ) {
    let repository = SpyDeviceLocalZoneRepository()
    let geocoder = SpyPostcodeGeocoder()
    let sut = DeviceLocalZoneListViewModel(repository: repository, geocoder: geocoder)
    return (sut, repository, geocoder)
  }

  private func makeZone(name: String = "Home") throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: 1000)
  }

  // MARK: - Load

  @Test func load_populatesZonesFromRepository() throws {
    let (sut, repository, _) = makeSUT()
    let zone = try makeZone(name: "Home")
    repository.loadAllResult = [zone]

    sut.load()

    #expect(sut.zones == [zone])
  }

  // MARK: - editZone

  @Test func editZone_opensEditorForThatZone() throws {
    let (sut, _, _) = makeSUT()
    let zone = try makeZone()

    sut.editZone(zone)

    #expect(sut.editorTarget == zone)
  }

  // MARK: - Alert affordance -> sign-up CTA

  @Test func requestAlertsSignUp_presentsSignUpCTA() {
    let (sut, _, _) = makeSUT()

    sut.requestAlertsSignUp()

    #expect(sut.isSignUpCTAPresented)
  }

  /// The persistent "want more areas or alerts?" pitch below the zone
  /// (GH#888) is the only remaining route to another area now the cap is
  /// one — it routes to the same sign-up CTA as the per-row bell.
  @Test func requestSignUpFromPitch_presentsSignUpCTA() {
    let (sut, _, _) = makeSUT()

    sut.requestSignUpFromPitch()

    #expect(sut.isSignUpCTAPresented)
  }

  @Test func dismissSignUpCTA_clearsTheFlag() {
    let (sut, _, _) = makeSUT()
    sut.requestAlertsSignUp()

    sut.dismissSignUpCTA()

    #expect(!sut.isSignUpCTAPresented)
  }

  @Test func confirmSignUp_clearsTheFlagAndInvokesOnRequestSignUp() {
    let (sut, _, _) = makeSUT()
    sut.requestAlertsSignUp()
    var requested = false
    sut.onRequestSignUp = { requested = true }

    sut.confirmSignUp()

    #expect(!sut.isSignUpCTAPresented)
    #expect(requested)
  }

  // MARK: - Editor dismissal / reload

  @Test func dismissEditor_clearsEditorTarget() throws {
    let (sut, _, _) = makeSUT()
    sut.editZone(try makeZone())

    sut.dismissEditor()

    #expect(sut.editorTarget == nil)
  }

  @Test func makeEditorViewModel_seedsFromTheZoneBeingEdited() throws {
    let (sut, _, _) = makeSUT()
    let zone = try makeZone(name: "Existing")

    let editorVM = sut.makeEditorViewModel(for: zone)

    #expect(editorVM.nameInput == "Existing")
  }

  @Test func makeEditorViewModel_onSave_dismissesEditorAndReloadsZones() throws {
    let (sut, repository, _) = makeSUT()
    let zone = try makeZone(name: "Home")
    sut.editZone(zone)
    let editorVM = sut.makeEditorViewModel(for: zone)
    let saved = try DeviceLocalZone(
      id: zone.id, name: "Renamed", centre: .cambridge, radiusMetres: 2000)
    repository.loadAllResult = [saved]

    editorVM.onSave?(saved)

    #expect(sut.editorTarget == nil)
    #expect(sut.zones == [saved])
  }

  /// GH#888 acceptance criterion: a successful editor save fires
  /// `onZonesChanged` with the saved zone, so the coordinator can propagate
  /// it to the Map and Applications tabs.
  @Test func makeEditorViewModel_onSave_invokesOnZonesChangedWithTheSavedZone() throws {
    let (sut, repository, _) = makeSUT()
    let zone = try makeZone(name: "Home")
    sut.editZone(zone)
    let editorVM = sut.makeEditorViewModel(for: zone)
    let saved = try DeviceLocalZone(
      id: zone.id, name: "Renamed", centre: .cambridge, radiusMetres: 2000)
    repository.loadAllResult = [saved]
    var changed: [DeviceLocalZone] = []
    sut.onZonesChanged = { changed.append($0) }

    editorVM.onSave?(saved)

    #expect(changed == [saved])
  }

  /// The regression this bead fixes (live-verified): the postcode field was
  /// previously hidden whenever the editor opened in "editing" mode, which
  /// GH#888 made the ONLY reachable mode — permanently stranding a mistyped
  /// onboarding postcode. Proves the field is reachable end-to-end: editing
  /// the postcode and saving still fires `onZonesChanged` with the
  /// corrected coordinate.
  @Test func makeEditorViewModel_afterCorrectingPostcode_onSave_invokesOnZonesChangedWithNewCoordinate() async throws {
    let (sut, _, geocoder) = makeSUT()
    let zone = try makeZone(name: "Home")
    sut.editZone(zone)
    let editorVM = sut.makeEditorViewModel(for: zone)
    let newCoordinate = try Coordinate(latitude: 51.5074, longitude: -0.1278)
    geocoder.geocodeResult = .success(newCoordinate)
    editorVM.postcodeInput = "SW1A 1AA"
    await editorVM.submitPostcode()
    var changed: [DeviceLocalZone] = []
    sut.onZonesChanged = { changed.append($0) }

    await editorVM.save()

    #expect(changed.first?.centre == newCoordinate)
  }

  @Test func makeEditorViewModel_onRequestSignUp_dismissesEditorAndPresentsCTA() throws {
    let (sut, _, _) = makeSUT()
    let zone = try makeZone()
    sut.editZone(zone)
    let editorVM = sut.makeEditorViewModel(for: zone)

    editorVM.onRequestSignUp?()

    #expect(sut.editorTarget == nil)
    #expect(sut.isSignUpCTAPresented)
  }
}
