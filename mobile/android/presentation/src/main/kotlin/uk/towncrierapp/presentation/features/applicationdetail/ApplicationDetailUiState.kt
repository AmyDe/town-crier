package uk.towncrierapp.presentation.features.applicationdetail

import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.auth.DomainError

/**
 * [application] starts as the row passed in from the list or saved screen and is replaced once
 * [ApplicationDetailViewModel.refresh]'s by-id fetch completes. [canShare] is true only once that fetch has
 * supplied an `authoritySlug`, which list and saved rows never carry.
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
