package uk.towncrierapp.data.map

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.watchzones.WatchZoneId
import java.io.File
import kotlin.test.assertEquals
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
}
