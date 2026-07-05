package uk.towncrierapp.presentation.sharing

import uk.towncrierapp.domain.applications.PlanningApplicationId

/** Navigation targets reachable via App Links or (future, #777) notification taps. Port of iOS `DeepLink`. */
public sealed interface DeepLink {
    public data class ApplicationDetail(val id: PlanningApplicationId) : DeepLink

    /** The Applications tab root - used when an App Link points at `/applications` exactly (e.g. a digest email CTA). */
    public data object ApplicationsList : DeepLink

    /**
     * A public share link `/a/{authoritySlug}/{ref...}` (GH#738 Slice 4 / GH#782).
     * [authoritySlug] is the API-emitted slug and [ref] is the application's
     * full area-prefixed PlanIt name, verbatim (slashes preserved). Resolved
     * into the native detail screen via the anonymous by-slug read.
     */
    public data class ShareApplication(val authoritySlug: String, val ref: String) : DeepLink
}
