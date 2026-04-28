import Foundation

extension Data {
  /// Encodes the data as a lowercased hex string, suitable for transmitting
  /// an APNs device token to the backend.
  func hexEncodedString() -> String {
    map { String(format: "%02x", $0) }.joined()
  }
}
