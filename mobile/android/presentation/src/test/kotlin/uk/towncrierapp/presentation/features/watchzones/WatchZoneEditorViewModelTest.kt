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
 * Create/edit field state + save-enablement + radius clamping — port of iOS
 * `WatchZoneEditorViewModelTests`. Save-outcome routing (success/403/other
 * error) lives in [WatchZoneEditorViewModelSaveOutcomeTest]; entitlement
 * gating lives in [WatchZoneEditorViewModelGatingTest] (tc-z95t).
 */
@ExtendWith(MainDispatcherExtension::class)
class WatchZoneEditorViewModelTest {
    @Test
    fun `a new editor is not editing and starts with save disabled`() {
        val viewModel =
            WatchZoneEditorViewModel(FakePostcodeGeocoder(), FakeWatchZoneRepository(), SubscriptionTier.FREE)

        val state = viewModel.uiState.value

        assertFalse(state.isEditing)
        assertFalse(state.isSaveEnabled)
    }

    @Test
    fun `editing an existing zone pre-fills its fields and starts with save enabled`() {
        val zone = aWatchZone(name = "Home", radiusMetres = 1_500.0)
        val viewModel =
            WatchZoneEditorViewModel(
                FakePostcodeGeocoder(),
                FakeWatchZoneRepository(),
                SubscriptionTier.PERSONAL,
                editingZone = zone,
            )

        val state = viewModel.uiState.value

        assertTrue(state.isEditing)
        assertEquals("Home", state.name)
        assertEquals(zone.centre, state.geocodedCoordinate)
        assertEquals(1_500f, state.radiusMetres)
        assertTrue(state.isSaveEnabled)
    }

    @Test
    fun `submitting a postcode geocodes and enables save once a name is present`() {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = WatchZoneEditorViewModel(geocoder, FakeWatchZoneRepository(), SubscriptionTier.FREE)
        viewModel.updateName("Home")
        viewModel.updatePostcode("CB1 2AD")

        viewModel.submitPostcode()

        val state = viewModel.uiState.value
        assertEquals(aCoordinate(), state.geocodedCoordinate)
        assertTrue(state.isSaveEnabled)
        assertNull(state.error)
    }

    @Test
    fun `submitting a postcode defaults a blank name to the postcode`() {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = WatchZoneEditorViewModel(geocoder, FakeWatchZoneRepository(), SubscriptionTier.FREE)
        viewModel.updatePostcode("CB1 2AD")

        viewModel.submitPostcode()

        assertEquals("CB1 2AD", viewModel.uiState.value.name)
    }

    @Test
    fun `a geocoding failure sets the inline error and does not enable save`() {
        val geocoder = FakePostcodeGeocoder(Result.failure(DomainError.NotFound))
        val viewModel = WatchZoneEditorViewModel(geocoder, FakeWatchZoneRepository(), SubscriptionTier.FREE)
        viewModel.updatePostcode("ZZ99 9ZZ")

        viewModel.submitPostcode()

        val state = viewModel.uiState.value
        assertEquals(DomainError.NotFound, state.error)
        assertFalse(state.isSaveEnabled)
        assertNull(state.geocodedCoordinate)
    }

    @Test
    fun `save without geocoding does nothing`() {
        val repository = FakeWatchZoneRepository()
        val viewModel = WatchZoneEditorViewModel(FakePostcodeGeocoder(), repository, SubscriptionTier.FREE)
        viewModel.updateName("Home")

        viewModel.save()

        assertTrue(repository.createCalls.isEmpty())
        assertFalse(viewModel.uiState.value.saveCompleted)
    }

    @Test
    fun `radius is clamped to the tier max on save`() {
        val repository = FakeWatchZoneRepository()
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = WatchZoneEditorViewModel(geocoder, repository, SubscriptionTier.FREE)
        viewModel.updateName("Home")
        viewModel.updatePostcode("CB1 2AD")
        viewModel.submitPostcode()
        viewModel.updateRadius(8_000f)

        viewModel.save()

        assertEquals(2_000.0, repository.createCalls.single().radiusMetres)
    }

    @Test
    fun `updateRadius shows the large radius warning at or above the threshold`() {
        val viewModel =
            WatchZoneEditorViewModel(FakePostcodeGeocoder(), FakeWatchZoneRepository(), SubscriptionTier.PRO)

        viewModel.updateRadius(2_100f)
        assertTrue(viewModel.uiState.value.showsLargeRadiusWarning)

        viewModel.updateRadius(2_099f)
        assertFalse(viewModel.uiState.value.showsLargeRadiusWarning)
    }
}
