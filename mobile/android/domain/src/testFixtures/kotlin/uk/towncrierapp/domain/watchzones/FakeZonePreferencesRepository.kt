package uk.towncrierapp.domain.watchzones

import uk.towncrierapp.domain.auth.DomainError

/** Hand-written fake for [ZonePreferencesRepository]. */
public class FakeZonePreferencesRepository(
    public var fetchPreferencesResult: Result<ZoneNotificationPreferences> =
        Result.success(
            aZoneNotificationPreferences(),
        ),
) : ZonePreferencesRepository {
    public var updatePreferencesFailWith: DomainError? = null

    public val fetchPreferencesCalls: MutableList<WatchZoneId> = mutableListOf()
    public val updatePreferencesCalls: MutableList<ZoneNotificationPreferences> = mutableListOf()

    override suspend fun fetchPreferences(zoneId: WatchZoneId): ZoneNotificationPreferences {
        fetchPreferencesCalls += zoneId
        return fetchPreferencesResult.getOrThrow()
    }

    override suspend fun updatePreferences(preferences: ZoneNotificationPreferences) {
        updatePreferencesCalls += preferences
        updatePreferencesFailWith?.let { throw it }
    }
}
