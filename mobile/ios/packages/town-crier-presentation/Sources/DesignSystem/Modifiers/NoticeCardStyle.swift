import SwiftUI

/// The Public Notice card treatment: `tcSurface` background, a 1px `tcBorder`
/// outline, and a 2pt top rule (`tcTextPrimary` normally, `tcAmber` when the
/// card represents unread content).
public struct NoticeCardStyle: ViewModifier {
  static let topRuleHeight: CGFloat = 2

  private let isUnread: Bool

  public init(isUnread: Bool = false) {
    self.isUnread = isUnread
  }

  public func body(content: Content) -> some View {
    content
      .background(Color.tcSurface)
      .overlay(alignment: .top) {
        Self.topRuleColor(isUnread: isUnread)
          .frame(height: Self.topRuleHeight)
      }
      .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
      .overlay(
        RoundedRectangle(cornerRadius: TCCornerRadius.medium)
          .strokeBorder(Color.tcBorder, lineWidth: 1)
      )
  }

  static func topRuleColor(isUnread: Bool) -> Color {
    isUnread ? .tcAmber : .tcTextPrimary
  }
}

extension View {
  /// Applies the Public Notice filed-notice card treatment: `tcSurface`
  /// background, a 1px `tcBorder` outline, and a 2pt top rule (`tcAmber`
  /// when `isUnread`, `tcTextPrimary` otherwise), clipped to
  /// `TCCornerRadius.medium`.
  public func noticeCardStyle(isUnread: Bool = false) -> some View {
    modifier(NoticeCardStyle(isUnread: isUnread))
  }
}
