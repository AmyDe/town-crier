package uk.towncrierapp.presentation.features.applicationdetail

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextStyle
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.DateDisplayFormatter
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.noticeCard

/**
 * The detail screen's address/reference/authority/received-date card, styled
 * as a Public Notice "filed notice" ([noticeCard], epic #848 R5). Split into
 * its own file to keep `ApplicationDetailScreen.kt` under detekt's per-file
 * function budget. Reference and received-date use [TownCrierTheme.mono] —
 * the two metadata fields the mono role targets — the rest stay body text.
 * Port of iOS `ApplicationDetailView.detailCard`.
 */
@Composable
internal fun FieldsCard(
    application: PlanningApplication,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxWidth().noticeCard().padding(TownCrierSpacing.md),
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
    ) {
        FieldRow(label = stringResource(R.string.application_detail_address_label), value = application.address)
        FieldRow(
            label = stringResource(R.string.application_detail_reference_label),
            value = application.reference,
            valueStyle = TownCrierTheme.mono,
        )
        FieldRow(
            label = stringResource(R.string.application_detail_authority_label),
            value = application.authority.name,
        )
        FieldRow(
            label = stringResource(R.string.application_detail_received_label),
            value = DateDisplayFormatter.format(application.receivedDate),
            valueStyle = TownCrierTheme.mono,
        )
    }
}

@Composable
private fun FieldRow(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
    valueStyle: TextStyle = MaterialTheme.typography.bodyMedium,
) {
    Row(modifier = modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(text = value, style = valueStyle)
    }
}
