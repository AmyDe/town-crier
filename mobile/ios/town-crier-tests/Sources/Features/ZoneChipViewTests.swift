import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("ZoneChipView")
@MainActor
struct ZoneChipViewTests {

  @Test func init_selected_rendersWithoutCrashing() {
    let sut = ZoneChipView(label: "Mill Road", isSelected: true) {}
    _ = sut.body
  }

  @Test func init_unselected_rendersWithoutCrashing() {
    let sut = ZoneChipView(label: "City Centre", isSelected: false) {}
    _ = sut.body
  }

  @Test func onTap_invokesClosure() {
    var tapped = false
    let sut = ZoneChipView(label: "Trumpington", isSelected: false) { tapped = true }
    sut.onTap()
    #expect(tapped)
  }
}
