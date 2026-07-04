package uk.towncrierapp.presentation.features.applicationlist

import androidx.annotation.StringRes
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.R

/**
 * Maps a [DomainError] to the inline-error string resource shared by the
 * three applications-browsing screens (list/saved/detail) — `internal` is
 * module-wide in Kotlin, so this single mapping is reused across those
 * feature packages rather than duplicated per screen (compose-ui.md:
 * user-facing copy is a `:presentation` resource, keyed off the sealed
 * outcome).
 */
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
