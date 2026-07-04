package uk.towncrierapp.data.subscriptions

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import java.io.File
import kotlin.test.assertEquals
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
}
