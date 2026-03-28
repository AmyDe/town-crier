import Combine
import TownCrierDomain

/// ViewModel that checks whether the current app version meets the minimum
/// required version. If not, exposes state for a blocking update modal.
@MainActor
public final class ForceUpdateViewModel: ObservableObject {
    @Published public private(set) var requiresUpdate = false
    @Published public private(set) var isChecking = false

    private let versionConfigService: VersionConfigService
    private let appVersionProvider: AppVersionProvider

    public init(
        versionConfigService: VersionConfigService,
        appVersionProvider: AppVersionProvider
    ) {
        self.versionConfigService = versionConfigService
        self.appVersionProvider = appVersionProvider
    }

    public func checkVersion() async {
        isChecking = true
        defer {
            isChecking = false
        }

        guard let currentVersion = AppVersion(appVersionProvider.version) else {
            return
        }

        do {
            let minimumVersion = try await versionConfigService.fetchMinimumVersion()
            requiresUpdate = currentVersion < minimumVersion
        } catch {
            // On failure, allow the app to continue — don't block users
        }
    }
}
