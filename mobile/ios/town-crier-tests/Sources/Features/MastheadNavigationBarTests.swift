import SwiftUI
import Testing

@testable import TownCrierPresentation

/// Regression coverage for `View.mastheadNavigationBar()` (GH#912 Phase 1):
/// Applications, Saved, and Watch Zones were each rendering both the system
/// nav-bar title and the `MastheadView` row, doubling the title.
///
/// SwiftUI view trees aren't introspectable in this codebase (no
/// ViewInspector — see `AccountCTABannerHostingTests`), so this is a
/// construction/render smoke test. The meaningful regression guard is
/// architectural: every masthead screen calls `.mastheadNavigationBar()`
/// alongside `.navigationTitle`, asserted directly on those views'
/// `body_renders` tests.
@Suite("View.mastheadNavigationBar")
@MainActor
struct MastheadNavigationBarTests {
  @Test func body_renders_onArbitraryContent() {
    let sut = ProbeContent()

    _ = sut.body
  }
}

private struct ProbeContent: View {
  var body: some View {
    Text("screen content")
      .navigationTitle("Probe")
      .mastheadNavigationBar()
  }
}
