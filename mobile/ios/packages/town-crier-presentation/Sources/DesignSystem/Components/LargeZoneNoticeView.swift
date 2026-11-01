import SwiftUI

/// One-line, non-dismissible notice shown under the zone selector on the list
/// and map when the selected zone is large.
public struct LargeZoneNoticeView: View {
  public init() {}

  public var body: some View {
    HStack(alignment: .firstTextBaseline, spacing: TCSpacing.small) {
      Image(systemName: "exclamationmark.triangle.fill")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcAmber)
        .accessibilityHidden(true)

      Text(LargeZoneCopy.notice)
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
        .fixedSize(horizontal: false, vertical: true)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .accessibilityElement(children: .combine)
  }
}
