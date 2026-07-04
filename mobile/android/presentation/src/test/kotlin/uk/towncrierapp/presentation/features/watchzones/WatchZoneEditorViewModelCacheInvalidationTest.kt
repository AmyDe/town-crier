package uk.towncrierapp.presentation.features.watchzones

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.applications.FakeApplicationCacheStore
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.domain.watchzones.aWatchZone
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * tc-cnme: closes #774's cache-invalidation loop now that #775 introduces the
 * cache seam ([uk.towncrierapp.domain.applications.ApplicationCacheStore]).
 * Only the EDIT path (`isEditing == true`) invalidates — mirrors iOS
 * `AppCoordinator+WatchZones.swift`'s `onSave` callback, which has no
 * equivalent zone identity to invalidate on a brand-new create.
 */
@ExtendWith(MainDispatcherExtension::class)
class WatchZoneEditorViewModelCacheInvalidationTest {
    @Test
    fun `a successful edit-save invalidates that zone's applications cache`() {
        val zone = aWatchZone()
        val repository = FakeWatchZoneRepository(mutableListOf(zone))
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val cacheStore = FakeApplicationCacheStore()
        val viewModel =
            WatchZoneEditorViewModel(
                geocoder,
                repository,
                SubscriptionTier.PRO,
                editingZone = zone,
                applicationCacheStore = cacheStore,
            )

        viewModel.save()

        assertEquals(listOf(zone.id), cacheStore.invalidateCalls)
    }

    @Test
    fun `creating a brand-new zone does not invalidate any cache`() {
        val repository = FakeWatchZoneRepository()
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val cacheStore = FakeApplicationCacheStore()
        val viewModel =
            WatchZoneEditorViewModel(geocoder, repository, SubscriptionTier.FREE, applicationCacheStore = cacheStore)
        viewModel.updateName("Home")
        viewModel.updatePostcode("CB1 2AD")
        viewModel.submitPostcode()

        viewModel.save()

        assertTrue(cacheStore.invalidateCalls.isEmpty())
    }

    @Test
    fun `a failed edit-save does not invalidate the cache`() {
        val zone = aWatchZone()
        val repository =
            FakeWatchZoneRepository(mutableListOf(zone)).apply {
                updateFailWith =
                    DomainError.NetworkUnavailable
            }
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val cacheStore = FakeApplicationCacheStore()
        val viewModel =
            WatchZoneEditorViewModel(
                geocoder,
                repository,
                SubscriptionTier.PRO,
                editingZone = zone,
                applicationCacheStore = cacheStore,
            )

        viewModel.save()

        assertTrue(cacheStore.invalidateCalls.isEmpty())
    }
}
