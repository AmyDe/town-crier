/// The short random code carried by a shared link as `?s=<code>`. It is drawn
/// from the system random source, with nothing derived from the device or user.
/// Never persist it or send it at share time.
public enum ShareLinkCode {
  private static let length = 6
  private static let alphabet = Array(
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

  /// Returns a new code.
  public static func generate() -> String {
    var generator = SystemRandomNumberGenerator()
    return String((0..<length).compactMap { _ in alphabet.randomElement(using: &generator) })
  }

  /// Returns `value` if it is exactly six ASCII letters or digits, otherwise `nil`.
  public static func validated(_ value: String) -> String? {
    value.count == length && value.allSatisfy(alphabet.contains) ? value : nil
  }
}
