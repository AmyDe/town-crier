package uk.towncrierapp.data.settings

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.emptyPreferences
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.settings.AppearancePreference
import java.io.File
import java.io.IOException
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
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

    @Test
    fun `write swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreAppearanceStore(ThrowingDataStore(IOException("disk full")))

            sut.write(AppearancePreference.OLED_DARK)
        }

    @Test
    fun `write rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreAppearanceStore(ThrowingDataStore(CancellationException("cancelled")))

            assertFailsWith<CancellationException> { sut.write(AppearancePreference.OLED_DARK) }
        }
}

/** A [DataStore] whose `updateData` (and so `edit { }`) always throws [exception] — proves write methods guard against it (tc-l31ve). */
private class ThrowingDataStore(
    private val exception: Throwable,
) : DataStore<Preferences> {
    override val data = flowOf(emptyPreferences())

    override suspend fun updateData(transform: suspend (t: Preferences) -> Preferences): Preferences = throw exception
}
