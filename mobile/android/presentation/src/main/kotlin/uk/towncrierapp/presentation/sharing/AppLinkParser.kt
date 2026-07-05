package uk.towncrierapp.presentation.sharing

import uk.towncrierapp.domain.applications.PlanningApplicationId
import java.net.URI
import java.net.URISyntaxException

/**
 * Parses inbound App Link URLs (handed to the app via an `ACTION_VIEW` intent
 * whose data matched one of the manifest's `autoVerify` intent filters) into
 * the in-app [DeepLink] vocabulary. Port of iOS `UniversalLinkParser.parse`
 * (GH#782).
 *
 * Three shapes are recognised:
 * - the public share scheme `/a/{authoritySlug}/{ref...}`, where `ref` is the
 *   application's full area-prefixed PlanIt name, verbatim (slashes preserved
 *   as path separators);
 * - the legacy `/applications/{uid...}` (a specific planning application) -
 *   PlanIt UIDs may contain `/`, so the entire path suffix after
 *   `/applications/` is preserved verbatim;
 * - `/applications` (the root list).
 */
public object AppLinkParser {
    private const val APPLICATIONS_PATH = "/applications"
    private const val SHARE_PREFIX = "/a/"

    /** Parses the full URL string (e.g. `Intent.getDataString()`). Returns `null` for a malformed or unrecognised URL. */
    public fun parse(url: String): DeepLink? {
        val path =
            try {
                URI(url).path
            } catch (e: URISyntaxException) {
                null
            } ?: return null
        return parseShare(path) ?: parseApplications(path)
    }

    /**
     * Parses `/a/{authoritySlug}/{ref...}`: the first segment after `/a/` is
     * the slug, everything after it (verbatim, slashes preserved) is the ref.
     * A bare `/a` or `/a/`, or a slug with no ref, returns `null`. The `/a/`
     * separator is required, so `/afoo` does not match.
     */
    private fun parseShare(path: String): DeepLink? {
        if (!path.startsWith(SHARE_PREFIX)) return null
        val parts = path.substring(SHARE_PREFIX.length).split("/", limit = 2)
        if (parts.size != 2 || parts[0].isEmpty() || parts[1].isEmpty()) return null
        return DeepLink.ShareApplication(authoritySlug = parts[0], ref = parts[1])
    }

    private fun parseApplications(path: String): DeepLink? {
        if (!path.startsWith(APPLICATIONS_PATH)) return null
        val suffix = path.substring(APPLICATIONS_PATH.length)
        if (suffix.isEmpty()) return DeepLink.ApplicationsList
        if (!suffix.startsWith("/")) return null
        val uid = suffix.substring(1)
        if (uid.isEmpty()) return DeepLink.ApplicationsList
        // App Links only carry the uid path segment - split on the first "/" to
        // reconstruct authority + name. A legacy uid without a "/" is treated as
        // name-only with an empty authority; this parser is best-effort, and a
        // URL that doesn't carry an authority fails gracefully at the fetch.
        val components = uid.split("/", limit = 2)
        val authority = if (components.size > 1) components[0] else ""
        val name = if (components.size > 1) components[1] else uid
        return DeepLink.ApplicationDetail(PlanningApplicationId(authority = authority, name = name))
    }
}
