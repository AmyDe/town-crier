package uk.towncrierapp.presentation.features.login

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.auth.anAuthSession
import uk.towncrierapp.presentation.MainDispatcherExtension
import uk.towncrierapp.presentation.R
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `login()`/`logout()`/`checkExistingSession()` — port of iOS
 * `LoginViewModelTests`. Error copy is verbatim iOS wording, resolved as
 * string resources (compose-ui.md), asserted here by resource id.
 */
@ExtendWith(MainDispatcherExtension::class)
class LoginViewModelTest {
    @Test
    fun `initial state is unauthenticated with no error`() {
        val viewModel = LoginViewModel(FakeAuthenticationService())

        val state = viewModel.uiState.value

        assertFalse(state.isAuthenticated)
        assertFalse(state.isLoading)
        assertNull(state.errorMessageRes)
    }

    @Test
    fun `login succeeds, becomes authenticated, and fires onAuthenticated once`() {
        val authService = FakeAuthenticationService()
        val viewModel = LoginViewModel(authService)
        var authenticatedCalls = 0
        viewModel.onAuthenticated = { authenticatedCalls++ }

        viewModel.login()

        assertTrue(viewModel.uiState.value.isAuthenticated)
        assertFalse(viewModel.uiState.value.isLoading)
        assertEquals(1, authenticatedCalls)
        assertEquals(1, authService.loginCalls.size)
    }

    @Test
    fun `login failure surfaces the sign-in-failed caption and does not authenticate`() {
        val authService = FakeAuthenticationService().apply { loginResult = Result.failure(DomainError.AuthenticationFailed("cancelled")) }
        val viewModel = LoginViewModel(authService)
        var authenticatedCalls = 0
        viewModel.onAuthenticated = { authenticatedCalls++ }

        viewModel.login()

        assertFalse(viewModel.uiState.value.isAuthenticated)
        assertEquals(R.string.login_error_authentication_failed, viewModel.uiState.value.errorMessageRes)
        assertEquals(0, authenticatedCalls)
    }

    @Test
    fun `a session-expired login failure surfaces the session-expired caption`() {
        val authService = FakeAuthenticationService().apply { loginResult = Result.failure(DomainError.SessionExpired) }
        val viewModel = LoginViewModel(authService)

        viewModel.login()

        assertEquals(R.string.login_error_session_expired, viewModel.uiState.value.errorMessageRes)
    }

    @Test
    fun `logout resets to the signed-out state`() {
        val authService = FakeAuthenticationService()
        val viewModel = LoginViewModel(authService)
        viewModel.login()

        viewModel.logout()

        assertFalse(viewModel.uiState.value.isAuthenticated)
        assertEquals(1, authService.logoutCalls.size)
    }

    @Test
    fun `checkExistingSession authenticates and fires onAuthenticated when a session is already stored`() {
        val authService = FakeAuthenticationService(currentSessionResult = anAuthSession())
        val viewModel = LoginViewModel(authService)
        var authenticatedCalls = 0
        viewModel.onAuthenticated = { authenticatedCalls++ }

        viewModel.checkExistingSession()

        assertTrue(viewModel.uiState.value.isAuthenticated)
        assertEquals(1, authenticatedCalls)
    }

    @Test
    fun `checkExistingSession leaves the signed-out state alone when there is no stored session`() {
        val authService = FakeAuthenticationService(currentSessionResult = null)
        val viewModel = LoginViewModel(authService)
        var authenticatedCalls = 0
        viewModel.onAuthenticated = { authenticatedCalls++ }

        viewModel.checkExistingSession()

        assertFalse(viewModel.uiState.value.isAuthenticated)
        assertEquals(0, authenticatedCalls)
    }
}
