package uk.towncrierapp.presentation.features.applicationdetail

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationRepository
import uk.towncrierapp.domain.applications.SavedApplicationRepository
import uk.towncrierapp.domain.auth.DomainError

/**
 * Drives the application detail screen: stale-while-revalidate (the passed-in
 * [initial] row renders instantly; [refresh] replaces it with the by-id
 * fetch's fresh copy, re-entrancy-guarded against a second rapid
 * navigation), and save/unsave with saved-state comparison by the
 * RECONSTRUCTED [uk.towncrierapp.domain.applications.PlanningApplicationId]
 * (tc-jjl4), never a raw uid string. Port of iOS `ApplicationDetailViewModel`
 * (GH#775).
 */
public class ApplicationDetailViewModel(
    private val repository: PlanningApplicationRepository,
    private val savedApplicationRepository: SavedApplicationRepository,
    initial: PlanningApplication,
) : ViewModel() {
    private val _uiState =
        MutableStateFlow(ApplicationDetailUiState(application = initial, authoritySlug = initial.authority.slug))
    public val uiState: StateFlow<ApplicationDetailUiState> = _uiState.asStateFlow()

    private var refreshJob: Job? = null

    /** Refreshes by id in the background. A second call while one is already in flight is ignored, not queued or restarted. */
    public fun refresh() {
        if (refreshJob?.isActive == true) return
        refreshJob =
            viewModelScope.launch {
                _uiState.update { it.copy(isRefreshing = true, error = null) }
                val id = _uiState.value.application.id
                try {
                    val fresh = repository.detail(id.authority, id.name)
                    _uiState.update {
                        it.copy(
                            application = fresh,
                            authoritySlug = fresh.authority.slug,
                            isRefreshing = false,
                        )
                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: DomainError) {
                    _uiState.update { it.copy(isRefreshing = false, error = e) }
                }
            }
    }

    /** Resolves [ApplicationDetailUiState.isSaved] by comparing the RECONSTRUCTED id, never the raw wire uid (tc-jjl4). */
    @Suppress("SwallowedException")
    // Best-effort by design: a failed saved-state check leaves the toggle at
    // its last-known value rather than surfacing an error banner for a
    // background consistency check the user didn't initiate.
    public fun checkSavedState() {
        viewModelScope.launch {
            try {
                val currentId = _uiState.value.application.id
                val isSaved = savedApplicationRepository.savedApplications().any { it.applicationUid == currentId }
                _uiState.update { it.copy(isSaved = isSaved) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                // Best-effort — a failed saved-state check leaves the toggle at its last-known state.
            }
        }
    }

    /** Optimistically flips [ApplicationDetailUiState.isSaved], reverting and surfacing an error on failure. */
    public fun toggleSave() {
        val id = _uiState.value.application.id
        val wasSaved = _uiState.value.isSaved
        _uiState.update { it.copy(isSaved = !wasSaved, error = null) }
        viewModelScope.launch {
            try {
                if (wasSaved) savedApplicationRepository.unsave(id) else savedApplicationRepository.save(id)
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isSaved = wasSaved, error = e) }
            }
        }
    }
}
