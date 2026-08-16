package uk.towncrierapp.data.onboarding

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.emptyPreferences
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.io.IOException
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse

/**
 * The `isOnboardingComplete` DataStore Preferences latch - a fast-path hint
 * only, never the source of truth for whether the wizard is required
 * (tc-7ttz; see `AuthCoordinator`). `PreferenceDataStoreFactory.create` over
 * a temp file runs on the plain JVM - no Robolectric needed.
 */
class DataStoreOnboardingRepositoryTest {
    private fun aDataStore(directory: File) =
        PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

    @Test
    fun `isOnboardingComplete defaults to false when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreOnboardingRepository(aDataStore(directory))

        assertFalse(sut.isOnboardingComplete())
    }

    @Test
    fun `setOnboardingComplete then isOnboardingComplete round-trips true`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreOnboardingRepository(aDataStore(directory))

        sut.setOnboardingComplete(true)

        assertEquals(true, sut.isOnboardingComplete())
    }

    @Test
    fun `a second write overwrites the first`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreOnboardingRepository(aDataStore(directory))

        sut.setOnboardingComplete(true)
        sut.setOnboardingComplete(false)

        assertFalse(sut.isOnboardingComplete())
    }

    @Test
    fun `setOnboardingComplete swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreOnboardingRepository(ThrowingDataStore(IOException("disk full")))

            sut.setOnboardingComplete(true)
        }

    @Test
    fun `setOnboardingComplete rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreOnboardingRepository(ThrowingDataStore(CancellationException("cancelled")))

            assertFailsWith<CancellationException> { sut.setOnboardingComplete(true) }
        }
}

/** A [DataStore] whose `updateData` (and so `edit { }`) always throws [exception] — proves write methods guard against it (tc-l31ve). */
private class ThrowingDataStore(
    private val exception: Throwable,
) : DataStore<Preferences> {
    override val data = flowOf(emptyPreferences())

    override suspend fun updateData(transform: suspend (t: Preferences) -> Preferences): Preferences = throw exception
}
