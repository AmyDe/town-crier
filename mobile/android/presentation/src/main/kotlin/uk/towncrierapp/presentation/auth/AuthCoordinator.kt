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
import uk.towncrierapp.domain.watchzones.WatchZoneRepository

/**
 * Owns the signed-in transition's ordering (#549): ensure the server profile
 * exists, THEN resolve the subscription tier, THEN persist it, THEN - and
 * only then (tc-7ttz) - check account state for the onboarding gate. Port of
 * iOS `AppCoordinator.resolveSubscriptionTier()` / `ServerTierResolver` /
 * `SubscriptionTierResolver` / `AppCoordinator+Onboarding`, degraded to the
 * 2-way tier merge this issue implements (`max(serverTier ?? max(cachedTier,
 * jwtTier), jwtTier)` — no store tier until #783).
 */
public class AuthCoordinator(
    private val authService: AuthenticationService,
    private val userProfileRepository: UserProfileRepository,
    private val tierCache: SubscriptionTierCache,
    private val watchZoneRepository: WatchZoneRepository,
) {
    private val _subscriptionTier = MutableStateFlow(SubscriptionTier.FREE)
    public val subscriptionTier: StateFlow<SubscriptionTier> = _subscriptionTier.asStateFlow()

    private val _onboardingPresentation = MutableStateFlow(OnboardingPresentation.Undetermined)

    /** Whether the onboarding wizard should be shown - `Undetermined` (render a loading screen) until [onSignedIn] has resolved account state. */
    public val onboardingPresentation: StateFlow<OnboardingPresentation> = _onboardingPresentation.asStateFlow()

    /**
     * Runs the full signed-in sequence: ensure profile → resolve tier →
     * persist → resolve the onboarding gate from account state. Call on
     * every signed-out→signed-in transition, including cold-start session
     * restore. The final step is the tc-k9fk/#549 fix ported to Android:
     * moving the watch-zone check any earlier in this function reproduces
     * that production bug.
     */
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

        _onboardingPresentation.value = resolveOnboardingPresentation()
    }

    /** Forces the gate to `NotRequired` once the wizard itself has finished (tc-7ttz) - completion is best-effort, so this doesn't re-check the server. */
    public fun onOnboardingCompleted() {
        _onboardingPresentation.value = OnboardingPresentation.NotRequired
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

    /**
     * Fails open to [OnboardingPresentation.NotRequired] on a transport
     * error - a transient failure checking zone count must never strand a
     * returning user on an indefinite loading screen, and must never trap
     * them in the wizard either (deliberate simplification: the device latch
     * is not consulted here, since no acceptance criterion depends on it -
     * see the bead's KEY DECISIONS).
     */
    @Suppress("SwallowedException")
    private suspend fun resolveOnboardingPresentation(): OnboardingPresentation =
        try {
            if (watchZoneRepository.zones().isEmpty()) {
                OnboardingPresentation.Required
            } else {
                OnboardingPresentation.NotRequired
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            OnboardingPresentation.NotRequired
        }
}
