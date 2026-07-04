package uk.towncrierapp.data.onboarding

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import kotlin.test.assertEquals
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
}
