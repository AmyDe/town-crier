import TownCrierDomain

final class SpyAppVersionProvider: AppVersionProvider, @unchecked Sendable {
  var version: String = "1.0.0"
  var buildNumber: String = "42"
}
