package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `save()` outcome routing (tc-gpjk / tc-z95t): success completes the save;
 * a quota 403 (`InsufficientEntitlement`) routes to the paywall placeholder
 * and leaves `error` unset; any other failure sets the inline error and
 * leaves the editor open. Port of iOS
 * `WatchZoneEditorSaveOutcomeTests`.
 */
@ExtendWith(MainDispatcherExtension::class)
class WatchZoneEditorViewModelSaveOutcomeTest {
    private fun geocodedViewModel(
        repository: FakeWatchZoneRepository = FakeWatchZoneRepository(),
    ): WatchZoneEditorViewModel {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = WatchZoneEditorViewModel(geocoder, repository, SubscriptionTier.FREE)
        viewModel.updateName("Home")
        viewModel.updatePostcode("CB1 2AD")
        viewModel.submitPostcode()
        return viewModel
    }

    @Test
    fun `a successful create completes the save and clears any error`() {
        val repository = FakeWatchZoneRepository()
        val viewModel = geocodedViewModel(repository)

        viewModel.save()

        val state = viewModel.uiState.value
        assertTrue(state.saveCompleted)
        assertNull(state.error)
        assertEquals(1, repository.createCalls.size)
    }

    @Test
    fun `a quota 403 routes to the paywall without setting an inline error or completing the save`() {
        val repository =
            FakeWatchZoneRepository().apply { createFailWith = DomainError.InsufficientEntitlement("personal") }
        val viewModel = geocodedViewModel(repository)

        viewModel.save()

        val state = viewModel.uiState.value
        assertTrue(state.navigateToPaywall)
        assertFalse(state.saveCompleted)
        assertNull(state.error)
    }

    @Test
    fun `any other failure sets the inline error and does not route to the paywall`() {
        val repository = FakeWatchZoneRepository().apply { createFailWith = DomainError.NetworkUnavailable }
        val viewModel = geocodedViewModel(repository)

        viewModel.save()

        val state = viewModel.uiState.value
        assertEquals(DomainError.NetworkUnavailable, state.error)
        assertFalse(state.navigateToPaywall)
        assertFalse(state.saveCompleted)
    }

    @Test
    fun `editing an existing zone calls update, not create`() {
        val zone = aWatchZone()
        val repository = FakeWatchZoneRepository(mutableListOf(zone))
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = WatchZoneEditorViewModel(geocoder, repository, SubscriptionTier.PRO, editingZone = zone)

        viewModel.save()

        assertEquals(1, repository.updateCalls.size)
        assertTrue(repository.createCalls.isEmpty())
    }
}
