import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("DeviceLocalZoneEditorView")
struct DeviceLocalZoneEditorViewTests {
  private func makeViewModel(
    editing zone: DeviceLocalZone? = nil
  ) -> DeviceLocalZoneEditorViewModel {
    DeviceLocalZoneEditorViewModel(
      geocoder: SpyPostcodeGeocoder(),
      repository: SpyDeviceLocalZoneRepository(),
      editing: zone
    )
  }

  @Test func body_renders_inCreateMode_withoutCoordinate() {
    let vm = makeViewModel()
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_inEditMode() throws {
    let zone = try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1500)
    let vm = makeViewModel(editing: zone)
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_afterPostcodeGeocoded() async {
    let vm = makeViewModel()
    vm.postcodeInput = "CB1 2AD"
    await vm.submitPostcode()
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_withErrorState() async {
    let vm = makeViewModel()
    vm.postcodeInput = "not a postcode"
    await vm.submitPostcode()
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }
}
