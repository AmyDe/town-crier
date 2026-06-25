import StoreKit
import SwiftUI
import TownCrierPresentation

extension View {
  /// Performs the real `requestReview` OS call when the coordinator flags an
  /// engagement peak, then resets the flag (GH #628).
  ///
  /// This is the app-layer edge for the review prompt: the decision logic lives
  /// in the testable `ReviewPromptTracker`/`ReviewPromptPolicy`, and only the
  /// un-testable StoreKit call sits here — mirroring the established
  /// `isOpeningSystemNotificationSettings` → `openURL` pattern.
  func requestingReview(when coordinator: AppCoordinator) -> some View {
    modifier(ReviewPromptRequestModifier(coordinator: coordinator))
  }
}

/// Bridges ``AppCoordinator/isRequestingReview`` to SwiftUI's environment
/// `requestReview` action. Apple caps prompts (~3/year) and may show nothing;
/// we cannot detect display, so the flag is simply reset after the attempt.
private struct ReviewPromptRequestModifier: ViewModifier {
  @Environment(\.requestReview) private var requestReview
  @ObservedObject var coordinator: AppCoordinator

  func body(content: Content) -> some View {
    content.onChange(of: coordinator.isRequestingReview) { _, requested in
      guard requested else { return }
      requestReview()
      coordinator.isRequestingReview = false
    }
  }
}
