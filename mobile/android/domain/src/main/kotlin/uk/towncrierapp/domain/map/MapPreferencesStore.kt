package uk.towncrierapp.domain.map

import uk.towncrierapp.domain.watchzones.WatchZoneId

/**
 * The Map tab's one device latch: `lastSelectedZone.map` (epic #770 iOS-key
 * parity — same key name as iOS `UserDefaults`, distinct from the
 * applications list's own `lastSelectedZone.applications`). A one-shot
 * suspend read/write, not a `Flow` — this is the cold-start restore value,
 * not something the UI observes live (mirrors
 * [uk.towncrierapp.domain.applications.ApplicationListPreferencesStore]'s shape).
 */
public interface MapPreferencesStore {
    public suspend fun readLastSelectedZoneId(): WatchZoneId?

    public suspend fun writeLastSelectedZoneId(zoneId: WatchZoneId)
}
