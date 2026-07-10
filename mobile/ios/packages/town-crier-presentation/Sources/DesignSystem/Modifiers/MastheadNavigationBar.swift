import SwiftUI

/// Suppresses the system navigation bar's own rendered title text on
/// masthead screens (Applications, Saved, Watch Zones — GH#912 Phase 1),
/// while keeping `.navigationTitle` wired so VoiceOver still announces the
/// screen and any screen pushed on top still gets a correctly labelled back
/// button.
///
/// `MastheadView`'s doc comment records the original intent: the masthead
/// row is the sole *visible* title, with the system title kept only for
/// VoiceOver/back-button correctness. That suppression was never wired up,
/// so Applications and Watch Zones rendered both titles and Saved never grew
/// a masthead at all. `.inline` plus an empty `.principal` toolbar item
/// removes the rendered bar text without touching the underlying
/// `navigationItem.title` that VoiceOver and the back button read from.
private struct MastheadNavigationBarModifier: ViewModifier {
  func body(content: Content) -> some View {
    content
      #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
          ToolbarItem(placement: .principal) {
            Color.clear.frame(width: 0, height: 0)
          }
        }
      #endif
  }
}

extension View {
  /// Applies the masthead-screen navigation bar treatment: `.navigationTitle`
  /// stays set for accessibility and back-button labelling, but the system's
  /// own rendered title text is suppressed so the `MastheadView` row in the
  /// list is the single visible title.
  public func mastheadNavigationBar() -> some View {
    modifier(MastheadNavigationBarModifier())
  }
}
