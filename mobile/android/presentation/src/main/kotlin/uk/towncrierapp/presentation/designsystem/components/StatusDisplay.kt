package uk.towncrierapp.presentation.designsystem.components

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.HelpOutline
import androidx.compose.material.icons.automirrored.filled.Undo
import androidx.compose.material.icons.filled.Cancel
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Schedule
import androidx.compose.material.icons.filled.Warning
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.wireValue
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/** The resolved label/color/icon for one [ApplicationStatus] — feeds [StatusBadge] wherever a status renders. Port of iOS's status display mapping (GH#775). */
public data class StatusDisplay(
    public val label: String,
    public val color: Color,
    public val icon: ImageVector,
)

/**
 * The exact status -> (label, token, icon) table from the design spec: the
 * six known/actionable statuses get a friendly label and their own status
 * color; [ApplicationStatus.Unresolved]/[ApplicationStatus.Referred]/
 * [ApplicationStatus.NotAvailable] display AS-IS (their own wire value,
 * server-supplied dynamic text — not a hardcoded UI string) in
 * `tcTextTertiary`; a genuinely unrecognised [ApplicationStatus.Unknown]
 * value falls back to the literal word "Unknown", also in `tcTextTertiary`.
 */
@Composable
public fun statusDisplay(status: ApplicationStatus): StatusDisplay {
    val colors = TownCrierTheme.colors
    return when (status) {
        ApplicationStatus.Undecided -> {
            StatusDisplay(
                stringResource(R.string.application_status_pending),
                colors.statusPending,
                Icons.Filled.Schedule,
            )
        }

        ApplicationStatus.Permitted -> {
            StatusDisplay(
                stringResource(R.string.application_status_permitted),
                colors.statusPermitted,
                Icons.Filled.CheckCircle,
            )
        }

        ApplicationStatus.Conditions -> {
            StatusDisplay(
                stringResource(R.string.application_status_conditions),
                colors.statusConditions,
                Icons.Filled.CheckCircle,
            )
        }

        ApplicationStatus.Rejected -> {
            StatusDisplay(
                stringResource(R.string.application_status_rejected),
                colors.statusRejected,
                Icons.Filled.Cancel,
            )
        }

        ApplicationStatus.Withdrawn -> {
            StatusDisplay(
                stringResource(R.string.application_status_withdrawn),
                colors.statusWithdrawn,
                Icons.AutoMirrored.Filled.Undo,
            )
        }

        ApplicationStatus.Appealed -> {
            StatusDisplay(
                stringResource(R.string.application_status_appealed),
                colors.statusAppealed,
                Icons.Filled.Warning,
            )
        }

        ApplicationStatus.Unresolved, ApplicationStatus.Referred, ApplicationStatus.NotAvailable -> {
            StatusDisplay(status.wireValue, colors.textTertiary, Icons.AutoMirrored.Filled.HelpOutline)
        }

        is ApplicationStatus.Unknown -> {
            StatusDisplay(
                stringResource(R.string.application_status_unknown),
                colors.textTertiary,
                Icons.AutoMirrored.Filled.HelpOutline,
            )
        }
    }
}
