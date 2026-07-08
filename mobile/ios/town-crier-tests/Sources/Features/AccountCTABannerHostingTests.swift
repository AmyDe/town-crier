import SwiftUI
import Testing

@testable import TownCrierPresentation

/// Regression coverage for the tab-shell CTA banner defect found by live
/// simulator verification (tc-hq9h7.3 fix): `.safeAreaInset(edge: .bottom)`
/// applied directly to a `TabView` insets against the *window's* bottom
/// edge, drawing the banner over the tab bar and swallowing taps on the
/// other tabs. The fix is `View.accountCTABanner(onCreateAccount:onSignIn:)`
/// — applied INSIDE each tab's content instead, mirroring the pre-Phase-3
/// `AnonymousMapView`, which hosted this exact banner inside its own content
/// and stacked correctly above the tab bar.
///
/// SwiftUI view trees aren't introspectable in this codebase (no
/// ViewInspector), so this is a construction/render smoke test — the
/// meaningful regression guard is architectural: `AnonymousMainTabView` has
/// no `.safeAreaInset` call on its `TabView` at all; every tab that wants
/// the banner applies this modifier to its own content.
@Suite("View.accountCTABanner")
@MainActor
struct AccountCTABannerHostingTests {
  @Test func body_renders_onArbitraryContent() {
    let sut = ProbeContent()

    _ = sut.body
  }
}

/// A minimal custom `View` applying the modifier under test. SwiftUI's
/// `.safeAreaInset` produces a primitive `ModifiedContent` whose own `.body`
/// traps if evaluated directly (it's rendered natively, not via a
/// user-defined `body`) — calling `.body` on a custom `View` struct that
/// merely *returns* that content, as every other body-render smoke test in
/// this suite does, is the safe pattern.
private struct ProbeContent: View {
  var body: some View {
    Text("tab content")
      .accountCTABanner(onCreateAccount: {}, onSignIn: {})
  }
}
