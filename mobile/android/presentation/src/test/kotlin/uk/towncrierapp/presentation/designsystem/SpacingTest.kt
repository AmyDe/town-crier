package uk.towncrierapp.presentation.designsystem

import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.unit.dp
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals

/**
 * Public Notice sharpens the corner-radius scale (epic #848 R5, design-language
 * skill): sm 8dp→3dp, md 12dp→6dp, lg 16dp→12dp. [TownCrierShapes] derives from
 * these constants, so asserting it here catches any accidental hand-edit that
 * would let the two drift apart (android-coding-standards skill).
 */
class SpacingTest {
    @Test
    fun `TownCrierRadius matches the Public Notice sharpened scale`() {
        assertEquals(3.dp, TownCrierRadius.sm)
        assertEquals(6.dp, TownCrierRadius.md)
        assertEquals(12.dp, TownCrierRadius.lg)
        assertEquals(CircleShape, TownCrierRadius.full)
    }

    @Test
    fun `TownCrierShapes derives its corner shapes from TownCrierRadius, not a second hand-edited value`() {
        assertEquals(RoundedCornerShape(TownCrierRadius.sm), TownCrierShapes.extraSmall)
        assertEquals(RoundedCornerShape(TownCrierRadius.md), TownCrierShapes.medium)
        assertEquals(RoundedCornerShape(TownCrierRadius.lg), TownCrierShapes.large)
    }
}
