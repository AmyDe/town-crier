package uk.towncrierapp.domain.settings

/** Persists the user's local-only appearance preference (`appearanceMode`, epic #770). No server, network, or PII involved. */
public interface AppearanceStore {
    /** Returns the persisted preference, or `null` if nothing has been chosen yet. */
    public suspend fun read(): AppearancePreference?

    /** Persists the given preference. */
    public suspend fun write(preference: AppearancePreference)
}
