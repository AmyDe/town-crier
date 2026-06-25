import Foundation
import SwiftUI
import TownCrierPresentation

extension View {
  /// Opens the iOS system Settings notifications subpage when the coordinator
  /// raises ``AppCoordinator/isOpeningSystemNotificationSettings``, then resets
  /// the flag. Keeps the coordinator UIKit-free.
  func openingSystemNotificationSettings(when coordinator: AppCoordinator) -> some View {
    modifier(
      OpenURLOnFlagModifier(
        coordinator: coordinator,
        flag: \.isOpeningSystemNotificationSettings,
        urlString: AppCoordinator.systemNotificationSettingsURLString
      )
    )
  }

  /// Opens the App Store write-review composer when the coordinator raises
  /// ``AppCoordinator/isOpeningAppStoreReview`` (the "Rate the App" row), then
  /// resets the flag (GH #629).
  func openingAppStoreReview(when coordinator: AppCoordinator) -> some View {
    modifier(
      OpenURLOnFlagModifier(
        coordinator: coordinator,
        flag: \.isOpeningAppStoreReview,
        urlString: AppCoordinator.appStoreWriteReviewURLString
      )
    )
  }
}

/// Bridges a coordinator `Bool` flag to SwiftUI's environment `openURL` action:
/// when the flag flips to `true` the app opens `urlString` and resets the flag.
/// The coordinator only raises the flag; the un-testable `openURL` side effect
/// lives here, mirroring `ReviewPromptRequestModifier`.
private struct OpenURLOnFlagModifier: ViewModifier {
  @Environment(\.openURL) private var openURL
  @ObservedObject var coordinator: AppCoordinator
  let flag: ReferenceWritableKeyPath<AppCoordinator, Bool>
  let urlString: String

  func body(content: Content) -> some View {
    content.onChange(of: coordinator[keyPath: flag]) { _, requested in
      guard requested else { return }
      if let url = URL(string: urlString) {
        openURL(url)
      }
      coordinator[keyPath: flag] = false
    }
  }
}
