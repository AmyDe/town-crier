package uk.towncrierapp.presentation.features.login

import androidx.annotation.StringRes
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.R

/**
 * Sign in, sign out, and cold-start session restore — port of iOS
 * `LoginViewModel`. [onAuthenticated] fires on every transition into the
 * signed-in state (fresh login or a restored session), which is what the
 * auth-state coordinator hooks to sequence ensure-profile → tier resolution
 * (#549 ordering) and what a future push registrar hooks to flush a pending
 * device-token registration.
 */
public class LoginViewModel(
    private val authService: AuthenticationService,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LoginUiState())
    public val uiState: StateFlow<LoginUiState> = _uiState.asStateFlow()

    public var onAuthenticated: (() -> Unit)? = null

    /** Presents Universal Login. */
    public fun login() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, errorMessageRes = null) }
            try {
                authService.login()
                _uiState.update { it.copy(isLoading = false, isAuthenticated = true) }
                onAuthenticated?.invoke()
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, errorMessageRes = errorMessageRes(e)) }
            }
        }
    }

    /** Clears the current session. */
    public fun logout() {
        viewModelScope.launch {
            try {
                authService.logout()
                _uiState.value = LoginUiState()
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(errorMessageRes = errorMessageRes(e)) }
            }
        }
    }

    /**
     * Checks for an already-stored session (cold start). `currentSession()`
     * itself is responsible for auto-renewing a near-expiry access token
     * (see `Auth0AuthenticationService`), so a non-null result here is
     * already usable — no separate expiry-then-refresh dance is needed.
     */
    public fun checkExistingSession() {
        viewModelScope.launch {
            val existing = authService.currentSession() ?: return@launch
            _uiState.update { it.copy(isAuthenticated = true) }
            onAuthenticated?.invoke()
        }
    }

    @StringRes
    private fun errorMessageRes(error: DomainError): Int =
        when (error) {
            is DomainError.AuthenticationFailed -> R.string.login_error_authentication_failed
            DomainError.SessionExpired -> R.string.login_error_session_expired
            else -> R.string.login_error_generic
        }
}
