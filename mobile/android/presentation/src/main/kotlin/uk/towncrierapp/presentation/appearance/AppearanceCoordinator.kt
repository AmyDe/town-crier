package uk.towncrierapp.presentation.appearance

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import uk.towncrierapp.domain.settings.AppearancePreference
import uk.towncrierapp.domain.settings.AppearanceStore
import uk.towncrierapp.presentation.designsystem.Appearance

/**
 * Owns the app-wide appearance preference (System/Light/Dark/OLED Dark,
 * epic #770): reads/writes it via the injected [AppearanceStore] and
 * exposes it as a [StateFlow] `MainActivity` feeds straight into
 * `TownCrierTheme`, so a [setAppearance] call restyles the whole app the
 * instant it's called — no separate "apply" step. Constructed once in
 * `AppGraph`, alongside `AuthCoordinator`.
 */
public class AppearanceCoordinator(
    private val store: AppearanceStore,
) {
    private val _appearance = MutableStateFlow(Appearance.System)
    public val appearance: StateFlow<Appearance> = _appearance.asStateFlow()

    /** Resolves the persisted preference (falling back to [Appearance.System] when none is stored yet). */
    public suspend fun load() {
        _appearance.value = store.read()?.toAppearance() ?: Appearance.System
    }

    /** Updates the live [appearance] StateFlow immediately, then persists the mapped preference. */
    public suspend fun setAppearance(value: Appearance) {
        _appearance.value = value
        store.write(value.toPreference())
    }
}

internal fun Appearance.toPreference(): AppearancePreference =
    when (this) {
        Appearance.System -> AppearancePreference.SYSTEM
        Appearance.Light -> AppearancePreference.LIGHT
        Appearance.Dark -> AppearancePreference.DARK
        Appearance.OledDark -> AppearancePreference.OLED_DARK
    }

internal fun AppearancePreference.toAppearance(): Appearance =
    when (this) {
        AppearancePreference.SYSTEM -> Appearance.System
        AppearancePreference.LIGHT -> Appearance.Light
        AppearancePreference.DARK -> Appearance.Dark
        AppearancePreference.OLED_DARK -> Appearance.OledDark
    }
