import SwiftUI

/// The Public Notice "filed-notice" card treatment (GH#857): `tcSurface`
/// background, a 1px `tcBorder` outline, and a 2pt top rule — `tcTextPrimary`
/// normally, `tcAmber` when the card represents unread content (wired to the
/// existing per-application read-state signal, ADR 0035). Applied to the
/// application feed cards, zone cards, and detail containers.
public struct NoticeCardStyle: ViewModifier {
  /// 2pt — top rule thickness.
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

  /// The top rule's colour: `tcAmber` for unread content, `tcTextPrimary`
  /// otherwise. A pure function so the read/unread switch is testable
  /// without rendering SwiftUI.
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
