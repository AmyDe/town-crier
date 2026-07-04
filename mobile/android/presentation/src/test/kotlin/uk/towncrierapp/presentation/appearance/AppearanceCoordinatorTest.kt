package uk.towncrierapp.presentation.appearance

import kotlinx.coroutines.test.runTest
import org.junit.jupiter.api.Test
import uk.towncrierapp.domain.settings.AppearancePreference
import uk.towncrierapp.domain.settings.FakeAppearanceStore
import uk.towncrierapp.presentation.designsystem.Appearance
import kotlin.test.assertEquals

/**
 * The four-way appearance picker's persistence glue (System/Light/Dark/OLED
 * Dark, epic #770). [AppearanceCoordinator.appearance] is what
 * `MainActivity` feeds into `TownCrierTheme`, so a [setAppearance] call
 * restyling the theme immediately is exactly this StateFlow updating
 * synchronously.
 */
class AppearanceCoordinatorTest {
    @Test
    fun `appearance defaults to System before load`() {
        val sut = AppearanceCoordinator(FakeAppearanceStore())

        assertEquals(Appearance.System, sut.appearance.value)
    }

    @Test
    fun `load resolves the persisted preference`() =
        runTest {
            val store = FakeAppearanceStore(stored = AppearancePreference.OLED_DARK)
            val sut = AppearanceCoordinator(store)

            sut.load()

            assertEquals(Appearance.OledDark, sut.appearance.value)
        }

    @Test
    fun `load falls back to System when nothing has been chosen yet`() =
        runTest {
            val sut = AppearanceCoordinator(FakeAppearanceStore(stored = null))

            sut.load()

            assertEquals(Appearance.System, sut.appearance.value)
        }

    @Test
    fun `setAppearance updates the StateFlow immediately and persists the mapped preference`() =
        runTest {
            val store = FakeAppearanceStore()
            val sut = AppearanceCoordinator(store)

            sut.setAppearance(Appearance.Dark)

            assertEquals(Appearance.Dark, sut.appearance.value)
            assertEquals(listOf(AppearancePreference.DARK), store.writeCalls)
        }

    @Test
    fun `every Appearance value round-trips through its AppearancePreference mapping`() =
        runTest {
            val store = FakeAppearanceStore()
            val sut = AppearanceCoordinator(store)

            for (appearance in listOf(Appearance.System, Appearance.Light, Appearance.Dark, Appearance.OledDark)) {
                sut.setAppearance(appearance)
                assertEquals(appearance, sut.appearance.value)

                val reloaded = AppearanceCoordinator(store)
                reloaded.load()
                assertEquals(appearance, reloaded.appearance.value)
            }
        }
}
