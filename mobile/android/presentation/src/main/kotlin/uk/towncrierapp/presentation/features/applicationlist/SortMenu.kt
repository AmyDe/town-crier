package uk.towncrierapp.presentation.features.applicationlist

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Sort
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.res.stringResource
import uk.towncrierapp.domain.applications.ApplicationSortOrder
import uk.towncrierapp.presentation.R

/**
 * The top-app-bar sort menu: [availableSorts] already has `distance` filtered
 * out by [ApplicationListUiState.availableSorts] whenever no zone is active —
 * this composable has no opinion on that, it just renders whatever list it's
 * given with a checkmark on the current selection. Split into its own file
 * to keep `ApplicationListScreen.kt` under detekt's per-file function budget.
 */
@Composable
internal fun SortMenu(
    currentSort: ApplicationSortOrder,
    availableSorts: List<ApplicationSortOrder>,
    onSortSelected: (ApplicationSortOrder) -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }
    IconButton(onClick = { expanded = true }) {
        Icon(
            imageVector = Icons.AutoMirrored.Filled.Sort,
            contentDescription = stringResource(R.string.applications_sort_content_description),
        )
    }
    DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
        availableSorts.forEach { sort ->
            DropdownMenuItem(
                text = { Text(sortLabel(sort)) },
                onClick = {
                    onSortSelected(sort)
                    expanded = false
                },
                trailingIcon = {
                    if (sort == currentSort) Icon(imageVector = Icons.Filled.Check, contentDescription = null)
                },
            )
        }
    }
}

@Composable
private fun sortLabel(sort: ApplicationSortOrder): String =
    when (sort) {
        ApplicationSortOrder.DISTANCE -> stringResource(R.string.applications_sort_distance)
        ApplicationSortOrder.NEWEST -> stringResource(R.string.applications_sort_newest)
        ApplicationSortOrder.OLDEST -> stringResource(R.string.applications_sort_oldest)
        ApplicationSortOrder.STATUS -> stringResource(R.string.applications_sort_status)
        ApplicationSortOrder.RECENT_ACTIVITY -> stringResource(R.string.applications_sort_recent_activity)
    }
