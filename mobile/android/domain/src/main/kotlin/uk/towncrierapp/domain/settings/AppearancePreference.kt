package uk.towncrierapp.domain.settings

/**
 * The user's persisted appearance choice — mirrors iOS's local
 * `appearanceMode` (epic #770). Kept distinct from `:presentation`'s
 * `Appearance` enum (which the Compose theme resolves directly) so
 * `:domain` stays free of design-system-adjacent presentation types; the
 * two are mapped 1:1 at the `:presentation` boundary.
 */
public enum class AppearancePreference(
    public val wireValue: String,
) {
    SYSTEM("system"),
    LIGHT("light"),
    DARK("dark"),
    OLED_DARK("oledDark"),
    ;

    public companion object {
        /** Parses a preference from its exact wire string. Returns `null` for anything unrecognised. */
        public fun fromWireValue(value: String): AppearancePreference? = entries.firstOrNull { it.wireValue == value }
    }
}
