package uk.towncrierapp.presentation.auth

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.subscriptions.SubscriptionTierCache
import uk.towncrierapp.domain.subscriptions.resolveTier

/**
 * Owns the signed-in transition's ordering (#549): ensure the server profile
 * exists, THEN resolve the subscription tier, THEN persist it — before any
 * downstream gate (onboarding, in #773) runs. Port of iOS
 * `AppCoordinator.resolveSubscriptionTier()` / `ServerTierResolver` /
 * `SubscriptionTierResolver`, degraded to the 2-way merge this issue
 * implements (`max(serverTier ?? max(cachedTier, jwtTier), jwtTier)` — no
 * store tier until #783).
 */
public class AuthCoordinator(
    private val authService: AuthenticationService,
    private val userProfileRepository: UserProfileRepository,
    private val tierCache: SubscriptionTierCache,
) {
    private val _subscriptionTier = MutableStateFlow(SubscriptionTier.FREE)
    public val subscriptionTier: StateFlow<SubscriptionTier> = _subscriptionTier.asStateFlow()

    /** Runs the full signed-in sequence: ensure profile → resolve tier → persist. Call on every signed-out→signed-in transition, including cold-start session restore. */
    @Suppress("SwallowedException")
    // Both catches below are deliberate degrade paths (epic #770): a refresh
    // failure keeps the pre-refresh jwtTier; an ensure-profile failure yields
    // "server tier unknown" to resolveTier — neither cares WHY, just THAT.
    public suspend fun onSignedIn() {
        val cachedTier = tierCache.read() ?: SubscriptionTier.FREE
        val jwtTier = authService.currentSession()?.subscriptionTier ?: SubscriptionTier.FREE

        var resolved = resolveTier(ensureProfileTier(), cachedTier, jwtTier)

        if (resolved == SubscriptionTier.FREE) {
            // Winner is Free — the JWT tier claim may be stale (e.g. right after
            // an upgrade). Refresh once and re-resolve; never loop beyond this.
            val refreshedJwtTier =
                try {
                    authService.refreshSession().subscriptionTier
                } catch (e: CancellationException) {
                    throw e
                } catch (e: DomainError) {
                    jwtTier
                }
            resolved = resolveTier(ensureProfileTier(), cachedTier, refreshedJwtTier)
        }

        _subscriptionTier.value = resolved
        tierCache.write(resolved)
    }

    /** `null` on failure — [resolveTier] treats that as "server tier unknown", falling back to `max(cachedTier, jwtTier)` rather than Free. */
    @Suppress("SwallowedException")
    private suspend fun ensureProfileTier(): SubscriptionTier? =
        try {
            userProfileRepository.ensureProfile().tier
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            null
        }
}
