package uk.towncrierapp.presentation.auth

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.auth.anAuthSession
import uk.towncrierapp.domain.profile.FakeUserProfileRepository
import uk.towncrierapp.domain.profile.aServerProfile
import uk.towncrierapp.domain.subscriptions.FakeSubscriptionTierCache
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * The signed-in transition: `POST /v1/me` (ensure-profile) BEFORE tier
 * resolution (#549 ordering), `max(serverTier ?? max(cachedTier, jwtTier),
 * jwtTier)` with a single refresh-and-retry when the first pass lands on
 * Free, and persistence to the `cachedSubscriptionTier` device latch. Port
 * of iOS `AppCoordinator.resolveSubscriptionTier()` / `SubscriptionTierResolver`,
 * degraded to the 2-way merge this issue implements (no store tier until #783).
 */
class AuthCoordinatorTest {
    @Test
    fun `onSignedIn calls ensure-profile and folds its tier into the resolved result`() =
        runTest {
            val authService = FakeAuthenticationService(currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE))
            val userProfileRepository =
                FakeUserProfileRepository(ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.PRO)))
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.FREE)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache)

            sut.onSignedIn()

            assertEquals(1, userProfileRepository.ensureProfileCalls.size)
            assertEquals(SubscriptionTier.PRO, sut.subscriptionTier.value)
        }

    @Test
    fun `an ensure-profile failure falls back to max of cached and jwt, never forced to Free`() =
        runTest {
            val authService = FakeAuthenticationService(currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE))
            val userProfileRepository = FakeUserProfileRepository(ensureProfileResult = Result.failure(DomainError.NetworkUnavailable))
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.PRO)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache)

            sut.onSignedIn()

            assertEquals(SubscriptionTier.PRO, sut.subscriptionTier.value)
        }

    @Test
    fun `landing on Free on the first pass refreshes the session exactly once and re-resolves`() =
        runTest {
            val authService =
                FakeAuthenticationService(currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE)).apply {
                    refreshSessionResult = Result.success(anAuthSession(subscriptionTier = SubscriptionTier.PERSONAL))
                }
            val userProfileRepository =
                FakeUserProfileRepository(ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.FREE)))
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.FREE)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache)

            sut.onSignedIn()

            assertEquals(1, authService.refreshSessionCalls.size)
            assertEquals(SubscriptionTier.PERSONAL, sut.subscriptionTier.value)
        }

    @Test
    fun `a resolution that is still Free after the retry stays Free, without a second refresh`() =
        runTest {
            val authService = FakeAuthenticationService(currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE)).apply {
                refreshSessionResult = Result.success(anAuthSession(subscriptionTier = SubscriptionTier.FREE))
            }
            val userProfileRepository =
                FakeUserProfileRepository(ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.FREE)))
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.FREE)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache)

            sut.onSignedIn()

            assertEquals(SubscriptionTier.FREE, sut.subscriptionTier.value)
            assertEquals(1, authService.refreshSessionCalls.size)
        }

    @Test
    fun `the resolved tier is persisted to the subscription tier cache`() =
        runTest {
            val authService = FakeAuthenticationService(currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.PRO))
            val userProfileRepository = FakeUserProfileRepository(ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.PRO)))
            val tierCache = FakeSubscriptionTierCache()
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache)

            sut.onSignedIn()

            assertEquals(listOf(SubscriptionTier.PRO), tierCache.writeCalls)
        }
}
