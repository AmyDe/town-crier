package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/** Gap between the masthead's two rules — see [Masthead] doc. */
private val MASTHEAD_RULE_GAP = 2.dp

/** Thickness of the masthead's top (heavier) rule — see [Masthead] doc. */
private val MASTHEAD_RULE_THICK = 2.5.dp

/** Thickness of the masthead's bottom (lighter) rule — see [Masthead] doc. */
private val MASTHEAD_RULE_THIN = 1.dp

/** Letter-spacing of the masthead's small-caps wordmark — see [Masthead] doc. */
private val MASTHEAD_LETTER_SPACING = 0.1.em

/**
 * The Public Notice masthead (epic #848 R5): a small-caps [title] over a
 * double rule (2.5dp then 1dp, with a small gap), echoing a printed notice's
 * banner. Used on top-level screen titles — the Applications feed, Watch
 * Zones, and Saved screens.
 *
 * Rendered as ordinary scrollable content, not the app bar's title — the
 * `TopAppBar` title stays in place for back-button/VoiceOver correctness;
 * this is the branded content banner underneath it. Port of iOS
 * `MastheadView` (GH#857).
 */
@Composable
public fun Masthead(
    title: String,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm)
                .semantics(mergeDescendants = true) { heading() },
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs),
    ) {
        Text(
            text = title.uppercase(),
            style = MaterialTheme.typography.titleMedium.copy(letterSpacing = MASTHEAD_LETTER_SPACING),
            color = MaterialTheme.colorScheme.onSurface,
        )
        Column(verticalArrangement = Arrangement.spacedBy(MASTHEAD_RULE_GAP)) {
            HorizontalDivider(thickness = MASTHEAD_RULE_THICK, color = MaterialTheme.colorScheme.onSurface)
            HorizontalDivider(thickness = MASTHEAD_RULE_THIN, color = MaterialTheme.colorScheme.onSurface)
        }
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun MastheadPreview() {
    TownCrierTheme {
        Masthead(title = "Applications")
    }
}
