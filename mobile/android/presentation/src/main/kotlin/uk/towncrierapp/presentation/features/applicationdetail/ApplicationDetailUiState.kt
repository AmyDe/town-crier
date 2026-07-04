package uk.towncrierapp.presentation.features.applicationdetail

import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.auth.DomainError

/**
 * `ApplicationDetailScreen` state. [application] starts as the row passed in
 * from the list/saved screen (stale-while-revalidate: rendered instantly,
 * never a blank/loading flash) and is replaced once [ApplicationDetailViewModel.refresh]'s
 * by-id fetch completes. [canShare] is only true once that fetch has supplied
 * an `authoritySlug` — share needs the by-slug identity, which list/saved
 * rows never carry. Port of iOS `ApplicationDetailViewModel`'s published
 * state (GH#775).
 */
public data class ApplicationDetailUiState(
    val application: PlanningApplication,
    val isRefreshing: Boolean = false,
    val isSaved: Boolean = false,
    val authoritySlug: String? = null,
    val error: DomainError? = null,
) {
    public val canShare: Boolean get() = authoritySlug != null
}
