import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("UpgradeBadgeView")
@MainActor
struct UpgradeBadgeViewTests {

  @Test func init_rendersWithoutCrashing() {
    let sut = UpgradeBadgeView()
    _ = sut.body
  }

  @Test func init_withCustomLabel_rendersWithoutCrashing() {
    let sut = UpgradeBadgeView(label: "PRO")
    _ = sut.body
  }
}
