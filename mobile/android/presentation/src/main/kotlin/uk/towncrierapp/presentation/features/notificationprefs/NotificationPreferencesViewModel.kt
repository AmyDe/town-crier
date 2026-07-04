package uk.towncrierapp.presentation.features.notificationprefs

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.profile.DigestDay
import uk.towncrierapp.domain.profile.ServerProfile
import uk.towncrierapp.domain.profile.UserPreferences
import uk.towncrierapp.domain.profile.UserProfileRepository

/**
 * Drives the in-app notification preferences screen: global toggles for
 * saved-application push/email, the weekly email digest, and its day
 * (Monday-Sunday). Loads via `GET /v1/me`
 * ([UserProfileRepository.fetchProfile]) — not the `POST /v1/me`
 * ensure-profile call — since only `GET`/`PATCH` return the user's actual
 * saved preferences on the wire; the profile is guaranteed to already exist
 * by the time an authenticated user reaches Settings ([uk.towncrierapp.presentation.auth.AuthCoordinator.onSignedIn]
 * already ensures it). Every setter is optimistic (updates the UI
 * immediately) and PATCHes the FULL five-field body — the server treats
 * them as a set — rolling back to the prior value on failure. Port of iOS
 * `NotificationPreferencesViewModel`.
 */
public class NotificationPreferencesViewModel(
    private val userProfileRepository: UserProfileRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(NotificationPreferencesUiState())
    public val uiState: StateFlow<NotificationPreferencesUiState> = _uiState.asStateFlow()

    /**
     * Loads the server profile. On any failure — including "no profile yet"
     * — every toggle falls back to the API's documented opt-out semantics
     * (the user is treated as opted in until they say otherwise), matching
     * [NotificationPreferencesUiState]'s own defaults.
     */
    public fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            val profile = fetchProfileOrNull()
            _uiState.value = profile?.let(::stateFrom) ?: NotificationPreferencesUiState(isLoading = false)
        }
    }

    public fun setSavedDecisionPush(value: Boolean) {
        persist { it.copy(savedDecisionPush = value) }
    }

    public fun setSavedDecisionEmail(value: Boolean) {
        persist { it.copy(savedDecisionEmail = value) }
    }

    public fun setEmailDigestEnabled(value: Boolean) {
        persist { it.copy(emailDigestEnabled = value) }
    }

    public fun setDigestDay(value: DigestDay) {
        persist { it.copy(digestDay = value) }
    }

    @Suppress("SwallowedException")
    // A fetch failure — including 404 (no profile yet) — degrades to the
    // documented opt-in defaults rather than surfacing an error banner; the
    // user can still change and save preferences from those defaults.
    private suspend fun fetchProfileOrNull(): ServerProfile? =
        try {
            userProfileRepository.fetchProfile()
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            null
        }

    /**
     * Shared persistence path for all four setters: optimistically applies
     * [transform] to the current state, PATCHes the full resulting
     * five-field body, and rolls back to the pre-transform state (with the
     * error attached) on failure.
     */
    private fun persist(transform: (NotificationPreferencesUiState) -> NotificationPreferencesUiState) {
        val previous = _uiState.value
        val next = transform(previous).copy(error = null)
        _uiState.value = next
        viewModelScope.launch {
            try {
                val updated = userProfileRepository.updatePreferences(next.toUserPreferences())
                _uiState.value = stateFrom(updated)
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.value = previous.copy(error = e)
            }
        }
    }
}

private fun stateFrom(profile: ServerProfile): NotificationPreferencesUiState =
    NotificationPreferencesUiState(
        isLoading = false,
        pushEnabled = profile.pushEnabled,
        savedDecisionPush = profile.savedDecisionPush,
        savedDecisionEmail = profile.savedDecisionEmail,
        emailDigestEnabled = profile.emailDigestEnabled,
        digestDay = profile.digestDay,
    )

private fun NotificationPreferencesUiState.toUserPreferences(): UserPreferences =
    UserPreferences(
        pushEnabled = pushEnabled,
        digestDay = digestDay,
        emailDigestEnabled = emailDigestEnabled,
        savedDecisionPush = savedDecisionPush,
        savedDecisionEmail = savedDecisionEmail,
    )
