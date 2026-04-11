/// Provides the app's version and build number for display.
public protocol AppVersionProvider: Sendable {
  var version: String { get }
  var buildNumber: String { get }
}
