package uk.towncrierapp.domain.versionconfig

/** Semantic version (major.minor.patch) for comparing the running app build against the server's minimum. */
public data class AppVersion(
    val major: Int,
    val minor: Int,
    val patch: Int,
) : Comparable<AppVersion> {
    override fun compareTo(other: AppVersion): Int =
        compareValuesBy(this, other, AppVersion::major, AppVersion::minor, AppVersion::patch)

    public companion object {
        /** Parses a version string like "1.2.3". Returns `null` if the format is invalid. */
        public fun parse(value: String): AppVersion? {
            val segments = value.split(".")
            if (segments.size != 3) return null
            val parts = segments.map { it.toIntOrNull() ?: return null }
            return AppVersion(parts[0], parts[1], parts[2])
        }
    }
}
