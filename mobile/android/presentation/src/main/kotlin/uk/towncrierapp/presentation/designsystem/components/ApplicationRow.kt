package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.LatestUnreadEvent
import uk.towncrierapp.domain.applications.LocalAuthority
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.presentation.designsystem.DateDisplayFormatter
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.noticeCard
import java.time.LocalDate
import java.time.OffsetDateTime

/** A single applications-list row; unread when [PlanningApplication.latestUnreadEvent] is present. */
@Composable
public fun ApplicationRow(
    application: PlanningApplication,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val isUnread = application.latestUnreadEvent != null
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .noticeCard(isNew = isUnread)
                .clickable(onClick = onClick)
                .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
    ) {
        UnreadDot(isUnread = isUnread, modifier = Modifier.padding(top = 6.dp))
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
            DocHeaderStrip(reference = application.reference, receivedDate = application.receivedDate)
            val display = statusDisplay(application.status)
            StatusBadge(label = display.label, color = display.color, icon = display.icon)
            Text(text = application.address, style = MaterialTheme.typography.titleMedium)
            Text(
                text = application.description,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun DocHeaderStrip(
    reference: String,
    receivedDate: LocalDate,
    modifier: Modifier = Modifier,
) {
    Row(modifier = modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(text = reference, style = TownCrierTheme.mono, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(
            text = DateDisplayFormatter.format(receivedDate),
            style = TownCrierTheme.mono,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun UnreadDot(
    isUnread: Boolean,
    modifier: Modifier = Modifier,
) {
    val color = if (isUnread) MaterialTheme.colorScheme.primary else Color.Transparent
    Box(modifier = modifier.size(8.dp).background(color, CircleShape))
}

// Previews cannot see :domain's testFixtures from the main source set.
private val previewApplication =
    PlanningApplication(
        id = PlanningApplicationId("42", "24/0001"),
        reference = "24/0001",
        authority = LocalAuthority(code = "42", name = "Camden"),
        status = ApplicationStatus.Undecided,
        receivedDate = LocalDate.of(2026, 1, 15),
        description = "Two-storey rear extension with roof lantern",
        address = "1 Example Street, Camden, London",
    )

@Preview(name = "unread, light")
@Preview(name = "unread, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun ApplicationRowUnreadPreview() {
    TownCrierTheme {
        ApplicationRow(
            application =
                previewApplication.copy(
                    latestUnreadEvent = LatestUnreadEvent(type = "NewApplication", createdAt = OffsetDateTime.now()),
                ),
            onClick = {},
        )
    }
}

@Preview(name = "read")
@Composable
private fun ApplicationRowReadPreview() {
    TownCrierTheme {
        ApplicationRow(application = previewApplication, onClick = {})
    }
}
