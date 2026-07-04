package uk.towncrierapp.presentation.features.onboarding

import androidx.annotation.StringRes
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.R

/**
 * Maps a postcode-step [DomainError] to its inline-error string resource
 * (compose-ui.md: user-facing copy is a `:presentation` resource, keyed off
 * the sealed outcome). [DomainError.GeocodingFailed] is the local
 * format-validation failure (never reached the network); every other case
 * came back from the geocoder itself.
 */
@StringRes
internal fun onboardingPostcodeErrorMessageRes(error: DomainError): Int =
    when (error) {
        is DomainError.GeocodingFailed -> R.string.onboarding_postcode_error_invalid

        DomainError.NotFound -> R.string.onboarding_postcode_error_not_found

        DomainError.NetworkUnavailable -> R.string.onboarding_postcode_error_network

        DomainError.SessionExpired -> R.string.login_error_session_expired

        is DomainError.InsufficientEntitlement,
        is DomainError.ServerError,
        is DomainError.AuthenticationFailed,
        is DomainError.LogoutFailed,
        is DomainError.Unexpected,
        -> R.string.onboarding_postcode_error_generic
    }
