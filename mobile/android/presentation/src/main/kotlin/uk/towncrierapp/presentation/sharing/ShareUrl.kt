package uk.towncrierapp.presentation.sharing

/**
 * Builds the canonical public share URL for a planning application:
 * `https://share.towncrierapp.uk/a/{authoritySlug}/{ref}`. Port of iOS
 * `ShareURL.build` (GH#738 Slice 4 / GH#782).
 *
 * [ref] is the application's full area-prefixed PlanIt name, verbatim — it
 * contains slashes (e.g. `Kingston/25/02755/CLC`), which are preserved as
 * path separators. [authoritySlug] always comes from the API (`authoritySlug`
 * on the detail/by-slug JSON) — this object never computes one.
 */
public object ShareUrl {
    /** Origin of the public share surface. Mirrors the web `SHARE_ORIGIN` constant and iOS `ShareURL.origin`. */
    public const val ORIGIN: String = "https://share.towncrierapp.uk"

    /**
     * Returns the canonical share URL, or `null` when either component is
     * empty. Only unsafe characters in [ref] are percent-encoded —
     * [encodePathAllowed] keeps `/`, so the ref's slashes remain path
     * separators. [authoritySlug] is already URL-safe (API-emitted slug).
     */
    public fun build(
        authoritySlug: String,
        ref: String,
    ): String? {
        if (authoritySlug.isEmpty() || ref.isEmpty()) return null
        return "$ORIGIN/a/$authoritySlug/${encodePathAllowed(ref)}"
    }
}

// The Kotlin equivalent of iOS's `CharacterSet.urlPathAllowed`: unreserved
// characters, sub-delimiters, ":", "@", and "/" (so a literal "/" in `ref`
// survives as a path separator rather than being escaped to "%2F"). Hand-
// rolled rather than `android.net.Uri.encode` deliberately — this file needs
// to be testable as a plain JVM unit (no Robolectric, per android-coding-
// standards skill), and `android.net.Uri` is unusable off-device.
private val PATH_ALLOWED_CHARS: Set<Char> =
    (('A'..'Z') + ('a'..'z') + ('0'..'9') + "-._~!$&'()*+,;=:@/".toList()).toSet()

private fun encodePathAllowed(value: String): String =
    buildString {
        for (byte in value.toByteArray(Charsets.UTF_8)) {
            val char = byte.toInt().toChar()
            if (byte >= 0 && char in PATH_ALLOWED_CHARS) {
                append(char)
            } else {
                append('%')
                append("%02X".format(byte.toInt() and 0xFF))
            }
        }
    }
