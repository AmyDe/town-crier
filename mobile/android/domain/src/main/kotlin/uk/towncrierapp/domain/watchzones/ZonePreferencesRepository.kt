package uk.towncrierapp.domain.watchzones

/**
 * Fetches and updates per-zone notification preferences. Maps to
 * `GET/PUT /v1/me/watch-zones/{zoneId}/preferences`. Port of iOS
 * `ZonePreferencesRepository`.
 */
public interface ZonePreferencesRepository {
    public suspend fun fetchPreferences(zoneId: WatchZoneId): ZoneNotificationPreferences

    public suspend fun updatePreferences(preferences: ZoneNotificationPreferences)
}
