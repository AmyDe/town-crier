package uk.towncrierapp.presentation.features.applicationlist

import androidx.annotation.StringRes
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.R

/** Maps a [DomainError] to the inline-error string resource shared by the list, saved and detail screens. */
@StringRes
internal fun applicationErrorMessageRes(error: DomainError): Int =
    when (error) {
        DomainError.NetworkUnavailable -> R.string.applications_error_network

        DomainError.SessionExpired -> R.string.login_error_session_expired

        DomainError.NotFound,
        is DomainError.InsufficientEntitlement,
        is DomainError.ServerError,
        is DomainError.AuthenticationFailed,
        is DomainError.LogoutFailed,
        is DomainError.Unexpected,
        is DomainError.GeocodingFailed,
        -> R.string.applications_error_generic
    }
