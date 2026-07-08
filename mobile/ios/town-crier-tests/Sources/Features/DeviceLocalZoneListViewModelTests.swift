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
  private func makeSUT() -> (DeviceLocalZoneListViewModel, SpyDeviceLocalZoneRepository) {
    let repository = SpyDeviceLocalZoneRepository()
    let sut = DeviceLocalZoneListViewModel(repository: repository, geocoder: SpyPostcodeGeocoder())
    return (sut, repository)
  }

  private func makeZone(name: String = "Home") throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: 1000)
  }

  // MARK: - Load

  @Test func load_populatesZonesFromRepository() throws {
    let (sut, repository) = makeSUT()
    let zone = try makeZone(name: "Home")
    repository.loadAllResult = [zone]

    sut.load()

    #expect(sut.zones == [zone])
  }

  // MARK: - editZone

  @Test func editZone_opensEditorForThatZone() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()

    sut.editZone(zone)

    #expect(sut.editorTarget == zone)
  }

  // MARK: - Alert affordance -> sign-up CTA

  @Test func requestAlertsSignUp_presentsSignUpCTA() {
    let (sut, _) = makeSUT()

    sut.requestAlertsSignUp()

    #expect(sut.isSignUpCTAPresented)
  }

  /// The persistent "want more areas or alerts?" pitch below the zone
  /// (GH#888) is the only remaining route to another area now the cap is
  /// one — it routes to the same sign-up CTA as the per-row bell.
  @Test func requestSignUpFromPitch_presentsSignUpCTA() {
    let (sut, _) = makeSUT()

    sut.requestSignUpFromPitch()

    #expect(sut.isSignUpCTAPresented)
  }

  @Test func dismissSignUpCTA_clearsTheFlag() {
    let (sut, _) = makeSUT()
    sut.requestAlertsSignUp()

    sut.dismissSignUpCTA()

    #expect(!sut.isSignUpCTAPresented)
  }

  @Test func confirmSignUp_clearsTheFlagAndInvokesOnRequestSignUp() {
    let (sut, _) = makeSUT()
    sut.requestAlertsSignUp()
    var requested = false
    sut.onRequestSignUp = { requested = true }

    sut.confirmSignUp()

    #expect(!sut.isSignUpCTAPresented)
    #expect(requested)
  }

  // MARK: - Editor dismissal / reload

  @Test func dismissEditor_clearsEditorTarget() throws {
    let (sut, _) = makeSUT()
    sut.editZone(try makeZone())

    sut.dismissEditor()

    #expect(sut.editorTarget == nil)
  }

  @Test func makeEditorViewModel_seedsFromTheZoneBeingEdited() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone(name: "Existing")

    let editorVM = sut.makeEditorViewModel(for: zone)

    #expect(editorVM.isEditing)
    #expect(editorVM.nameInput == "Existing")
  }

  @Test func makeEditorViewModel_onSave_dismissesEditorAndReloadsZones() throws {
    let (sut, repository) = makeSUT()
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
    let (sut, repository) = makeSUT()
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

  @Test func makeEditorViewModel_onRequestSignUp_dismissesEditorAndPresentsCTA() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()
    sut.editZone(zone)
    let editorVM = sut.makeEditorViewModel(for: zone)

    editorVM.onRequestSignUp?()

    #expect(sut.editorTarget == nil)
    #expect(sut.isSignUpCTAPresented)
  }
}
