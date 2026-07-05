package uk.towncrierapp.presentation.sharing

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.applications.PlanningApplicationRepository
import uk.towncrierapp.domain.auth.DomainError

/**
 * Resolves a parsed [DeepLink] (from an App Link, dispatched once
 * [PendingLinkHolder] has surfaced it post-auth) into a [DeepLinkResolution]
 * the NavHost layer acts on. Port of iOS `AppCoordinator+Detail`'s
 * `showApplicationDetail(_:)`/`showApplicationDetail(bySlug:ref:)` (GH#782).
 *
 * [onOpenedAlert] fires the review-prompt `openedAlert` signal (GH #628) —
 * ONLY for the legacy by-id path (push taps and `/applications/{uid}` App
 * Links, the "instant-alert payoff moment"). A public share link is
 * deliberately excluded: arriving from a shared web link is not that payoff
 * moment (iOS `AppCoordinator.handleDeepLink`'s `.shareApplication` case
 * omits the signal — a deliberate distinction, not an oversight, ported
 * faithfully here).
 */
public class DeepLinkViewModel(
    private val repository: PlanningApplicationRepository,
    private val onOpenedAlert: suspend () -> Unit = {},
) : ViewModel() {
    private val _uiState = MutableStateFlow(DeepLinkUiState())
    public val uiState: StateFlow<DeepLinkUiState> = _uiState.asStateFlow()

    public fun resolve(deepLink: DeepLink) {
        viewModelScope.launch {
            _uiState.update { it.copy(isResolving = true, error = null) }
            when (deepLink) {
                DeepLink.ApplicationsList ->
                    _uiState.update {
                        it.copy(isResolving = false, resolution = DeepLinkResolution.ShowApplicationsList)
                    }
                is DeepLink.ApplicationDetail -> resolveApplicationDetail(deepLink)
                is DeepLink.ShareApplication -> resolveShareApplication(deepLink)
            }
        }
    }

    private suspend fun resolveApplicationDetail(deepLink: DeepLink.ApplicationDetail) {
        try {
            val application = repository.detail(deepLink.id.authority, deepLink.id.name)
            _uiState.update {
                it.copy(isResolving = false, resolution = DeepLinkResolution.ShowApplication(application))
            }
            onOpenedAlert()
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            _uiState.update { it.copy(isResolving = false, error = e) }
        }
    }

    private suspend fun resolveShareApplication(deepLink: DeepLink.ShareApplication) {
        try {
            val application = repository.detailBySlug(deepLink.authoritySlug, deepLink.ref)
            _uiState.update {
                it.copy(isResolving = false, resolution = DeepLinkResolution.ShowApplication(application))
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            _uiState.update { it.copy(isResolving = false, error = e) }
        }
    }

    /** Clears [DeepLinkUiState.resolution] once the NavHost layer has acted on it — a one-shot effect (compose-ui.md). */
    public fun consumeResolution() {
        _uiState.update { it.copy(resolution = null) }
    }

    /** Clears [DeepLinkUiState.error] once the caller has surfaced it. */
    public fun consumeError() {
        _uiState.update { it.copy(error = null) }
    }
}
