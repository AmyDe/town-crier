package uk.towncrierapp.data

/**
 * Placeholder establishing `:data` as an Android library depending only on
 * `:domain` (see android-coding-standards skill, architecture-and-modules.md).
 * Real repository implementations (`Http*`, `DataStore*`), DTOs, and the
 * hand-rolled `ApiClient` land feature-by-feature in later phases of the
 * Android parity epic (#770).
 */
public object DataModule {
    public const val NAME: String = "data"
}
