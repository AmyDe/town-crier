package uk.towncrierapp.data.map

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.emptyPreferences
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.io.File
import java.io.IOException
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

/** The `lastSelectedZone.map` DataStore latch (GH#776) — port of the DataStore latch pattern established by `DataStoreApplicationListPreferencesStoreTest`. */
class DataStoreMapPreferencesStoreTest {
    private fun aDataStore(directory: File) =
        PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

    @Test
    fun `readLastSelectedZoneId returns null when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreMapPreferencesStore(aDataStore(directory))

        assertNull(sut.readLastSelectedZoneId())
    }

    @Test
    fun `writeLastSelectedZoneId then readLastSelectedZoneId round-trips the zone id`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreMapPreferencesStore(aDataStore(directory))

        sut.writeLastSelectedZoneId(WatchZoneId("wz-9"))

        assertEquals(WatchZoneId("wz-9"), sut.readLastSelectedZoneId())
    }

    @Test
    fun `writeLastSelectedZoneId swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreMapPreferencesStore(ThrowingDataStore(IOException("disk full")))

            sut.writeLastSelectedZoneId(WatchZoneId("wz-9"))
        }

    @Test
    fun `writeLastSelectedZoneId rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreMapPreferencesStore(ThrowingDataStore(CancellationException("cancelled")))

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
