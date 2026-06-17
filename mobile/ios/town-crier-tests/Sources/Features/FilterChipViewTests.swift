import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("FilterChipView")
@MainActor
struct FilterChipViewTests {

  @Test func init_selected_rendersWithoutCrashing() {
    let sut = FilterChipView(label: "All", isSelected: true) {}
    _ = sut.body
  }

  @Test func init_unselected_rendersWithoutCrashing() {
    let sut = FilterChipView(label: "Pending", isSelected: false) {}
    _ = sut.body
  }

  @Test func onTap_invokesClosure() {
    var tapped = false
    let sut = FilterChipView(label: "Refused", isSelected: false) { tapped = true }
    sut.onTap()
    #expect(tapped)
  }
}
