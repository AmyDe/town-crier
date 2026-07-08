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

  @Test func body_renders_withZones() throws {
    let (vm, repository) = makeViewModel()
    repository.loadAllResult = [try makeZone(name: "One"), try makeZone(name: "Two")]
    vm.load()

    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenEditorPresented() {
    let (vm, _) = makeViewModel()
    vm.addZoneTapped()

    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenSignUpCTAPresented() throws {
    let (vm, repository) = makeViewModel()
    repository.loadAllResult = [
      try makeZone(name: "1"), try makeZone(name: "2"), try makeZone(name: "3"),
    ]
    vm.load()
    vm.addZoneTapped()

    #expect(vm.isSignUpCTAPresented)

    let sut = DeviceLocalZoneListView(viewModel: vm)
    _ = sut.body
  }
}
