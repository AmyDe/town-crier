package uk.towncrierapp.presentation.features.onboarding

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.onboarding.FakeOnboardingRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Completion is best-effort (tc-7ttz, iOS `try?` parity): the in-memory zone
 * is saved via `WatchZoneRepository.create`, the device latch is always set,
 * and a save failure still completes the wizard rather than trapping the
 * user in it. Port of iOS `OnboardingViewModelTests`'s completion cases.
 */
@ExtendWith(MainDispatcherExtension::class)
class OnboardingViewModelCompletionTest {
    private fun aViewModelReadyToFinish(
        watchZoneRepository: FakeWatchZoneRepository,
        onboardingRepository: FakeOnboardingRepository,
    ): OnboardingViewModel {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                watchZoneRepository,
                onboardingRepository,
                SubscriptionTier.FREE,
            )
        viewModel.advance() // Welcome -> Postcode
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance() // Postcode -> Radius
        viewModel.confirmRadius() // Radius -> NotificationPermission
        return viewModel
    }

    @Test
    fun `completeOnboarding sends create with exactly the built name, coordinate, and radius`() {
        val watchZoneRepository = FakeWatchZoneRepository()
        val onboardingRepository = FakeOnboardingRepository()
        val viewModel = aViewModelReadyToFinish(watchZoneRepository, onboardingRepository)
        viewModel.updateRadius(1_500f)
        // updateRadius happens after confirmRadius here purely to prove the
        // SENT payload matches uiState.pendingZone (built at confirm time),
        // not whatever radiusMetres happens to hold afterwards.
        val expectedZone = viewModel.uiState.value.pendingZone

        viewModel.completeOnboarding()

        val sent = watchZoneRepository.createCalls.single()
        assertEquals(expectedZone?.name, sent.name)
        assertEquals(expectedZone?.centre?.latitude, sent.centre.latitude)
        assertEquals(expectedZone?.centre?.longitude, sent.centre.longitude)
        assertEquals(expectedZone?.radiusMetres, sent.radiusMetres)
    }

    @Test
    fun `completeOnboarding sets the device latch and signals completion`() {
        val watchZoneRepository = FakeWatchZoneRepository()
        val onboardingRepository = FakeOnboardingRepository()
        val viewModel = aViewModelReadyToFinish(watchZoneRepository, onboardingRepository)

        viewModel.completeOnboarding()

        assertEquals(listOf(true), onboardingRepository.setOnboardingCompleteCalls)
        assertTrue(viewModel.uiState.value.isComplete)
    }

    @Test
    fun `a save failure still completes the wizard and still sets the latch`() {
        val watchZoneRepository = FakeWatchZoneRepository().apply { createFailWith = DomainError.NetworkUnavailable }
        val onboardingRepository = FakeOnboardingRepository()
        val viewModel = aViewModelReadyToFinish(watchZoneRepository, onboardingRepository)

        viewModel.completeOnboarding()

        assertTrue(viewModel.uiState.value.isComplete)
        assertEquals(listOf(true), onboardingRepository.setOnboardingCompleteCalls)
    }
}
