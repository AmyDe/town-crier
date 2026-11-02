import SwiftUI

/// Slow-load message shown beside the spinner while a large zone loads.
public struct LargeZoneLoadingMessageView: View {
  public init() {}

  public var body: some View {
    Text(LargeZoneCopy.loadingMessage)
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcTextSecondary)
      .multilineTextAlignment(.center)
      .fixedSize(horizontal: false, vertical: true)
  }
}
