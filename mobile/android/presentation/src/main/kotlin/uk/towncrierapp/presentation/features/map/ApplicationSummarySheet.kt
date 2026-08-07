package uk.towncrierapp.presentation.features.map

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.Place
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.LocalAuthority
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.DateDisplayFormatter
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton
import uk.towncrierapp.presentation.designsystem.components.StatusBadge
import uk.towncrierapp.presentation.designsystem.components.statusDisplay
import java.time.LocalDate

/**
 * The bottom sheet a single-pin (or disambiguation-list-row) tap opens:
 * status badge, save toggle, reference, description, address, received
 * date, and "View full details". Same field set as iOS
 * `ApplicationSummarySheet` (GH#776).
 */
@Composable
internal fun ApplicationSummarySheetContent(
    application: PlanningApplication,
    isSaved: Boolean,
    onSaveToggleClick: () -> Unit,
    onViewFullDetailsClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxWidth().padding(TownCrierSpacing.md),
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        ) {
            val display = statusDisplay(application.status)
            StatusBadge(label = display.label, color = display.color, icon = display.icon)
            Text(
                text = application.reference,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.weight(1f))
            IconButton(onClick = onSaveToggleClick) {
                Icon(
                    imageVector = if (isSaved) Icons.Filled.Bookmark else Icons.Filled.BookmarkBorder,
                    contentDescription =
                        stringResource(
                            if (isSaved) {
                                R.string.application_detail_unsave_content_description
                            } else {
                                R.string.application_detail_save_content_description
                            },
                        ),
                )
            }
        }

        Text(text = application.description, style = MaterialTheme.typography.titleMedium)

        Row(horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
            Icon(imageVector = Icons.Filled.Place, contentDescription = null)
            Text(
                text = application.address,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        Text(
            text =
                stringResource(
                    R.string.map_summary_received_date,
                    DateDisplayFormatter.format(application.receivedDate),
                ),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )

        PrimaryButton(
            text = stringResource(R.string.map_summary_view_full_details),
            onClick = onViewFullDetailsClick,
        )
    }
}

// Preview-only sample data — cannot reuse :domain's testFixtures from the main source set (compose-ui.md).
private val previewApplication =
    PlanningApplication(
        id = PlanningApplicationId("42", "24/0001"),
        reference = "24/0001",
        authority = LocalAuthority(code = "42", name = "Camden"),
        status = ApplicationStatus.Permitted,
        receivedDate = LocalDate.of(2026, 1, 15),
        description = "Two-storey rear extension with roof lantern",
        address = "1 Example Street, Camden, London",
    )

@Preview(name = "light")
@Composable
private fun ApplicationSummarySheetContentPreview() {
    TownCrierTheme {
        ApplicationSummarySheetContent(
            application = previewApplication,
            isSaved = true,
            onSaveToggleClick = {},
            onViewFullDetailsClick = {},
        )
    }
}
