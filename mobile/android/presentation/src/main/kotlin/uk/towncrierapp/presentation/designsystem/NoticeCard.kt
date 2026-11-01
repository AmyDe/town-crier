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

private val NOTICE_CARD_TOP_RULE_THICKNESS = 2.dp

private val NOTICE_CARD_BORDER_THICKNESS = 1.dp

/** The top rule's colour: `amber` for new/unread content, `onSurface` otherwise. */
internal fun noticeCardTopRuleColor(
    isNew: Boolean,
    onSurface: Color,
    amber: Color,
): Color = if (isNew) amber else onSurface

/**
 * The Public Notice card treatment: [surface] background, a 1dp `outline`
 * border, and a 2dp top rule (`onSurface` normally, `amber` when [isNew]),
 * clipped to [TownCrierShapes.medium].
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
