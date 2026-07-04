package uk.towncrierapp.data.settings

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.settings.AppearancePreference
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** The `appearanceMode` DataStore latch — same key name as iOS (epic #770 pre-resolved decision). */
class DataStoreAppearanceStoreTest {
    private fun aDataStore(directory: File) =
        PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

    @Test
    fun `read returns null when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreAppearanceStore(aDataStore(directory))

        assertNull(sut.read())
    }

    @Test
    fun `write then read round-trips the preference`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreAppearanceStore(aDataStore(directory))

        sut.write(AppearancePreference.OLED_DARK)

        assertEquals(AppearancePreference.OLED_DARK, sut.read())
    }
}
