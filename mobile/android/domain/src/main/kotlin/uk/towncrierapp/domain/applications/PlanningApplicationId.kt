package uk.towncrierapp.domain.applications

/**
 * A planning application's identity: [authority] (the decimal local-authority
 * area id, as a string) and [name] (the PlanIt case reference, e.g.
 * `"24/0001"` — itself routinely containing further slashes). [value] is the
 * canonical `"authority/name"` wire encoding used as the saved-applications
 * path segment (percent-encoded there — see `ApiSavedApplicationRepository`)
 * and as the reconstruction key for saved-application rows (GH#775; tc-jjl4:
 * saved-state comparison must use this reconstructed id, not a raw uid
 * string). Port of iOS `PlanningApplicationId`.
 */
public data class PlanningApplicationId(
    public val authority: String,
    public val name: String,
) {
    public val value: String get() = "$authority/$name"

    public companion object {
        /** Splits on the FIRST `/` only — [name] itself may contain further slashes. */
        public fun parse(raw: String): PlanningApplicationId =
            PlanningApplicationId(authority = raw.substringBefore("/"), name = raw.substringAfter("/"))
    }
}
