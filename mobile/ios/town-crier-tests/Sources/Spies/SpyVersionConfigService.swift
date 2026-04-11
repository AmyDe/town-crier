import TownCrierDomain

final class SpyVersionConfigService: VersionConfigService, @unchecked Sendable {
  private(set) var fetchMinimumVersionCallCount = 0
  var fetchMinimumVersionResult: Result<AppVersion, Error> = .success(
    AppVersion(major: 1, minor: 0, patch: 0))

  func fetchMinimumVersion() async throws -> AppVersion {
    fetchMinimumVersionCallCount += 1
    return try fetchMinimumVersionResult.get()
  }
}
