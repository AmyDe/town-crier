package uk.towncrierapp.domain.settings

/** Hand-written fake for [AppearanceStore]: an in-memory single-slot preference, `null` until written. */
public class FakeAppearanceStore(
    public var stored: AppearancePreference? = null,
) : AppearanceStore {
    public val writeCalls: MutableList<AppearancePreference> = mutableListOf()

    override suspend fun read(): AppearancePreference? = stored

    override suspend fun write(preference: AppearancePreference) {
        stored = preference
        writeCalls += preference
    }
}
