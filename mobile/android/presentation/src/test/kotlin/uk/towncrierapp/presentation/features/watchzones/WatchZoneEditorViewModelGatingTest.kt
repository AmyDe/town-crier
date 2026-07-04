package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The instant push/email toggles are paid-only (tc-bd6i) — free-tier users
 * see them locked with an upsell, matching the server never delivering
 * instant alerts to free accounts. Port of iOS
 * `WatchZoneEditorViewModelGatingTests`.
 */
@ExtendWith(MainDispatcherExtension::class)
class WatchZoneEditorViewModelGatingTest {
    private fun makeViewModel(tier: SubscriptionTier) =
        WatchZoneEditorViewModel(FakePostcodeGeocoder(), FakeWatchZoneRepository(), tier)

    @Test
    fun `featureGate carries the tier`() {
        assertEquals(
            SubscriptionTier.FREE,
            makeViewModel(SubscriptionTier.FREE)
                .uiState.value.featureGate.tier,
        )
        assertEquals(
            SubscriptionTier.PRO,
            makeViewModel(SubscriptionTier.PRO)
                .uiState.value.featureGate.tier,
        )
    }

    @Test
    fun `free tier does not grant the instant alert entitlement`() {
        val state = makeViewModel(SubscriptionTier.FREE).uiState.value

        assertFalse(state.featureGate.hasEntitlement(state.instantAlertEntitlement))
    }

    @Test
    fun `personal and pro tiers grant the instant alert entitlement`() {
        val personal = makeViewModel(SubscriptionTier.PERSONAL).uiState.value
        assertTrue(personal.featureGate.hasEntitlement(personal.instantAlertEntitlement))

        val pro = makeViewModel(SubscriptionTier.PRO).uiState.value
        assertTrue(pro.featureGate.hasEntitlement(pro.instantAlertEntitlement))
    }

    @Test
    fun `canUnlockLargerRadius is true below Pro and false at Pro`() {
        assertTrue(makeViewModel(SubscriptionTier.FREE).uiState.value.canUnlockLargerRadius)
        assertTrue(makeViewModel(SubscriptionTier.PERSONAL).uiState.value.canUnlockLargerRadius)
        assertFalse(makeViewModel(SubscriptionTier.PRO).uiState.value.canUnlockLargerRadius)
    }

    @Test
    fun `requestInstantAlertUpgrade routes to the paywall without setting an inline error`() {
        val viewModel = makeViewModel(SubscriptionTier.FREE)

        viewModel.requestInstantAlertUpgrade()

        val state = viewModel.uiState.value
        assertTrue(state.navigateToPaywall)
        assertNull(state.error)
    }
}
