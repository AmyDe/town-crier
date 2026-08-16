package uk.towncrierapp.data.applications

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.emptyPreferences
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.io.File
import java.io.IOException
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

/**
 * The `applicationsListSort` / `lastSelectedZone.applications` DataStore
 * latches — key names reused verbatim from iOS (epic #770 pre-resolved
 * decision). Port of the DataStore latch pattern established by
 * `DataStoreSubscriptionTierCacheTest` (GH#775).
 */
class DataStoreApplicationListPreferencesStoreTest {
    private fun aDataStore(directory: File) =
        PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

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

    @Test
    fun `writeSort swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreApplicationListPreferencesStore(ThrowingDataStore(IOException("disk full")))

            sut.writeSort(ApplicationSortOrder.OLDEST)
        }

    @Test
    fun `writeSort rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreApplicationListPreferencesStore(ThrowingDataStore(CancellationException("cancelled")))

            assertFailsWith<CancellationException> { sut.writeSort(ApplicationSortOrder.OLDEST) }
        }

    @Test
    fun `writeLastSelectedZoneId swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreApplicationListPreferencesStore(ThrowingDataStore(IOException("disk full")))

            sut.writeLastSelectedZoneId(WatchZoneId("wz-9"))
        }

    @Test
    fun `writeLastSelectedZoneId rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreApplicationListPreferencesStore(ThrowingDataStore(CancellationException("cancelled")))

            assertFailsWith<CancellationException> { sut.writeLastSelectedZoneId(WatchZoneId("wz-9")) }
        }
}

/** A [DataStore] whose `updateData` (and so `edit { }`) always throws [exception] — proves write methods guard against it (tc-l31ve). */
private class ThrowingDataStore(
    private val exception: Throwable,
) : DataStore<Preferences> {
    override val data = flowOf(emptyPreferences())

    override suspend fun updateData(transform: suspend (t: Preferences) -> Preferences): Preferences = throw exception
}
