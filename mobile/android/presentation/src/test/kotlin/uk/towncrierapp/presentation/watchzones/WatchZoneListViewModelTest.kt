package uk.towncrierapp.presentation.watchzones

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `load()`/`deleteZone()` + at-quota gating surfaces — port of iOS
 * `WatchZoneListViewModelTests` (tc-z95t). The at-quota matrix is the
 * acceptance-critical part: Free at cap shows the inline upsell card AND the
 * badge; Personal at cap shows the badge only; anyone under cap shows
 * neither.
 */
@ExtendWith(MainDispatcherExtension::class)
class WatchZoneListViewModelTest {
    @Test
    fun `load replaces zones and clears a previous error`() {
        val repository = FakeWatchZoneRepository(mutableListOf(aWatchZone()))
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.PERSONAL))

        viewModel.load()

        val state = viewModel.uiState.value
        assertEquals(listOf(aWatchZone()), state.zones)
        assertFalse(state.isLoading)
        assertNull(state.error)
    }

    @Test
    fun `load failure surfaces the error and clears the loading flag`() {
        val repository = FakeWatchZoneRepository().apply { zonesFailWith = DomainError.NetworkUnavailable }
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.FREE))

        viewModel.load()

        val state = viewModel.uiState.value
        assertEquals(DomainError.NetworkUnavailable, state.error)
        assertFalse(state.isLoading)
    }

    @Test
    fun `free tier at quota shows the inline upsell card and the upgrade badge`() {
        val repository = FakeWatchZoneRepository(mutableListOf(aWatchZone()))
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.FREE))

        viewModel.load()

        val state = viewModel.uiState.value
        assertFalse(state.canAddZone)
        assertTrue(state.showUpgradeBadge)
        assertTrue(state.showsFreeTierUpsell)
    }

    @Test
    fun `personal tier at quota shows the badge only, never the inline upsell card`() {
        val zones =
            mutableListOf(
                aWatchZone(id = WatchZoneId("1")),
                aWatchZone(id = WatchZoneId("2")),
                aWatchZone(id = WatchZoneId("3")),
            )
        val repository = FakeWatchZoneRepository(zones)
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.PERSONAL))

        viewModel.load()

        val state = viewModel.uiState.value
        assertFalse(state.canAddZone)
        assertTrue(state.showUpgradeBadge)
        assertFalse(state.showsFreeTierUpsell)
    }

    @Test
    fun `under quota shows neither the badge nor the inline upsell card`() {
        val repository = FakeWatchZoneRepository()
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.FREE))

        viewModel.load()

        val state = viewModel.uiState.value
        assertTrue(state.canAddZone)
        assertFalse(state.showUpgradeBadge)
        assertFalse(state.showsFreeTierUpsell)
    }

    @Test
    fun `pro tier is never at quota regardless of zone count`() {
        val zones = (1..50).map { aWatchZone(id = WatchZoneId("$it")) }.toMutableList()
        val repository = FakeWatchZoneRepository(zones)
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.PRO))

        viewModel.load()

        val state = viewModel.uiState.value
        assertTrue(state.canAddZone)
        assertFalse(state.showUpgradeBadge)
        assertFalse(state.showsFreeTierUpsell)
    }

    @Test
    fun `deleteZone removes the zone from state on success`() {
        val zone = aWatchZone()
        val repository = FakeWatchZoneRepository(mutableListOf(zone))
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.PRO))
        viewModel.load()

        viewModel.deleteZone(zone)

        assertEquals(emptyList(), viewModel.uiState.value.zones)
        assertEquals(listOf(zone.id), repository.deleteCalls)
    }

    @Test
    fun `deleteZone failure surfaces the error and keeps the zone`() {
        val zone = aWatchZone()
        val repository =
            FakeWatchZoneRepository(mutableListOf(zone)).apply { deleteFailWith = DomainError.NetworkUnavailable }
        val viewModel = WatchZoneListViewModel(repository, FeatureGate(SubscriptionTier.PRO))
        viewModel.load()

        viewModel.deleteZone(zone)

        assertEquals(listOf(zone), viewModel.uiState.value.zones)
        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.error)
    }
}
