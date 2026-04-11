/// Fetches the minimum supported app version from the server.
public protocol VersionConfigService: Sendable {
  func fetchMinimumVersion() async throws -> AppVersion
}
