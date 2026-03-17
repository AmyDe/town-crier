import TownCrierDomain

/// Stub implementation that returns a fixed minimum version.
/// Replace with an API-backed implementation when the version config endpoint is available.
public final class StubVersionConfigService: VersionConfigService {
    private let minimumVersion: AppVersion

    public init(minimumVersion: AppVersion = AppVersion(major: 1, minor: 0, patch: 0)) {
        self.minimumVersion = minimumVersion
    }

    public func fetchMinimumVersion() async throws -> AppVersion {
        minimumVersion
    }
}
