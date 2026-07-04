package uk.towncrierapp.data.applications

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * The `applicationsListSort` / `lastSelectedZone.applications` DataStore
 * latches — key names reused verbatim from iOS (epic #770 pre-resolved
 * decision). Port of the DataStore latch pattern established by
 * `DataStoreSubscriptionTierCacheTest` (GH#775).
 */
class DataStoreApplicationListPreferencesStoreTest {
    private fun aDataStore(directory: File) = PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

    @Test
    fun `readSort returns null when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreApplicationListPreferencesStore(aDataStore(directory))

        assertNull(sut.readSort())
    }

    @Test
    fun `writeSort then readSort round-trips the sort order`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreApplicationListPreferencesStore(aDataStore(directory))

        sut.writeSort(ApplicationSortOrder.OLDEST)

        assertEquals(ApplicationSortOrder.OLDEST, sut.readSort())
    }

    @Test
    fun `readLastSelectedZoneId returns null when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreApplicationListPreferencesStore(aDataStore(directory))

        assertNull(sut.readLastSelectedZoneId())
    }

    @Test
    fun `writeLastSelectedZoneId then readLastSelectedZoneId round-trips the zone id`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreApplicationListPreferencesStore(aDataStore(directory))

        sut.writeLastSelectedZoneId(WatchZoneId("wz-9"))

        assertEquals(WatchZoneId("wz-9"), sut.readLastSelectedZoneId())
    }
}
