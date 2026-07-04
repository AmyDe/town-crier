package uk.towncrierapp.presentation.features.login

import androidx.annotation.StringRes

/**
 * Login screen state. [errorMessageRes] is a resource id rather than a raw
 * string — user-facing copy stays a `:presentation` resource, never baked
 * into the ViewModel (android-coding-standards: compose-ui.md).
 */
public data class LoginUiState(
    val isLoading: Boolean = false,
    val isAuthenticated: Boolean = false,
    @StringRes val errorMessageRes: Int? = null,
)
