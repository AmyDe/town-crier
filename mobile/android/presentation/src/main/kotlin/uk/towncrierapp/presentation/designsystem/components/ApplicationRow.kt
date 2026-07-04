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
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import java.time.LocalDate
import java.time.OffsetDateTime

/**
 * A single applications-list row: status badge, address/description, and an
 * 8dp leading unread dot in `tcAmber` when
 * [PlanningApplication.latestUnreadEvent] is present (design-language skill —
 * status is never color-alone, so [StatusBadge] always pairs its color with
 * an icon and label too). Port of iOS `ApplicationRow` (GH#775).
 */
@Composable
public fun ApplicationRow(
    application: PlanningApplication,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .clickable(onClick = onClick)
                .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
    ) {
        UnreadDot(isUnread = application.latestUnreadEvent != null, modifier = Modifier.padding(top = 6.dp))
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
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
private fun UnreadDot(
    isUnread: Boolean,
    modifier: Modifier = Modifier,
) {
    val color = if (isUnread) MaterialTheme.colorScheme.primary else Color.Transparent
    Box(modifier = modifier.size(8.dp).background(color, CircleShape))
}

// Preview-only sample data — cannot reuse :domain's testFixtures from the
// main source set (compose-ui.md: previews can't see test/testFixtures
// source sets), so a small duplicate lives here instead.
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
