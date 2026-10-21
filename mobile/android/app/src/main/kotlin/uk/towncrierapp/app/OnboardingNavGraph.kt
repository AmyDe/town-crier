package uk.towncrierapp.app

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.features.onboarding.OnboardingRoute
import uk.towncrierapp.presentation.features.onboarding.OnboardingViewModel

/**
 * The paywall composable doesn't exist yet, so the radius step's "Unlock
 * larger zones" chip is hidden rather than routed to a dead tap target.
 */
internal const val PAYWALL_AVAILABLE: Boolean = false

/**
 * The onboarding wizard: constructed once per presentation, outside the
 * NavHost (see [TownCrierApp]), so it survives recomposition for as long as
 * the gate says it's required.
 */
@Composable
internal fun OnboardingGate(
    appGraph: AppGraph,
    subscriptionTier: SubscriptionTier,
    onOnboardingComplete: () -> Unit,
) {
    val viewModel: OnboardingViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        OnboardingViewModel(
                            appGraph.postcodeGeocoder,
                            appGraph.watchZoneRepository,
                            appGraph.onboardingRepository,
                            subscriptionTier,
                            paywallAvailable = PAYWALL_AVAILABLE,
                        )
                    }
                },
        )
    OnboardingRoute(
        viewModel = viewModel,
        onOnboardingComplete = onOnboardingComplete,
        onUpgradeRequired = {},
    )
}

/** Shown while [uk.towncrierapp.presentation.auth.OnboardingPresentation] is `Undetermined` - never flash Required/NotRequired before account state resolves. */
@Composable
internal fun OnboardingLoadingScreen(modifier: Modifier = Modifier) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
        }
    }
}
