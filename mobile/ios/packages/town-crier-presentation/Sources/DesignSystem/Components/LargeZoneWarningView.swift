import SwiftUI

/// User-facing copy for the large-zone warning and loading message.
public enum LargeZoneCopy {
  public static let warningTitle = "Large zones can be slower to browse"
  public static let warningSpeedParagraph =
    "Your notifications work as normal, but the map and list can take a few seconds to load, "
    + "especially in cities."
  public static let warningAdviceParagraph =
    "Just want to keep an eye on your street? Stick to 100-500 m in built-up areas, "
    + "and under 2 km everywhere else."
  public static let loadingMessage = "Loading a large zone. This can take a few seconds."
}

/// Callout shown on onboarding, create and edit when the zone is large under
/// ``LargeZone``. The rule is strict (`>` a 2 km circle), so a free user at the
/// 2 km cap in ``WatchZoneLimits`` never sees it.
public struct LargeZoneWarningView: View {
  public init() {}

  public var body: some View {
    HStack(alignment: .top, spacing: TCSpacing.small) {
      Image(systemName: "exclamationmark.triangle.fill")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcAmber)
        .accessibilityHidden(true)

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(LargeZoneCopy.warningTitle)
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Text(LargeZoneCopy.warningSpeedParagraph)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .fixedSize(horizontal: false, vertical: true)

        Text(LargeZoneCopy.warningAdviceParagraph)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .fixedSize(horizontal: false, vertical: true)
      }
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcAmberMuted)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
    .accessibilityElement(children: .combine)
    .accessibilityLabel(
      "\(LargeZoneCopy.warningTitle). \(LargeZoneCopy.warningSpeedParagraph) "
        + "Just want to keep an eye on your street? Stick to 100 to 500 metres in built-up areas, "
        + "and under 2 kilometres everywhere else."
    )
  }
}
