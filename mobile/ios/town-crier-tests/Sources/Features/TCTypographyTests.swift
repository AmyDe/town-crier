import SwiftUI
import Testing

@testable import TownCrierPresentation

/// Sans-serif typography (GH#912 Phase 5): the display roles that briefly
/// moved to Fraunces (Public Notice, GH#857) are back on the system sans,
/// standardised across every surface after tester feedback — the display-
/// serif treatment gave up a little of its "old-timey" character but tests
/// found the clarity/consistency worth it (owner-approved 2026-07-10). Each
/// role keeps the exact same base size the Fraunces role had (34/22/17pt,
/// matching `.largeTitle`/`.title2`/`.headline`'s own Dynamic Type defaults)
/// and the same `.semibold` weight, built on the underlying text style so
/// scaling is native, not a manual `relativeTo:` — SwiftUI's `Font.system`
/// has no `size:relativeTo:` overload (only `Font.custom` does), so the
/// idiomatic sans equivalent is the bare text style with a weight override,
/// mirroring how `body`/`caption` below are already built. body/caption
/// roles stay system sans (they always were). The monospace roles
/// (`mono`/`monoEmphasis`) style planning refs, dates, and distances and are
/// unaffected.
@Suite("TCTypography")
struct TCTypographyTests {

  // MARK: - Sans display roles (same sizes/weights as the Fraunces era)

  @Test func displayLarge_isSystemLargeTitleSemibold() {
    #expect(TCTypography.displayLarge == Font.system(.largeTitle).weight(.semibold))
  }

  @Test func displaySmall_isSystemTitle2Semibold() {
    #expect(TCTypography.displaySmall == Font.system(.title2).weight(.semibold))
  }

  @Test func headline_isSystemHeadlineSemibold() {
    #expect(TCTypography.headline == Font.system(.headline).weight(.semibold))
  }

  // MARK: - System sans roles (unchanged)

  @Test func body_isSystemBody() {
    #expect(TCTypography.body == Font.system(.body))
  }

  @Test func bodyEmphasis_isSystemBodySemibold() {
    #expect(TCTypography.bodyEmphasis == Font.system(.body, weight: .semibold))
  }

  @Test func caption_isSystemCaption() {
    #expect(TCTypography.caption == Font.system(.caption))
  }

  @Test func captionEmphasis_isSystemCaptionMedium() {
    #expect(TCTypography.captionEmphasis == Font.system(.caption, weight: .medium))
  }

  // MARK: - New monospace metadata roles

  @Test func mono_isSystemCaptionMonospaced() {
    #expect(TCTypography.mono == Font.system(.caption, design: .monospaced))
  }

  @Test func monoEmphasis_isSystemCaptionMediumMonospaced() {
    #expect(
      TCTypography.monoEmphasis
        == Font.system(.caption, design: .monospaced).weight(.medium))
  }
}
