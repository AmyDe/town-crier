import SwiftUI

/// The Public Notice masthead (GH#857): a small-caps wordmark title set over
/// a double rule, echoing a printed notice's banner. Used on top-level
/// screen titles — Applications, Saved, and Watch Zones.
///
/// Rendered as ordinary scrollable content (not a toolbar principal item) so
/// it composes with the existing `List`-based screens without disturbing
/// their `.navigationTitle` (still present for VoiceOver/back-button
/// correctness). The system nav bar's own rendered title text is separately
/// suppressed on each of those screens via `View.mastheadNavigationBar()`
/// (GH#912 Phase 1), so this masthead row is the single VISIBLE title.
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
