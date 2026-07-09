import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("DeviceLocalZoneListView")
@MainActor
struct DeviceLocalZoneListViewTests {
  private func makeViewModel() -> (DeviceLocalZoneListViewModel, SpyDeviceLocalZoneRepository) {
    let repository = SpyDeviceLocalZoneRepository()
    let vm = DeviceLocalZoneListViewModel(repository: repository, geocoder: SpyPostcodeGeocoder())
    return (vm, repository)
  }

  private func makeZone(name: String = "Home") throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: 1000)
  }

  @Test func body_renders_whenNoZones() {
    let (vm, _) = makeViewModel()
    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_withZone() throws {
    let (vm, repository) = makeViewModel()
    repository.loadAllResult = [try makeZone(name: "Home")]
    vm.load()

    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenEditorPresented() throws {
    let (vm, _) = makeViewModel()
    vm.editZone(try makeZone())

    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenSignUpCTAPresented() {
    let (vm, _) = makeViewModel()
    vm.requestAlertsSignUp()

    #expect(vm.isSignUpCTAPresented)

    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }

  // MARK: - Sign-up pitch upsell treatment (GH#896)

  @Test func copy_eyebrowMatchesApprovedText() {
    #expect(DeviceLocalZoneListView.Copy.eyebrow == "Free Account")
  }
}
