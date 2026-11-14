import SwiftUI

/// `.inline` plus an empty `.principal` toolbar item removes the rendered bar text
/// without touching `navigationItem.title`, which VoiceOver and the back button read.
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
