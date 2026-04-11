import Foundation

/// Semantic version for comparing app versions (major.minor.patch).
public struct AppVersion: Equatable, Comparable, Sendable {
  public let major: Int
  public let minor: Int
  public let patch: Int

  public init(major: Int, minor: Int, patch: Int) {
    self.major = major
    self.minor = minor
    self.patch = patch
  }

  /// Parse a version string like "1.2.3". Returns nil if the format is invalid.
  public init?(_ string: String) {
    let parts = string.split(separator: ".").compactMap { Int($0) }
    guard parts.count == 3 else { return nil }
    self.major = parts[0]
    self.minor = parts[1]
    self.patch = parts[2]
  }

  public static func < (lhs: AppVersion, rhs: AppVersion) -> Bool {
    if lhs.major != rhs.major { return lhs.major < rhs.major }
    if lhs.minor != rhs.minor { return lhs.minor < rhs.minor }
    return lhs.patch < rhs.patch
  }
}
