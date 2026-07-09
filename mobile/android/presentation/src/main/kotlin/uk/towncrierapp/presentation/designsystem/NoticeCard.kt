package uk.towncrierapp.presentation.designsystem

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

/** Thickness of [Modifier.noticeCard]'s top rule — see that doc for context. */
private val NOTICE_CARD_TOP_RULE_THICKNESS = 2.dp

/** Thickness of [Modifier.noticeCard]'s side/bottom border — see that doc for context. */
private val NOTICE_CARD_BORDER_THICKNESS = 1.dp

/**
 * The top rule's colour: `amber` for new/unread content, `onSurface`
 * otherwise. A pure function — mirrors iOS `NoticeCardStyle.topRuleColor` —
 * so the switch is testable without rendering Compose.
 */
internal fun noticeCardTopRuleColor(
    isNew: Boolean,
    onSurface: Color,
    amber: Color,
): Color = if (isNew) amber else onSurface

/**
 * The Public Notice "filed-notice" card treatment (epic #848 R5): [surface]
 * background, a 1dp `outline` border, and a 2dp top rule — `onSurface`
 * normally, `amber` when [isNew] (wired by callers to a card's unread/new
 * signal, e.g. [uk.towncrierapp.domain.applications.PlanningApplication.latestUnreadEvent]).
 * Clipped to [TownCrierShapes.medium]. Port of iOS `NoticeCardStyle` / web
 * `.card` (GH#857).
 *
 * Applied to [uk.towncrierapp.presentation.features.applicationdetail.FieldsCard],
 * application list rows, watch-zone rows, and saved-list rows. NOT applied to
 * [uk.towncrierapp.presentation.features.watchzones.WatchZoneInlineUpsellCard],
 * which keeps its own bespoke amber-bordered treatment (that card's border
 * carries the brand accent — folding it into this neutral-outline treatment
 * would either lose the amber border or break the "one filled-amber
 * container" rationing rule this modifier's neutral border/rule is designed
 * to respect).
 */
@Composable
public fun Modifier.noticeCard(isNew: Boolean = false): Modifier {
    val shape = TownCrierShapes.medium
    val backgroundColor = MaterialTheme.colorScheme.surface
    val borderColor = MaterialTheme.colorScheme.outline
    val ruleColor =
        noticeCardTopRuleColor(
            isNew = isNew,
            onSurface = MaterialTheme.colorScheme.onSurface,
            amber = MaterialTheme.colorScheme.primary,
        )
    return this
        .clip(shape)
        .background(backgroundColor, shape)
        .border(BorderStroke(NOTICE_CARD_BORDER_THICKNESS, borderColor), shape)
        .drawBehind {
            val strokePx = NOTICE_CARD_TOP_RULE_THICKNESS.toPx()
            drawLine(
                color = ruleColor,
                start = Offset(0f, strokePx / 2f),
                end = Offset(size.width, strokePx / 2f),
                strokeWidth = strokePx,
            )
        }
}
