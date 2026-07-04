package uk.towncrierapp.domain.profile

/**
 * Port for managing the user's server-side profile. This issue implements
 * only the ensure-profile seam (`POST /v1/me`, no body, idempotent); fetch/
 * update/delete/export land with settings work in #778.
 */
public interface UserProfileRepository {
    /**
     * Ensures the server profile exists (creates it on first call, returns
     * the existing one otherwise — the server-side handler is idempotent)
     * and returns it. Called on every signed-in transition, before tier
     * resolution (tc-a6it / #549 ordering).
     */
    public suspend fun ensureProfile(): ServerProfile
}
