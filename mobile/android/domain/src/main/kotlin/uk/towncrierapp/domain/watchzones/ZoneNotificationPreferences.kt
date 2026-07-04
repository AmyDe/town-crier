package uk.towncrierapp.domain.watchzones

/**
 * Per-zone notification preferences controlling which alert channels the
 * user receives for a specific watch zone. Wire shape matches
 * `GET/PUT /v1/me/watch-zones/{zoneId}/preferences`. All four per-channel
 * toggles default to `true` so newly-created zones opt in to every alert;
 * free-tier downgrades are applied at dispatch time on the server. Port of
 * iOS `ZoneNotificationPreferences`.
 */
public data class ZoneNotificationPreferences(
    public val zoneId: WatchZoneId,
    public val newApplicationPush: Boolean = true,
    public val newApplicationEmail: Boolean = true,
    public val decisionPush: Boolean = true,
    public val decisionEmail: Boolean = true,
)
