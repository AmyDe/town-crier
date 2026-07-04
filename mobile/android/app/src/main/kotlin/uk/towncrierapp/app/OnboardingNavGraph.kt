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
 * #783 hasn't shipped the paywall composable yet, so the radius step's
 * "Unlock larger zones" chip is hidden entirely rather than routed to a dead
 * tap target (tc-7ttz). Flip once the paywall exists and wire
 * [OnboardingViewModel.reconcileTierAfterUpgrade] to its dismiss callback.
 */
internal const val PAYWALL_AVAILABLE: Boolean = false

/**
 * The onboarding wizard: constructed ONCE per presentation (not scoped to
 * any nav-graph destination, since it's shown outside the NavHost entirely -
 * see [TownCrierApp]) so it survives recomposition for as long as the gate
 * says it's required, matching the "nav-graph-scoped, not screen-scoped"
 * requirement the wizard's iOS counterpart has (tc-7ttz).
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
    OnboardingRoute(viewModel = viewModel, onOnboardingComplete = onOnboardingComplete)
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
