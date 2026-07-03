package uk.towncrierapp.domain

/**
 * Placeholder establishing `:domain` as a pure Kotlin/JVM module — no `android.*`,
 * no HTTP, no serialization (see android-coding-standards skill,
 * architecture-and-modules.md). Real domain entities (WatchZone,
 * PlanningApplication, Tier, ...) and their repository ports land feature-by-
 * feature in later phases of the Android parity epic (#770).
 */
public object DomainModule {
    public const val NAME: String = "domain"
}
