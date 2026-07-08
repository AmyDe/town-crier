import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#888: no more "create" mode — the editor always opens on the anonymous
/// user's existing single zone, and the postcode field renders
/// unconditionally (see `DeviceLocalZoneEditorViewModel`'s docs).
@MainActor
@Suite("DeviceLocalZoneEditorView")
struct DeviceLocalZoneEditorViewTests {
  private func makeZone(name: String = "Home") throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: 1500)
  }

  private func makeViewModel(editing zone: DeviceLocalZone) -> DeviceLocalZoneEditorViewModel {
    DeviceLocalZoneEditorViewModel(
      geocoder: SpyPostcodeGeocoder(),
      repository: SpyDeviceLocalZoneRepository(),
      editing: zone
    )
  }

  @Test func body_renders() throws {
    let vm = makeViewModel(editing: try makeZone())
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_afterPostcodeGeocoded() async throws {
    let vm = makeViewModel(editing: try makeZone())
    vm.postcodeInput = "CB1 2AD"
    await vm.submitPostcode()
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_withErrorState() async throws {
    let vm = makeViewModel(editing: try makeZone())
    vm.postcodeInput = "not a postcode"
    await vm.submitPostcode()
    let sut = DeviceLocalZoneEditorView(viewModel: vm)
    _ = sut.body
  }
}
