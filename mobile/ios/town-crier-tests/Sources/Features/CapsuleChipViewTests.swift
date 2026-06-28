import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("CapsuleChipView")
@MainActor
struct CapsuleChipViewTests {

  // Labels cover both call-site styles the chip replaced: status-filter chips
  // (e.g. "All", "Pending", "Refused") and watch-zone picker chips (e.g.
  // "Mill Road", "City Centre", "Trumpington").

  @Test(arguments: ["All", "Mill Road"])
  func init_selected_rendersWithoutCrashing(label: String) {
    let sut = CapsuleChipView(label: label, isSelected: true) {}
    _ = sut.body
  }

  @Test(arguments: ["Pending", "City Centre"])
  func init_unselected_rendersWithoutCrashing(label: String) {
    let sut = CapsuleChipView(label: label, isSelected: false) {}
    _ = sut.body
  }

  @Test(arguments: ["Refused", "Trumpington"])
  func onTap_invokesClosure(label: String) {
    var tapped = false
    let sut = CapsuleChipView(label: label, isSelected: false) { tapped = true }
    sut.onTap()
    #expect(tapped)
  }
}
