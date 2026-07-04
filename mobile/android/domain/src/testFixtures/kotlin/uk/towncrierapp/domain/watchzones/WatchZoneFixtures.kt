package uk.towncrierapp.domain.watchzones

/** Fixture factory for [Coordinate] — Cambridge city centre, matching the iOS test fixture. */
public fun aCoordinate(
    latitude: Double = 52.2053,
    longitude: Double = 0.1218,
): Coordinate = Coordinate(latitude, longitude)

/** Fixture factory for [WatchZone] — override only what a test cares about. */
public fun aWatchZone(
    id: WatchZoneId = WatchZoneId("wz-1"),
    name: String = "Home",
    centre: Coordinate = aCoordinate(),
    radiusMetres: Double = 500.0,
    authorityId: Int = 0,
    pushEnabled: Boolean = true,
    emailInstantEnabled: Boolean = true,
): WatchZone =
    WatchZone(
        id = id,
        name = name,
        centre = centre,
        radiusMetres = radiusMetres,
        authorityId = authorityId,
        pushEnabled = pushEnabled,
        emailInstantEnabled = emailInstantEnabled,
    )

/** Fixture factory for [ZoneNotificationPreferences]. */
public fun aZoneNotificationPreferences(
    zoneId: WatchZoneId = WatchZoneId("wz-1"),
    newApplicationPush: Boolean = true,
    newApplicationEmail: Boolean = true,
    decisionPush: Boolean = true,
    decisionEmail: Boolean = true,
): ZoneNotificationPreferences =
    ZoneNotificationPreferences(
        zoneId = zoneId,
        newApplicationPush = newApplicationPush,
        newApplicationEmail = newApplicationEmail,
        decisionPush = decisionPush,
        decisionEmail = decisionEmail,
    )
