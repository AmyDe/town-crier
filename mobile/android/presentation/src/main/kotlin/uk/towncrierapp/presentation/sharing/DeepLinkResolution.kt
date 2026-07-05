package uk.towncrierapp.presentation.sharing

import uk.towncrierapp.domain.applications.PlanningApplication

/** What the NavHost layer should do once a [DeepLink] has been resolved against the repository. */
public sealed interface DeepLinkResolution {
    /** Navigate to the Applications tab and push the detail destination for [application]. */
    public data class ShowApplication(
        val application: PlanningApplication,
    ) : DeepLinkResolution

    /** Navigate to the Applications tab root. */
    public data object ShowApplicationsList : DeepLinkResolution
}
