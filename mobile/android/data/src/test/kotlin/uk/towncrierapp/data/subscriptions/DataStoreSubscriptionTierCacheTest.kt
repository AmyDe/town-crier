package uk.towncrierapp.data.subscriptions

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.emptyPreferences
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.io.File
import java.io.IOException
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

/**
 * The `cachedSubscriptionTier` DataStore Preferences latch — the fast-path
 * read on cold start, before [uk.towncrierapp.domain.subscriptions.resolveTier]'s
 * network round-trip completes (epic #770 "API contract essentials").
 * `PreferenceDataStoreFactory.create` over a temp file runs on the plain JVM
 * — no Robolectric needed.
 */
class DataStoreSubscriptionTierCacheTest {
    private fun aDataStore(directory: File) =
        PreferenceDataStoreFactory.create { File(directory, "test.preferences_pb") }

    @Test
    fun `read returns null when nothing has been written yet`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreSubscriptionTierCache(aDataStore(directory))

        assertNull(sut.read())
    }

    @Test
    fun `write then read round-trips the tier`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreSubscriptionTierCache(aDataStore(directory))

        sut.write(SubscriptionTier.PRO)

        assertEquals(SubscriptionTier.PRO, sut.read())
    }

    @Test
    fun `a second write overwrites the first`(
        @TempDir directory: File,
    ) = runTest {
        val sut = DataStoreSubscriptionTierCache(aDataStore(directory))

        sut.write(SubscriptionTier.PERSONAL)
        sut.write(SubscriptionTier.FREE)

        assertEquals(SubscriptionTier.FREE, sut.read())
    }

    @Test
    fun `write swallows an IOException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreSubscriptionTierCache(ThrowingDataStore(IOException("disk full")))

            sut.write(SubscriptionTier.PRO)
        }

    @Test
    fun `write rethrows a CancellationException from the underlying DataStore`() =
        runTest {
            val sut = DataStoreSubscriptionTierCache(ThrowingDataStore(CancellationException("cancelled")))

            assertFailsWith<CancellationException> { sut.write(SubscriptionTier.PRO) }
        }
}

/** A [DataStore] whose `updateData` (and so `edit { }`) always throws [exception] — proves write methods guard against it (tc-l31ve). */
private class ThrowingDataStore(
    private val exception: Throwable,
) : DataStore<Preferences> {
    override val data = flowOf(emptyPreferences())

    override suspend fun updateData(transform: suspend (t: Preferences) -> Preferences): Preferences = throw exception
}
