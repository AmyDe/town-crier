package uk.towncrierapp.presentation.features.watchzones

import androidx.annotation.StringRes
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.presentation.R

/**
 * Maps a [DomainError] to the inline-error string resource shown on the
 * watch-zone list/editor/preferences screens (compose-ui.md: user-facing
 * copy is a `:presentation` resource, keyed off the sealed outcome, never
 * baked into the ViewModel). [DomainError.InsufficientEntitlement] is listed
 * for exhaustiveness only — those cases route to the paywall placeholder
 * instead of rendering inline (see `WatchZoneEditorViewModel.save`).
 * [DomainError.GeocodingFailed] is likewise listed for exhaustiveness only —
 * this editor's postcode field never produces it (that's the onboarding
 * wizard's postcode step, tc-7ttz); grouped with [DomainError.NotFound]
 * since the copy fits either way.
 */
@StringRes
internal fun watchZoneErrorMessageRes(error: DomainError): Int =
    when (error) {
        DomainError.NetworkUnavailable -> R.string.watch_zone_error_network

        DomainError.NotFound,
        is DomainError.GeocodingFailed,
        -> R.string.watch_zone_error_not_found

        DomainError.SessionExpired -> R.string.login_error_session_expired

        is DomainError.InsufficientEntitlement -> R.string.watch_zone_error_upgrade_required

        is DomainError.ServerError,
        is DomainError.AuthenticationFailed,
        is DomainError.LogoutFailed,
        is DomainError.Unexpected,
        -> R.string.watch_zone_error_generic
    }
