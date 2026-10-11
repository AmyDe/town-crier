package uk.towncrierapp.presentation.features.saved

import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.SavedApplication
import uk.towncrierapp.domain.auth.DomainError

/**
 * `SavedListScreen` state. [displayed] drops legacy rows with a `null`
 * payload, narrows by [filter] (client-side only) when set, and is always
 * `savedAt` DESC.
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
