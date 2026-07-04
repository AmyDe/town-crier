package uk.towncrierapp.presentation.features.onboarding

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.onboarding.FakeOnboardingRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * The radius step: tier-max clamping, the large-radius warning threshold,
 * `confirmRadius` building the in-memory `WatchZone` (nothing saved yet),
 * and [OnboardingViewModel.reconcileTierAfterUpgrade] raising the max on the
 * SAME live instance without resetting anything already entered (tc-7ttz).
 * Port of iOS `OnboardingViewModelTests`'s radius-step + paywall-return
 * cases.
 */
@ExtendWith(MainDispatcherExtension::class)
class OnboardingViewModelRadiusTest {
    private fun aViewModelOnRadiusStep(tier: SubscriptionTier = SubscriptionTier.FREE): OnboardingViewModel {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                FakeWatchZoneRepository(),
                FakeOnboardingRepository(),
                tier,
            )
        viewModel.advance() // Welcome -> Postcode
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance() // Postcode -> Radius
        return viewModel
    }

    @Test
    fun `radius defaults to 1000m with a 100m floor`() {
        val state = aViewModelOnRadiusStep().uiState.value

        assertEquals(1_000f, state.radiusMetres)
        assertEquals(100f, state.minRadiusMetres)
    }

    @Test
    fun `maxRadiusMetres matches each tier's limit`() {
        assertEquals(2_000f, aViewModelOnRadiusStep(SubscriptionTier.FREE).uiState.value.maxRadiusMetres)
        assertEquals(5_000f, aViewModelOnRadiusStep(SubscriptionTier.PERSONAL).uiState.value.maxRadiusMetres)
        assertEquals(10_000f, aViewModelOnRadiusStep(SubscriptionTier.PRO).uiState.value.maxRadiusMetres)
    }

    @Test
    fun `updateRadius shows the large radius warning at or above the threshold`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.PRO)

        viewModel.updateRadius(2_100f)
        assertTrue(viewModel.uiState.value.showsLargeRadiusWarning)

        viewModel.updateRadius(2_099f)
        assertFalse(viewModel.uiState.value.showsLargeRadiusWarning)
    }

    @Test
    fun `confirmRadius builds the zone in memory without saving it`() {
        val repository = FakeWatchZoneRepository()
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                repository,
                FakeOnboardingRepository(),
                SubscriptionTier.FREE,
            )
        viewModel.advance()
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance()

        viewModel.confirmRadius()

        val zone = viewModel.uiState.value.pendingZone
        assertEquals(aCoordinate(), zone?.centre)
        assertTrue(repository.createCalls.isEmpty())
    }

    @Test
    fun `confirmRadius advances to the notification-permission step`() {
        val viewModel = aViewModelOnRadiusStep()

        viewModel.confirmRadius()

        assertEquals(OnboardingStep.NotificationPermission, viewModel.uiState.value.step)
    }

    @Test
    fun `confirmRadius clamps a radius above the tier max`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)
        viewModel.updateRadius(9_000f)

        viewModel.confirmRadius()

        assertEquals(2_000.0, viewModel.uiState.value.pendingZone?.radiusMetres)
    }

    @Test
    fun `reconcileTierAfterUpgrade raises the max without resetting postcode, coordinate, or radius`() {
        val viewModel = aViewModelOnRadiusStep(SubscriptionTier.FREE)
        viewModel.updateRadius(1_800f)
        val beforeCoordinate = viewModel.uiState.value.geocodedCoordinate
        val beforePostcode = viewModel.uiState.value.postcodeInput

        viewModel.reconcileTierAfterUpgrade(SubscriptionTier.PRO)

        val state = viewModel.uiState.value
        assertEquals(10_000f, state.maxRadiusMetres)
        assertEquals(1_800f, state.radiusMetres)
        assertEquals(beforeCoordinate, state.geocodedCoordinate)
        assertEquals(beforePostcode, state.postcodeInput)
    }

    @Test
    fun `canUnlockLargerRadius is always false while paywallAvailable is false, regardless of tier`() {
        // #783 hasn't shipped - the chip must be hidden entirely, not merely
        // routed to a dead tap target, so this must be false even below Pro.
        assertFalse(aViewModelOnRadiusStep(SubscriptionTier.FREE).uiState.value.canUnlockLargerRadius)
        assertFalse(aViewModelOnRadiusStep(SubscriptionTier.PERSONAL).uiState.value.canUnlockLargerRadius)
    }

    @Test
    fun `when paywallAvailable is true, reconcileTierAfterUpgrade to Pro hides the unlock-larger-zones chip`() {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                FakeWatchZoneRepository(),
                FakeOnboardingRepository(),
                SubscriptionTier.FREE,
                paywallAvailable = true,
            )
        viewModel.advance()
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance()
        assertTrue(viewModel.uiState.value.canUnlockLargerRadius)

        viewModel.reconcileTierAfterUpgrade(SubscriptionTier.PRO)

        assertFalse(viewModel.uiState.value.canUnlockLargerRadius)
    }
}
