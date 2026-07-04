package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakeZonePreferencesRepository
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.aZoneNotificationPreferences
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** GET/PUT round-trip + tier-locked toggle state — port of iOS `ZonePreferencesViewModelTests` (tc-z95t). */
@ExtendWith(MainDispatcherExtension::class)
class ZonePreferencesViewModelTest {
    @Test
    fun `load fetches and applies the preferences`() {
        val repository =
            FakeZonePreferencesRepository(
                Result.success(
                    aZoneNotificationPreferences(
                        zoneId = WatchZoneId("wz-1"),
                        newApplicationPush = false,
                        decisionEmail = false,
                    ),
                ),
            )
        val viewModel = ZonePreferencesViewModel(repository, WatchZoneId("wz-1"), "Home", SubscriptionTier.PERSONAL)

        viewModel.load()

        val state = viewModel.uiState.value
        assertFalse(state.newApplicationPush)
        assertTrue(state.newApplicationEmail)
        assertTrue(state.decisionPush)
        assertFalse(state.decisionEmail)
        assertFalse(state.isLoading)
        assertNull(state.error)
        assertEquals(listOf(WatchZoneId("wz-1")), repository.fetchPreferencesCalls)
    }

    @Test
    fun `load failure surfaces the error`() {
        val repository = FakeZonePreferencesRepository(Result.failure(DomainError.NetworkUnavailable))
        val viewModel = ZonePreferencesViewModel(repository, WatchZoneId("wz-1"), "Home", SubscriptionTier.FREE)

        viewModel.load()

        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.error)
    }

    @Test
    fun `save sends the current toggle state and completes on success`() {
        val repository = FakeZonePreferencesRepository()
        val viewModel = ZonePreferencesViewModel(repository, WatchZoneId("wz-1"), "Home", SubscriptionTier.PERSONAL)
        viewModel.updateNewApplicationPush(false)
        viewModel.updateDecisionEmail(false)

        viewModel.save()

        val sent = repository.updatePreferencesCalls.single()
        assertEquals(WatchZoneId("wz-1"), sent.zoneId)
        assertFalse(sent.newApplicationPush)
        assertFalse(sent.decisionEmail)
        assertTrue(viewModel.uiState.value.saveCompleted)
    }

    @Test
    fun `save failure surfaces the error and does not complete`() {
        val repository = FakeZonePreferencesRepository().apply { updatePreferencesFailWith = DomainError.NetworkUnavailable }
        val viewModel = ZonePreferencesViewModel(repository, WatchZoneId("wz-1"), "Home", SubscriptionTier.PERSONAL)

        viewModel.save()

        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.error)
        assertFalse(viewModel.uiState.value.saveCompleted)
    }

    @Test
    fun `featureGate reflects the tier so the Screen can lock toggles for Free`() {
        val free = ZonePreferencesViewModel(FakeZonePreferencesRepository(), WatchZoneId("wz-1"), "Home", SubscriptionTier.FREE)
        assertEquals(SubscriptionTier.FREE, free.uiState.value.featureGate.tier)

        val personal =
            ZonePreferencesViewModel(FakeZonePreferencesRepository(), WatchZoneId("wz-1"), "Home", SubscriptionTier.PERSONAL)
        assertEquals(SubscriptionTier.PERSONAL, personal.uiState.value.featureGate.tier)
    }
}
