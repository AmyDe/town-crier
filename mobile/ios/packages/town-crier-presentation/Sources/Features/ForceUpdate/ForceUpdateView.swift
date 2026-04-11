import SwiftUI

/// Blocking full-screen modal shown when the app version is below
/// the server-defined minimum. Cannot be dismissed by the user.
public struct ForceUpdateView: View {
  @Environment(\.openURL) private var openURL

  private let appStoreURL: URL?

  public init(appStoreURL: URL? = nil) {
    self.appStoreURL = appStoreURL
  }

  public var body: some View {
    VStack(spacing: TCSpacing.large) {
      Spacer()

      Image(systemName: "arrow.up.circle")
        .font(.system(.largeTitle))
        .foregroundStyle(Color.tcAmber)

      Text("Update Required")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text("A newer version of Town Crier is available. Please update to continue using the app.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      if let appStoreURL {
        PrimaryButton("Update Now") {
          openURL(appStoreURL)
        }
      }

      Spacer()
    }
    .padding(TCSpacing.extraLarge)
    .background(Color.tcBackground.ignoresSafeArea())
    .interactiveDismissDisabled()
  }
}
