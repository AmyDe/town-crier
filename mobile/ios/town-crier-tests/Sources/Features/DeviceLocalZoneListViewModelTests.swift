import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 4: the anonymous Zones tab — up to `DeviceLocalZone.maxZoneCount`
/// device-local areas with create/edit/delete. Any alert affordance and any
/// attempt to add a 4th zone route to the sign-up CTA rather than a real
/// quota/entitlement flow.
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
    repository.loadAllResult = [try makeZone(name: "One"), try makeZone(name: "Two")]

    sut.load()

    #expect(sut.zones.count == 2)
  }

  // MARK: - canAddZone

  @Test func canAddZone_true_belowCap() throws {
    let (sut, repository) = makeSUT()
    repository.loadAllResult = [try makeZone(), try makeZone()]
    sut.load()

    #expect(sut.canAddZone)
  }

  @Test func canAddZone_false_atCap() throws {
    let (sut, repository) = makeSUT()
    repository.loadAllResult = [try makeZone(name: "1"), try makeZone(name: "2"), try makeZone(name: "3")]
    sut.load()

    #expect(!sut.canAddZone)
  }

  // MARK: - addZoneTapped

  @Test func addZoneTapped_belowCap_opensNewEditor() {
    let (sut, _) = makeSUT()
    sut.load()

    sut.addZoneTapped()

    #expect(sut.editorTarget == .new)
    #expect(!sut.isSignUpCTAPresented)
  }

  @Test func addZoneTapped_atCap_presentsSignUpCTA() throws {
    let (sut, repository) = makeSUT()
    repository.loadAllResult = [try makeZone(name: "1"), try makeZone(name: "2"), try makeZone(name: "3")]
    sut.load()

    sut.addZoneTapped()

    #expect(sut.editorTarget == nil)
    #expect(sut.isSignUpCTAPresented)
  }

  // MARK: - editZone

  @Test func editZone_opensEditorForThatZone() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone()

    sut.editZone(zone)

    #expect(sut.editorTarget == .edit(zone))
  }

  // MARK: - deleteZone

  @Test func deleteZone_removesFromRepositoryAndLocalList() throws {
    let (sut, repository) = makeSUT()
    let zone = try makeZone()
    repository.loadAllResult = [zone]
    sut.load()

    sut.deleteZone(zone)

    #expect(repository.deleteCalls == [zone.id])
    #expect(sut.zones.isEmpty)
  }

  // MARK: - Alert affordance -> sign-up CTA

  @Test func requestAlertsSignUp_presentsSignUpCTA() {
    let (sut, _) = makeSUT()

    sut.requestAlertsSignUp()

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

  @Test func dismissEditor_clearsEditorTarget() {
    let (sut, _) = makeSUT()
    sut.addZoneTapped()

    sut.dismissEditor()

    #expect(sut.editorTarget == nil)
  }

  @Test func makeEditorViewModel_forNew_isNotEditing() {
    let (sut, _) = makeSUT()

    let editorVM = sut.makeEditorViewModel(for: .new)

    #expect(!editorVM.isEditing)
  }

  @Test func makeEditorViewModel_forEdit_seedsFromZone() throws {
    let (sut, _) = makeSUT()
    let zone = try makeZone(name: "Existing")

    let editorVM = sut.makeEditorViewModel(for: .edit(zone))

    #expect(editorVM.isEditing)
    #expect(editorVM.nameInput == "Existing")
  }

  @Test func makeEditorViewModel_onSave_dismissesEditorAndReloadsZones() throws {
    let (sut, repository) = makeSUT()
    sut.addZoneTapped()
    let editorVM = sut.makeEditorViewModel(for: .new)
    let saved = try makeZone(name: "Saved")
    repository.loadAllResult = [saved]

    editorVM.onSave?(saved)

    #expect(sut.editorTarget == nil)
    #expect(sut.zones == [saved])
  }

  @Test func makeEditorViewModel_onRequestSignUp_dismissesEditorAndPresentsCTA() {
    let (sut, _) = makeSUT()
    sut.addZoneTapped()
    let editorVM = sut.makeEditorViewModel(for: .new)

    editorVM.onRequestSignUp?()

    #expect(sut.editorTarget == nil)
    #expect(sut.isSignUpCTAPresented)
  }
}
