import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("DeviceLocalZoneConversionView")
@MainActor
struct DeviceLocalZoneConversionViewTests {
  private func makeZone(name: String = "Home") throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: 1000)
  }

  private func makeViewModel(zones: [DeviceLocalZone]) -> DeviceLocalZoneConversionViewModel {
    DeviceLocalZoneConversionViewModel(
      zones: zones,
      watchZoneRepository: SpyWatchZoneRepository(),
      deviceLocalZoneRepository: SpyDeviceLocalZoneRepository()
    )
  }

  @Test func body_renders_withZones() throws {
    let vm = makeViewModel(zones: [try makeZone(name: "Home"), try makeZone(name: "Work")])
    let sut = DeviceLocalZoneConversionView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whenEmpty() {
    let vm = makeViewModel(zones: [])
    let sut = DeviceLocalZoneConversionView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_whileConverting() async throws {
    let vm = makeViewModel(zones: [try makeZone()])
    await vm.convertAll()
    let sut = DeviceLocalZoneConversionView(viewModel: vm)
    _ = sut.body
  }
}
