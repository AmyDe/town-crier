import SwiftUI
import Testing

@testable import TownCrierPresentation

/// Public Notice typography (GH#857): display roles move to Fraunces (static
/// Regular/SemiBold instances, registered at runtime by ``FontRegistrar``)
/// while keeping full Dynamic Type scaling via `relativeTo:`; body/caption
/// roles stay system sans. Two new monospace roles (`mono`/`monoEmphasis`)
/// style planning refs, dates, and distances.
@Suite("TCTypography")
struct TCTypographyTests {

  // MARK: - Fraunces display roles (Dynamic Type preserved via relativeTo:)

  @Test func displayLarge_isFrauncesSemiboldRelativeToLargeTitle() {
    #expect(
      TCTypography.displayLarge
        == Font.custom("Fraunces", size: 34, relativeTo: .largeTitle).weight(.semibold))
  }

  @Test func displaySmall_isFrauncesSemiboldRelativeToTitle2() {
    #expect(
      TCTypography.displaySmall
        == Font.custom("Fraunces", size: 22, relativeTo: .title2).weight(.semibold))
  }

  @Test func headline_isFrauncesSemiboldRelativeToHeadline() {
    #expect(
      TCTypography.headline
        == Font.custom("Fraunces", size: 17, relativeTo: .headline).weight(.semibold))
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
