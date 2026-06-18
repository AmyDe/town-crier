#if os(iOS)
  import SwiftUI
  import UIKit

  /// Thin SwiftUI wrapper around `UIActivityViewController` so a file (or any
  /// activity item) can be shared via the standard iOS share sheet — Save to
  /// Files, AirDrop, Mail, and so on. Used to hand the GDPR data-export `.json`
  /// file to the user from Settings.
  public struct ShareSheet: UIViewControllerRepresentable {
    private let activityItems: [Any]

    public init(activityItems: [Any]) {
      self.activityItems = activityItems
    }

    public func makeUIViewController(context: Context) -> UIActivityViewController {
      UIActivityViewController(activityItems: activityItems, applicationActivities: nil)
    }

    public func updateUIViewController(
      _ uiViewController: UIActivityViewController,
      context: Context
    ) {}
  }
#endif
