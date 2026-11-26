import SwiftUI

#if canImport(UIKit)
  import UIKit
#else
  import AppKit
#endif

enum SecretTextLayout {
  // A secret can contain real hyphens, so a hyphen added at a line break reads as part of it.
  static var paragraphStyle: NSParagraphStyle {
    let style = NSMutableParagraphStyle()
    style.lineBreakMode = .byCharWrapping
    style.hyphenationFactor = 0
    style.lineBreakStrategy = []
    return style
  }
}

#if canImport(UIKit)
  // SwiftUI's `Text` hyphenates long unbroken strings and ignores paragraph styles.
  // Display-only break characters would end up in a manual copy.
  struct SecretText: UIViewRepresentable {
    let secret: String

    func makeUIView(context: Context) -> UITextView {
      let textView = UITextView()
      textView.isEditable = false
      textView.isSelectable = true
      textView.isScrollEnabled = false
      textView.backgroundColor = .clear
      textView.textContainerInset = .zero
      textView.textContainer.lineFragmentPadding = 0
      textView.adjustsFontForContentSizeCategory = true
      textView.dataDetectorTypes = []
      textView.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
      return textView
    }

    func updateUIView(_ textView: UITextView, context: Context) {
      textView.attributedText = NSAttributedString(
        string: secret,
        attributes: [
          .font: Self.font,
          .foregroundColor: UIColor(Color.tcTextPrimary),
          .paragraphStyle: SecretTextLayout.paragraphStyle,
        ])
      textView.accessibilityLabel = secret
      textView.accessibilityValue = ""
    }

    func sizeThatFits(
      _ proposal: ProposedViewSize, uiView: UITextView, context: Context
    ) -> CGSize? {
      guard let width = proposal.width, width.isFinite else { return nil }
      let fitted = uiView.sizeThatFits(CGSize(width: width, height: .greatestFiniteMagnitude))
      return CGSize(width: width, height: fitted.height)
    }

    private static var font: UIFont {
      let preferred = UIFontDescriptor.preferredFontDescriptor(withTextStyle: .caption1)
      let monospaced = preferred.withDesign(.monospaced) ?? preferred
      return UIFont(descriptor: monospaced, size: 0)
    }
  }
#else
  struct SecretText: View {
    let secret: String

    var body: some View {
      Text(secret)
        .font(TCTypography.mono)
        .foregroundStyle(Color.tcTextPrimary)
        .textSelection(.enabled)
        .fixedSize(horizontal: false, vertical: true)
    }
  }
#endif
