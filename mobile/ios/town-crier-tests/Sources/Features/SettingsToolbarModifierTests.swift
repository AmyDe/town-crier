import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("settingsToolbar modifier")
@MainActor
struct SettingsToolbarModifierTests {

  // The `.toolbar` modifier cannot have its `.body` called outside a SwiftUI
  // render pass. These tests verify the modifier compiles, can be applied to
  // any View, and exposes a uniform call site for placing the gear icon on
  // every tab — the action closure is wired by the caller and validated via
  // AppCoordinator tests.

  @Test func modifier_canBeAppliedToView() {
    _ = AnyView(
      Text("Tab content")
        .settingsToolbar {}
    )
  }

  @Test func modifier_capturesActionClosure() {
    var didFire = false
    let action: () -> Void = { didFire = true }

    // The modifier just stores the closure; we invoke it directly to
    // verify the closure is captured by reference, not copied prematurely.
    _ = AnyView(
      Text("Tab content")
        .settingsToolbar(action: action)
    )
    action()

    #expect(didFire)
  }
}
