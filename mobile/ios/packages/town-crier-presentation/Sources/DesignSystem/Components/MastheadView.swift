import SwiftUI

/// The Public Notice masthead (GH#857): a small-caps wordmark title set over
/// a double rule, echoing a printed notice's banner. Used on top-level
/// screen titles — the Applications feed and Watch Zones screens
/// definitely; other screens adopt it on a case-by-case basis.
///
/// Rendered as ordinary scrollable content (not a toolbar principal item) so
/// it composes with the existing `List`-based screens without disturbing
/// their `.navigationTitle` (still present for VoiceOver/back-button
/// correctness) or, on the Applications screen, the large-title collapse
/// animation that a single unambiguous scroll view depends on.
public struct MastheadView: View {
  public let title: String

  public init(title: String) {
    self.title = title
  }

  public var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
      Text(title)
        .font(TCTypography.headline)
        .textCase(.uppercase)
        .kerning(2)
        .foregroundStyle(Color.tcTextPrimary)

      VStack(spacing: 2) {
        Rectangle()
          .fill(Color.tcTextPrimary)
          .frame(height: 2)
        Rectangle()
          .fill(Color.tcTextPrimary)
          .frame(height: 1)
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .accessibilityElement(children: .ignore)
    .accessibilityLabel(title)
    .accessibilityAddTraits(.isHeader)
  }
}
