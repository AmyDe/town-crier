package uk.towncrierapp.domain.versionconfig

public class FakeVersionConfigService(
    public var fetchMinimumVersionResult: Result<AppVersion> = Result.success(AppVersion(1, 0, 0)),
) : VersionConfigService {
    public val fetchMinimumVersionCalls: MutableList<Unit> = mutableListOf()

    override suspend fun fetchMinimumVersion(): AppVersion {
        fetchMinimumVersionCalls += Unit
        return fetchMinimumVersionResult.getOrThrow()
    }
}
