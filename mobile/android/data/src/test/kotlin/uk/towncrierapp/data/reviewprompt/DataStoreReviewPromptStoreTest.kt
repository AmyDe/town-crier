package uk.towncrierapp.data.reviewprompt

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.emptyPreferences
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.reviewprompt.ReviewPromptState
import java.io.File
import java.io.IOException
import java.time.Instant
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

/**
 * DataStore Preferences-backed [ReviewPromptStore][uk.towncrierapp.domain.reviewprompt.ReviewPromptStore].
 * Port of iOS's `UserDefaults`-backed `UserDefaultsReviewPromptStore` (GH #628).
 */
class DataStoreReviewPromptStoreTest {
    private fun aDataStore(directory: File) =
        PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

    @Test
    fun `load returns a default-initialised state when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreReviewPromptStore(aDataStore(directory))

        assertEquals(ReviewPromptState(), sut.load())
    }

    @Test
    fun `save then load round-trips every field`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreReviewPromptStore(aDataStore(directory))
        val state =
            ReviewPromptState(
                firstLaunchDate = Instant.ofEpochSecond(1_700_000_000),
                engagementScore = 4,
                saveCount = 2,
                lastActiveDayKey = "2023-11-14",
                distinctActiveDays = 3,
                lastPromptDate = Instant.ofEpochSecond(1_690_000_000),
                promptTimestamps = listOf(Instant.ofEpochSecond(1_680_000_000), Instant.ofEpochSecond(1_690_000_000)),
                hasRecordedUpgrade = true,
            )

        sut.save(state)

        assertEquals(state, sut.load())
    }

    @Test
    fun `save then load round-trips null optional fields as null, not sentinel values`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreReviewPromptStore(aDataStore(directory))
        sut.save(ReviewPromptState(firstLaunchDate = Instant.ofEpochSecond(1_700_000_000)))

        val loaded = sut.load()

        assertNull(loaded.lastActiveDayKey)
        assertNull(loaded.lastPromptDate)
    }

    @Test
    fun `saving a fresh state after a populated one clears the previous values`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreReviewPromptStore(aDataStore(directory))
        sut.save(
            ReviewPromptState(
                firstLaunchDate = Instant.ofEpochSecond(1_700_000_000),
                lastPromptDate = Instant.ofEpochSecond(1_690_000_000),
                promptTimestamps = listOf(Instant.ofEpochSecond(1_690_000_000)),
            ),
        )

        sut.save(ReviewPromptState())

        assertEquals(ReviewPromptState(), sut.load())
    }

    @Test
    fun `save swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreReviewPromptStore(ThrowingDataStore(IOException("disk full")))

            sut.save(ReviewPromptState())
        }

    @Test
    fun `save rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreReviewPromptStore(ThrowingDataStore(CancellationException("cancelled")))

            assertFailsWith<CancellationException> { sut.save(ReviewPromptState()) }
        }
}

/** A [DataStore] whose `updateData` (and so `edit { }`) always throws [exception] — proves write methods guard against it (tc-l31ve). */
private class ThrowingDataStore(
    private val exception: Throwable,
) : DataStore<Preferences> {
    override val data = flowOf(emptyPreferences())

    override suspend fun updateData(transform: suspend (t: Preferences) -> Preferences): Preferences = throw exception
}
