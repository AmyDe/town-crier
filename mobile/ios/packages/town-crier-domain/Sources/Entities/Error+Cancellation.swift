import Foundation

extension Error {
  /// True for `CancellationError` and `URLError(.cancelled)`, the two forms
  /// that structured-concurrency cancellation of a request can surface as.
  public var isCancellation: Bool {
    self is CancellationError || (self as? URLError)?.code == .cancelled
  }
}
