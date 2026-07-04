package uk.towncrierapp.presentation.features.saved

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.SavedApplicationRepository
import uk.towncrierapp.domain.auth.DomainError

/** Drives the Saved tab: the flat cross-zone saved list, with a client-side status filter. Port of iOS `SavedApplicationListViewModel` (GH#775). */
public class SavedListViewModel(
    private val repository: SavedApplicationRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(SavedListUiState())
    public val uiState: StateFlow<SavedListUiState> = _uiState.asStateFlow()

    public fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val saved = repository.savedApplications()
                _uiState.update { it.copy(savedApplications = saved, isLoading = false) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
            }
        }
    }

    /** `null` shows every (non-legacy) saved application. */
    public fun selectFilter(status: ApplicationStatus?) {
        _uiState.update { it.copy(filter = status) }
    }
}
