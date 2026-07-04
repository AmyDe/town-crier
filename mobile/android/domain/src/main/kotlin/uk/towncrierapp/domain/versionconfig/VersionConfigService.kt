package uk.towncrierapp.domain.versionconfig

/** Fetches the minimum supported app version from the server (`GET /v1/version-config`, anonymous). */
public interface VersionConfigService {
    public suspend fun fetchMinimumVersion(): AppVersion
}
