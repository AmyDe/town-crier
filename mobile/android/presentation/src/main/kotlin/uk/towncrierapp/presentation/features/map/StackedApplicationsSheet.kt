package uk.towncrierapp.presentation.features.map

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
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
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.statusDisplay
import java.time.LocalDate

/**
 * The disambiguation list a stacked (unsplittable) cluster tap opens
 * (GH#722, GH#776): every application coincident at that point, each row
 * routing to its own summary sheet via [onSelect]. Presentation-only — no
 * ViewModel dependency — so `MapViewModel.selectFromStack` is the only place
 * that decides what happens next (compose-ui.md: ViewModels never navigate,
 * this composable just emits the tap).
 */
@Composable
internal fun StackedApplicationsSheetContent(
    applications: List<PlanningApplication>,
    onSelect: (PlanningApplication) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(TownCrierSpacing.md)) {
            Text(text = stringResource(R.string.map_stacked_title), style = MaterialTheme.typography.titleLarge)
            Text(
                text = stringResource(R.string.map_stacked_subtitle, applications.size),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        LazyColumn {
            items(applications, key = { it.id.value }) { application ->
                StackedApplicationRow(application = application, onClick = { onSelect(application) })
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            }
        }
    }
}

@Composable
private fun StackedApplicationRow(
    application: PlanningApplication,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(onClick = onClick, modifier = modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
            horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
            verticalAlignment = Alignment.Top,
        ) {
            Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
                Row(horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm)) {
                    val display = statusDisplay(application.status)
                    Text(text = display.label, style = MaterialTheme.typography.labelMedium, color = display.color)
                    Text(
                        text = application.reference,
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                Text(text = application.description, style = MaterialTheme.typography.bodyMedium, maxLines = 2)
                Text(
                    text = application.address,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                )
            }
            Icon(
                imageVector = Icons.AutoMirrored.Filled.KeyboardArrowRight,
                contentDescription = null,
                modifier = Modifier.widthIn(min = TownCrierSpacing.md),
            )
        }
    }
}

private val previewApplications =
    listOf(
        PlanningApplication(
            id = PlanningApplicationId("42", "24/0001"),
            reference = "24/0001",
            authority = LocalAuthority(code = "42", name = "Camden"),
            status = ApplicationStatus.Permitted,
            receivedDate = LocalDate.of(2026, 1, 15),
            description = "Two-storey rear extension",
            address = "1 Example Street, Camden",
        ),
        PlanningApplication(
            id = PlanningApplicationId("42", "24/0002"),
            reference = "24/0002",
            authority = LocalAuthority(code = "42", name = "Camden"),
            status = ApplicationStatus.Undecided,
            receivedDate = LocalDate.of(2026, 2, 1),
            description = "Loft conversion with dormer window",
            address = "1 Example Street, Camden",
        ),
    )

@Preview(name = "light")
@Composable
private fun StackedApplicationsSheetContentPreview() {
    TownCrierTheme {
        StackedApplicationsSheetContent(applications = previewApplications, onSelect = {})
    }
}
