import SwiftUI
import Testing

@testable import TownCrierPresentation

/// ``NoticeCardStyle`` is the Public Notice filed-notice card treatment
/// (GH#857): `tcSurface` background, 1px `tcBorder` outline, and a 2pt top
/// rule that switches to `tcAmber` for unread content — wired to the
/// existing per-application read-state signal (ADR 0035).
@Suite("NoticeCardStyle")
@MainActor
struct NoticeCardStyleTests {

  @Test func topRuleColor_isTextPrimary_whenRead() {
    #expect(NoticeCardStyle.topRuleColor(isUnread: false) == Color.tcTextPrimary)
  }

  @Test func topRuleColor_isAmber_whenUnread() {
    #expect(NoticeCardStyle.topRuleColor(isUnread: true) == Color.tcAmber)
  }

  // `.modifier()` returns `ModifiedContent`, a SwiftUI-primitive View whose
  // `body` traps if accessed outside a render pass (mirrors the established
  // pattern in EntitlementGateModifierTests) — wrapping in `AnyView` proves
  // the modifier compiles and applies without invoking that trap.

  @Test func viewModifier_appliesWithoutCrashing() {
    _ = AnyView(Text("Preview").noticeCardStyle())
  }

  @Test func viewModifier_appliesWithUnreadFlagWithoutCrashing() {
    _ = AnyView(Text("Preview").noticeCardStyle(isUnread: true))
  }
}
