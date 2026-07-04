package uk.towncrierapp.domain.watchzones

/** Persists and retrieves the user's watch zones. Port of iOS `WatchZoneRepository`. */
public interface WatchZoneRepository {
    /** Returns every watch zone belonging to the current user. */
    public suspend fun zones(): List<WatchZone>

    /**
     * Creates [zone]. Throws
     * [uk.towncrierapp.domain.auth.DomainError.InsufficientEntitlement] when
     * the user has hit their tier's watch-zone quota.
     */
    public suspend fun create(zone: WatchZone)

    /** Updates [zone] in place — implementations send the full field set even though the server accepts a partial one. */
    public suspend fun update(zone: WatchZone)

    /** Deletes the zone identified by [id]. Idempotent: deleting an already-absent zone succeeds silently. */
    public suspend fun delete(id: WatchZoneId)
}
