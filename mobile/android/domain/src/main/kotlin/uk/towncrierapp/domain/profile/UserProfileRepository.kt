package uk.towncrierapp.domain.profile

/** Port for managing the user's server-side profile. */
public interface UserProfileRepository {
    /**
     * Ensures the server profile exists (creates it on first call, returns
     * the existing one otherwise — the server-side handler is idempotent)
     * and returns it. Called on every signed-in transition, before tier
     * resolution (tc-a6it / #549 ordering). NOTE: the wire response for this
     * endpoint carries only `userId`/`pushEnabled`/`tier` — the returned
     * [ServerProfile]'s notification-preference fields are the server's
     * defaults, not necessarily this user's actual saved values; use
     * [fetchProfile] to read the real preferences (#778).
     */
    public suspend fun ensureProfile(): ServerProfile

    /** Fetches the current profile (`GET /v1/me`), or `null` if none exists yet (404). */
    public suspend fun fetchProfile(): ServerProfile?

    /**
     * Replaces ALL FIVE notification preferences in one call (`PATCH /v1/me`)
     * — the server treats them as a set, so every setter round-trips every
     * field, never a partial update (epic #770 pre-resolved decision).
     */
    public suspend fun updatePreferences(preferences: UserPreferences): ServerProfile

    /**
     * UK GDPR Art. 17 erasure (`DELETE /v1/me`). The server cascades to
     * every per-user store and, last, deletes the Auth0 user — the client
     * NEVER calls Auth0's management API directly.
     */
    public suspend fun deleteAccount()

    /** The full GDPR data export (`GET /v1/me/data`) as opaque, unmodified bytes. */
    public suspend fun exportData(): ByteArray
}
