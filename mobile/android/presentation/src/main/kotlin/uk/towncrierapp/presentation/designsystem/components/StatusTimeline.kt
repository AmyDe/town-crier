package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.StatusEvent
import uk.towncrierapp.presentation.designsystem.DateDisplayFormatter
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import java.time.LocalDate

/**
 * Renders the CLIENT-synthesized `statusHistory` (max 2 points: submitted,
 * then decided) as a Monzo-activity-feed-style vertical list — a
 * Transaction-feed list per the design-language skill, not a graphical
 * connector timeline. Dates are absolute only via [DateDisplayFormatter],
 * never relative. Port of iOS `StatusTimelineView` (GH#775).
 */
@Composable
public fun StatusTimeline(
    events: List<StatusEvent>,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md)) {
        events.forEach { event ->
            val display = statusDisplay(event.status)
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
            ) {
                Icon(imageVector = display.icon, contentDescription = null, tint = display.color, modifier = Modifier.size(20.dp))
                Column {
                    Text(text = display.label, style = MaterialTheme.typography.bodyLarge)
                    Text(
                        text = DateDisplayFormatter.format(event.date),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
    }
}

@Preview(name = "two points, light")
@Preview(name = "two points, dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun StatusTimelineTwoPointsPreview() {
    TownCrierTheme {
        StatusTimeline(
            events =
                listOf(
                    StatusEvent(ApplicationStatus.Undecided, LocalDate.of(2026, 1, 15)),
                    StatusEvent(ApplicationStatus.Permitted, LocalDate.of(2026, 3, 2)),
                ),
        )
    }
}

@Preview(name = "one point (undecided only)")
@Composable
private fun StatusTimelineOnePointPreview() {
    TownCrierTheme {
        StatusTimeline(events = listOf(StatusEvent(ApplicationStatus.Undecided, LocalDate.of(2026, 1, 15))))
    }
}
