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

/**
 * Linear step order, with no back navigation from [OnboardingStep.Welcome]
 * (tc-7ttz). Port of iOS `OnboardingViewModelTests`'s step-navigation cases.
 */
@ExtendWith(MainDispatcherExtension::class)
class OnboardingViewModelStepTest {
    private fun makeViewModel(tier: SubscriptionTier = SubscriptionTier.FREE) =
        OnboardingViewModel(FakePostcodeGeocoder(), FakeWatchZoneRepository(), FakeOnboardingRepository(), tier)

    @Test
    fun `a fresh wizard starts on the Welcome step`() {
        assertEquals(OnboardingStep.Welcome, makeViewModel().uiState.value.step)
    }

    @Test
    fun `advance moves Welcome to Postcode unconditionally`() {
        val viewModel = makeViewModel()

        viewModel.advance()

        assertEquals(OnboardingStep.Postcode, viewModel.uiState.value.step)
    }

    @Test
    fun `back is a no-op on the Welcome step`() {
        val viewModel = makeViewModel()

        viewModel.back()

        assertEquals(OnboardingStep.Welcome, viewModel.uiState.value.step)
    }

    @Test
    fun `advance from Postcode is blocked until a postcode has been resolved`() {
        val viewModel = makeViewModel()
        viewModel.advance() // Welcome -> Postcode

        viewModel.advance()

        assertEquals(OnboardingStep.Postcode, viewModel.uiState.value.step)
    }

    @Test
    fun `advance moves Postcode to Radius once geocoding has succeeded`() {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                FakeWatchZoneRepository(),
                FakeOnboardingRepository(),
                SubscriptionTier.FREE,
            )
        viewModel.advance() // Welcome -> Postcode
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()

        viewModel.advance()

        assertEquals(OnboardingStep.Radius, viewModel.uiState.value.step)
    }

    @Test
    fun `advance does nothing from Radius - confirmRadius is the only way past it`() {
        val viewModel = aViewModelOnRadiusStep()

        viewModel.advance()

        assertEquals(OnboardingStep.Radius, viewModel.uiState.value.step)
    }

    @Test
    fun `back steps backward through Postcode and Radius`() {
        val viewModel = aViewModelOnRadiusStep()

        viewModel.back()
        assertEquals(OnboardingStep.Postcode, viewModel.uiState.value.step)

        viewModel.back()
        assertEquals(OnboardingStep.Welcome, viewModel.uiState.value.step)

        viewModel.back()
        assertEquals(OnboardingStep.Welcome, viewModel.uiState.value.step)
    }

    @Test
    fun `back from NotificationPermission returns to Radius`() {
        val viewModel = aViewModelOnRadiusStep()
        viewModel.confirmRadius()

        viewModel.back()

        assertEquals(OnboardingStep.Radius, viewModel.uiState.value.step)
    }

    private fun aViewModelOnRadiusStep(): OnboardingViewModel {
        val viewModel =
            OnboardingViewModel(
                FakePostcodeGeocoder(Result.success(aCoordinate())),
                FakeWatchZoneRepository(),
                FakeOnboardingRepository(),
                SubscriptionTier.FREE,
            )
        viewModel.advance() // Welcome -> Postcode
        viewModel.updatePostcode("SW1A 1AA")
        viewModel.lookUpPostcode()
        viewModel.advance() // Postcode -> Radius
        return viewModel
    }
}
