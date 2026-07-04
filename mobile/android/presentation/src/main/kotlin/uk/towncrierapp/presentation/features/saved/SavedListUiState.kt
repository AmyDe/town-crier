package uk.towncrierapp.presentation.features.saved

import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.SavedApplication
import uk.towncrierapp.domain.auth.DomainError

/**
 * `SavedListScreen` state. [displayed] is the presentation-ready projection
 * of [savedApplications]: legacy rows with a `null` payload are dropped
 * entirely (never shown as an empty/placeholder row), [filter] (client-side
 * only — the endpoint itself is unfiltered) narrows by status when set, and
 * the result is always `savedAt` DESC. Port of iOS
 * `SavedApplicationListViewModel`'s published state (GH#775).
 */
public data class SavedListUiState(
    val savedApplications: List<SavedApplication> = emptyList(),
    val filter: ApplicationStatus? = null,
    val isLoading: Boolean = false,
    val error: DomainError? = null,
) {
    public val displayed: List<SavedApplication>
        get() =
            savedApplications
                .filter { it.application != null }
                .filter { filter == null || it.application?.status == filter }
                .sortedByDescending { it.savedAt }
}
