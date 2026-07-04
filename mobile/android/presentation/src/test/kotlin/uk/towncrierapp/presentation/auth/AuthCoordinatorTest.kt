package uk.towncrierapp.presentation.auth

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.auth.anAuthSession
import uk.towncrierapp.domain.profile.FakeUserProfileRepository
import uk.towncrierapp.domain.profile.ServerProfile
import uk.towncrierapp.domain.profile.UserPreferences
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.profile.aServerProfile
import uk.towncrierapp.domain.subscriptions.FakeSubscriptionTierCache
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneRepository
import uk.towncrierapp.domain.watchzones.aWatchZone
import kotlin.test.assertEquals

/**
 * The signed-in transition: `POST /v1/me` (ensure-profile) BEFORE tier
 * resolution (#549 ordering), `max(serverTier ?? max(cachedTier, jwtTier),
 * jwtTier)` with a single refresh-and-retry when the first pass lands on
 * Free, persistence to the `cachedSubscriptionTier` device latch, and -
 * strictly after all of that (tc-7ttz) - the onboarding account-state gate:
 * zero watch zones means the wizard is required. Port of iOS
 * `AppCoordinator.resolveSubscriptionTier()` / `SubscriptionTierResolver` /
 * `AppCoordinator+Onboarding`, degraded to the 2-way tier merge this issue
 * implements (no store tier until #783).
 */
class AuthCoordinatorTest {
    @Test
    fun `onSignedIn calls ensure-profile and folds its tier into the resolved result`() =
        runTest {
            val authService =
                FakeAuthenticationService(
                    currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE),
                )
            val userProfileRepository =
                FakeUserProfileRepository(
                    ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.PRO)),
                )
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.FREE)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache, FakeWatchZoneRepository())

            sut.onSignedIn()

            assertEquals(1, userProfileRepository.ensureProfileCalls.size)
            assertEquals(SubscriptionTier.PRO, sut.subscriptionTier.value)
        }

    @Test
    fun `an ensure-profile failure falls back to max of cached and jwt, never forced to Free`() =
        runTest {
            val authService =
                FakeAuthenticationService(
                    currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE),
                )
            val userProfileRepository =
                FakeUserProfileRepository(ensureProfileResult = Result.failure(DomainError.NetworkUnavailable))
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.PRO)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache, FakeWatchZoneRepository())

            sut.onSignedIn()

            assertEquals(SubscriptionTier.PRO, sut.subscriptionTier.value)
        }

    @Test
    fun `landing on Free on the first pass refreshes the session exactly once and re-resolves`() =
        runTest {
            val authService =
                FakeAuthenticationService(
                    currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE),
                ).apply {
                    refreshSessionResult = Result.success(anAuthSession(subscriptionTier = SubscriptionTier.PERSONAL))
                }
            val userProfileRepository =
                FakeUserProfileRepository(
                    ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.FREE)),
                )
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.FREE)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache, FakeWatchZoneRepository())

            sut.onSignedIn()

            assertEquals(1, authService.refreshSessionCalls.size)
            assertEquals(SubscriptionTier.PERSONAL, sut.subscriptionTier.value)
        }

    @Test
    fun `a resolution that is still Free after the retry stays Free, without a second refresh`() =
        runTest {
            val authService =
                FakeAuthenticationService(
                    currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.FREE),
                ).apply {
                    refreshSessionResult = Result.success(anAuthSession(subscriptionTier = SubscriptionTier.FREE))
                }
            val userProfileRepository =
                FakeUserProfileRepository(
                    ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.FREE)),
                )
            val tierCache = FakeSubscriptionTierCache(stored = SubscriptionTier.FREE)
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache, FakeWatchZoneRepository())

            sut.onSignedIn()

            assertEquals(SubscriptionTier.FREE, sut.subscriptionTier.value)
            assertEquals(1, authService.refreshSessionCalls.size)
        }

    @Test
    fun `the resolved tier is persisted to the subscription tier cache`() =
        runTest {
            val authService =
                FakeAuthenticationService(currentSessionResult = anAuthSession(subscriptionTier = SubscriptionTier.PRO))
            val userProfileRepository =
                FakeUserProfileRepository(
                    ensureProfileResult = Result.success(aServerProfile(tier = SubscriptionTier.PRO)),
                )
            val tierCache = FakeSubscriptionTierCache()
            val sut = AuthCoordinator(authService, userProfileRepository, tierCache, FakeWatchZoneRepository())

            sut.onSignedIn()

            assertEquals(listOf(SubscriptionTier.PRO), tierCache.writeCalls)
        }

    @Test
    fun `onboardingPresentation starts Undetermined before onSignedIn has run`() {
        val sut =
            AuthCoordinator(
                FakeAuthenticationService(),
                FakeUserProfileRepository(),
                FakeSubscriptionTierCache(),
                FakeWatchZoneRepository(),
            )

        assertEquals(OnboardingPresentation.Undetermined, sut.onboardingPresentation.value)
    }

    @Test
    fun `onSignedIn resolves onboardingPresentation to Required when the account has zero watch zones`() =
        runTest {
            val sut =
                AuthCoordinator(
                    FakeAuthenticationService(),
                    FakeUserProfileRepository(),
                    FakeSubscriptionTierCache(),
                    FakeWatchZoneRepository(stored = mutableListOf()),
                )

            sut.onSignedIn()

            assertEquals(OnboardingPresentation.Required, sut.onboardingPresentation.value)
        }

    @Test
    fun `onSignedIn resolves onboardingPresentation to NotRequired when the account already has a watch zone`() =
        runTest {
            val sut =
                AuthCoordinator(
                    FakeAuthenticationService(),
                    FakeUserProfileRepository(),
                    FakeSubscriptionTierCache(),
                    FakeWatchZoneRepository(stored = mutableListOf(aWatchZone())),
                )

            sut.onSignedIn()

            assertEquals(OnboardingPresentation.NotRequired, sut.onboardingPresentation.value)
        }

    @Test
    fun `a watch-zone fetch failure fails open to NotRequired rather than stranding the user Undetermined`() =
        runTest {
            val watchZoneRepository = FakeWatchZoneRepository().apply { zonesFailWith = DomainError.NetworkUnavailable }
            val sut =
                AuthCoordinator(
                    FakeAuthenticationService(),
                    FakeUserProfileRepository(),
                    FakeSubscriptionTierCache(),
                    watchZoneRepository,
                )

            sut.onSignedIn()

            assertEquals(OnboardingPresentation.NotRequired, sut.onboardingPresentation.value)
        }

    @Test
    fun `onOnboardingCompleted forces onboardingPresentation to NotRequired`() {
        val sut =
            AuthCoordinator(
                FakeAuthenticationService(),
                FakeUserProfileRepository(),
                FakeSubscriptionTierCache(),
                FakeWatchZoneRepository(stored = mutableListOf()),
            )

        sut.onOnboardingCompleted()

        assertEquals(OnboardingPresentation.NotRequired, sut.onboardingPresentation.value)
    }

    /**
     * The reproduction test for tc-k9fk/#549: evaluating the watch-zone gate
     * before ensure-profile completes crashed onboarding in production. Two
     * purpose-built local fakes (not the shared `Fake*` fixtures) record into
     * one shared list so the ordering, not just the outcome, is asserted.
     */
    @Test
    fun `the watch-zone gate check is never evaluated before ensure-profile completes`() =
        runTest {
            val callOrder = mutableListOf<String>()
            val userProfileRepository =
                object : UserProfileRepository {
                    override suspend fun ensureProfile(): ServerProfile {
                        callOrder += "ensureProfile"
                        return aServerProfile(tier = SubscriptionTier.PRO)
                    }

                    override suspend fun fetchProfile(): ServerProfile? = error("not used by this test")

                    override suspend fun updatePreferences(preferences: UserPreferences): ServerProfile =
                        error("not used by this test")

                    override suspend fun deleteAccount(): Unit = error("not used by this test")

                    override suspend fun exportData(): ByteArray = error("not used by this test")
                }
            val watchZoneRepository =
                object : WatchZoneRepository {
                    override suspend fun zones(): List<WatchZone> {
                        callOrder += "zones"
                        return emptyList()
                    }

                    override suspend fun create(zone: WatchZone): Unit = error("not used by this test")

                    override suspend fun update(zone: WatchZone): Unit = error("not used by this test")

                    override suspend fun delete(id: WatchZoneId): Unit = error("not used by this test")
                }
            val sut =
                AuthCoordinator(
                    FakeAuthenticationService(),
                    userProfileRepository,
                    FakeSubscriptionTierCache(),
                    watchZoneRepository,
                )

            sut.onSignedIn()

            assertEquals(listOf("ensureProfile", "zones"), callOrder)
        }
}
