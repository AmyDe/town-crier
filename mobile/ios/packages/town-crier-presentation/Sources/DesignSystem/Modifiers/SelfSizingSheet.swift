import SwiftUI

/// Sizes the enclosing sheet to its content's natural height.
///
/// A fixed `.presentationDetents([.medium])` clips short, text-heavy sheets: when
/// the content's intrinsic height exceeds the medium detent the body text is
/// compressed and truncates (worse at larger Dynamic Type sizes). This modifier
/// measures the content at its ideal height and makes that the sole detent, so
/// the copy is always shown in full.
private struct SelfSizingSheetModifier: ViewModifier {
  @State private var contentHeight: CGFloat?

  func body(content: Content) -> some View {
    content
      // Force the ideal vertical height so text wraps fully instead of truncating,
      // then measure that height to drive the detent.
      .fixedSize(horizontal: false, vertical: true)
      .background {
        GeometryReader { proxy in
          Color.clear
            .onAppear { contentHeight = proxy.size.height }
            .onChange(of: proxy.size.height) { _, newValue in
              contentHeight = newValue
            }
        }
      }
      // Fall back to `.large` until measured so the first frame never truncates.
      .presentationDetents(contentHeight.map { measured in [.height(measured)] } ?? [.large])
      .presentationDragIndicator(.visible)
  }
}

extension View {
  /// Presents this view in a sheet sized to its own content height.
  ///
  /// Use in place of `.presentationDetents([.medium])` for short, text-heavy
  /// sheets where a fixed detent would clip and truncate the copy.
  func selfSizingSheet() -> some View {
    modifier(SelfSizingSheetModifier())
  }
}
