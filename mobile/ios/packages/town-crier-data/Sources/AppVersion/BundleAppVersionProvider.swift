import Foundation
import TownCrierDomain

/// Reads app version and build number from the main bundle.
public final class BundleAppVersionProvider: AppVersionProvider, @unchecked Sendable {
    public var version: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "Unknown"
    }

    public var buildNumber: String {
        Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "0"
    }

    public init() {}
}
