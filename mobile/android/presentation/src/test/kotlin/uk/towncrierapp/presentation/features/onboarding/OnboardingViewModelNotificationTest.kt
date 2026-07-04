package uk.towncrierapp.presentation.features.onboarding

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.onboarding.FakeOnboardingRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Step 4's tier-aware copy switch. `hasInstantAlertEntitlement` is what the
 * Screen keys its Enable/Skip-vs-Finish rendering on; the actual OS
 * permission request lives in the Route (`rememberLauncherForActivityResult`
 * needs a composable scope), so this ViewModel only needs to prove the flag
 * is right and that both `requestNotificationPermission` and
 * `skipNotifications` complete the wizard the same way (tc-7ttz). Port of
 * iOS `OnboardingViewModelTests`'s notification-step cases.
 */
@ExtendWith(MainDispatcherExtension::class)
class OnboardingViewModelNotificationTest {
    private fun aViewModelReadyToFinish(tier: SubscriptionTier): OnboardingViewModel {
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
        viewModel.confirmRadius() // Radius -> NotificationPermission
        return viewModel
    }

    @Test
    fun `free tier has no instant alert entitlement`() {
        assertFalse(aViewModelReadyToFinish(SubscriptionTier.FREE).uiState.value.hasInstantAlertEntitlement)
    }

    @Test
    fun `personal and pro tiers have the instant alert entitlement`() {
        assertTrue(aViewModelReadyToFinish(SubscriptionTier.PERSONAL).uiState.value.hasInstantAlertEntitlement)
        assertTrue(aViewModelReadyToFinish(SubscriptionTier.PRO).uiState.value.hasInstantAlertEntitlement)
    }

    @Test
    fun `requestNotificationPermission completes the wizard`() {
        val viewModel = aViewModelReadyToFinish(SubscriptionTier.PERSONAL)

        viewModel.requestNotificationPermission()

        assertTrue(viewModel.uiState.value.isComplete)
    }

    @Test
    fun `skipNotifications also completes the wizard`() {
        val viewModel = aViewModelReadyToFinish(SubscriptionTier.PERSONAL)

        viewModel.skipNotifications()

        assertTrue(viewModel.uiState.value.isComplete)
    }
}
