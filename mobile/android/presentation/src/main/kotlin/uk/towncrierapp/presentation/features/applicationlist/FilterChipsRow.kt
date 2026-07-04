package uk.towncrierapp.presentation.features.applicationlist

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import uk.towncrierapp.domain.applications.ApplicationFilter
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.components.CapsuleChip

private val FILTERABLE_STATUSES =
    listOf(
        ApplicationStatus.Undecided,
        ApplicationStatus.Permitted,
        ApplicationStatus.Conditions,
        ApplicationStatus.Rejected,
        ApplicationStatus.Withdrawn,
        ApplicationStatus.Appealed,
    )

/**
 * The All/6-status/Unread chip row — single-select, status vs unread
 * mutually exclusive BY [ApplicationFilter]'s shape (see the type itself),
 * not a runtime check here. Split into its own file to keep
 * `ApplicationListScreen.kt` under detekt's per-file function budget.
 */
@Composable
internal fun FilterChipsRow(
    filter: ApplicationFilter,
    unreadCount: Int,
    onFilterSelected: (ApplicationFilter) -> Unit,
    modifier: Modifier = Modifier,
) {
    LazyRow(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md),
    ) {
        item {
            CapsuleChip(
                label = stringResource(R.string.applications_filter_all),
                selected = filter == ApplicationFilter.All,
                onClick = { onFilterSelected(ApplicationFilter.All) },
            )
        }
        items(FILTERABLE_STATUSES) { status ->
            CapsuleChip(
                label = statusFilterLabel(status),
                selected = filter == ApplicationFilter.Status(status),
                onClick = { onFilterSelected(ApplicationFilter.Status(status)) },
            )
        }
        item {
            CapsuleChip(
                label = "${stringResource(R.string.applications_filter_unread)} ($unreadCount)",
                selected = filter == ApplicationFilter.Unread,
                onClick = { onFilterSelected(ApplicationFilter.Unread) },
            )
        }
    }
}

@Composable
private fun statusFilterLabel(status: ApplicationStatus): String =
    when (status) {
        ApplicationStatus.Undecided -> stringResource(R.string.application_status_pending)
        ApplicationStatus.Permitted -> stringResource(R.string.application_status_permitted)
        ApplicationStatus.Conditions -> stringResource(R.string.application_status_conditions)
        ApplicationStatus.Rejected -> stringResource(R.string.application_status_rejected)
        ApplicationStatus.Withdrawn -> stringResource(R.string.application_status_withdrawn)
        ApplicationStatus.Appealed -> stringResource(R.string.application_status_appealed)
        else -> stringResource(R.string.application_status_unknown)
    }
