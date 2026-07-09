package uk.towncrierapp.presentation.designsystem

import androidx.compose.ui.graphics.Color
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * [noticeCardTopRuleColor] is the pure decision behind [Modifier.noticeCard]'s
 * 2dp top rule (epic #848 R5, "filed-notice" card treatment): `onSurface`
 * normally, `amber` when the card represents new/unread content. Extracted
 * as a standalone function — mirroring iOS `NoticeCardStyle.topRuleColor` —
 * so the switch is testable without rendering Compose (android-coding-
 * standards skill, testing.md: "what not to test" carve-out for layout/
 * rendering doesn't apply to plain decision logic like this).
 */
class NoticeCardTest {
    @Test
    fun `top rule is amber when the card is new`() {
        assertEquals(Color.Red, noticeCardTopRuleColor(isNew = true, onSurface = Color.Blue, amber = Color.Red))
    }

    @Test
    fun `top rule is onSurface when the card is not new`() {
        assertEquals(Color.Blue, noticeCardTopRuleColor(isNew = false, onSurface = Color.Blue, amber = Color.Red))
    }
}
