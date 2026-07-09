import SwiftUI
import Testing

@testable import TownCrierPresentation

/// ``MastheadView`` is the Public Notice masthead treatment (GH#857):
/// wordmark small-caps title over a double rule, used on top-level screen
/// titles — the Applications feed and Watch Zones screens definitely.
@Suite("MastheadView")
@MainActor
struct MastheadViewTests {

  @Test func title_isExposedVerbatim() {
    let sut = MastheadView(title: "Applications")

    #expect(sut.title == "Applications")
  }

  @Test func body_rendersWithoutCrashing() {
    _ = AnyView(MastheadView(title: "Applications"))
  }
}
