package uk.towncrierapp.presentation.features.applicationdetail

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.DateDisplayFormatter
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing

/**
 * The detail screen's address/reference/authority/received-date card. Split
 * into its own file to keep `ApplicationDetailScreen.kt` under detekt's
 * per-file function budget.
 */
@Composable
internal fun FieldsCard(
    application: PlanningApplication,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        Column(
            modifier = Modifier.padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        ) {
            FieldRow(label = stringResource(R.string.application_detail_address_label), value = application.address)
            FieldRow(label = stringResource(R.string.application_detail_reference_label), value = application.reference)
            FieldRow(
                label = stringResource(R.string.application_detail_authority_label),
                value = application.authority.name,
            )
            FieldRow(
                label = stringResource(R.string.application_detail_received_label),
                value = DateDisplayFormatter.format(application.receivedDate),
            )
        }
    }
}

@Composable
private fun FieldRow(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
) {
    Row(modifier = modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(text = value, style = MaterialTheme.typography.bodyMedium)
    }
}
