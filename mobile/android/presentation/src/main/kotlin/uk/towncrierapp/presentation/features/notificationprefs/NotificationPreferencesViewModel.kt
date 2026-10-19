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
 * Global notification preferences. Loads via `GET /v1/me`, not the `POST /v1/me`
 * ensure-profile call, because only `GET`/`PATCH` return the saved preferences.
 * Every setter is optimistic and PATCHes the full five-field body (the server
 * treats them as a set), rolling back on failure.
 */
public class NotificationPreferencesViewModel(
    private val userProfileRepository: UserProfileRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(NotificationPreferencesUiState())
    public val uiState: StateFlow<NotificationPreferencesUiState> = _uiState.asStateFlow()

    /**
     * Loads the server profile. Any failure, including 404 (no profile yet), falls
     * back to the opt-in defaults of [NotificationPreferencesUiState] rather than
     * surfacing an error.
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
    private suspend fun fetchProfileOrNull(): ServerProfile? =
        try {
            userProfileRepository.fetchProfile()
        } catch (e: CancellationException) {
            throw e
        } catch (e: DomainError) {
            null
        }

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
