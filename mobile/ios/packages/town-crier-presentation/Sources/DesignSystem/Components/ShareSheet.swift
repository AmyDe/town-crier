#if os(iOS)
  import SwiftUI
  import UIKit

  /// Thin SwiftUI wrapper around `UIActivityViewController` so a file (or any
  /// activity item) can be shared via the standard iOS share sheet — Save to
  /// Files, AirDrop, Mail, and so on. Used to hand the GDPR data-export `.json`
  /// file to the user from Settings, and the public link from application
  /// detail. `onCompleted` fires only when the user finishes a share, with the
  /// raw activity type.
  public struct ShareSheet: UIViewControllerRepresentable {
    private let activityItems: [Any]
    private let onCompleted: ((String?) -> Void)?

    public init(activityItems: [Any], onCompleted: ((String?) -> Void)? = nil) {
      self.activityItems = activityItems
      self.onCompleted = onCompleted
    }

    public func makeUIViewController(context: Context) -> UIActivityViewController {
      let controller = UIActivityViewController(
        activityItems: activityItems, applicationActivities: nil)
      if let onCompleted {
        controller.completionWithItemsHandler = { activityType, completed, _, _ in
          if completed { onCompleted(activityType?.rawValue) }
        }
      }
      return controller
    }

    public func updateUIViewController(
      _ uiViewController: UIActivityViewController,
      context: Context
    ) {}
  }
#endif
