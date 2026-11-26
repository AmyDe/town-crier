/// Writes text to the system clipboard.
public protocol ClipboardWriter: Sendable {
  /// Replaces the clipboard contents with `text`.
  func copy(_ text: String)
}
