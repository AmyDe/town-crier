package uk.towncrierapp.presentation.designsystem

import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Shapes
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** Town Crier's 4dp-base spacing scale (design-language skill). */
public object TownCrierSpacing {
    public val xs: Dp = 4.dp
    public val sm: Dp = 8.dp
    public val md: Dp = 16.dp
    public val lg: Dp = 24.dp
    public val xl: Dp = 32.dp
    public val xxl: Dp = 48.dp
}

/**
 * Town Crier's corner radius scale. Public Notice (epic #848 R5) sharpens
 * these from the original 8/12/16 to 3/6/12 — crisper corners read as a
 * printed notice rather than a rounded app-store tile (design-language
 * skill). [full] is a capsule, not a dp value — components needing a
 * pill/capsule shape (filter chips ONLY, post-R5 — every other status/badge
 * surface moved to the [uk.towncrierapp.presentation.designsystem.noticeCard]
 * / stamp treatment) use it directly rather than through [TownCrierShapes],
 * which has no "full" slot.
 */
public object TownCrierRadius {
    public val sm: Dp = 3.dp
    public val md: Dp = 6.dp
    public val lg: Dp = 12.dp
    public val full: Shape = CircleShape
}

/** Overrides Material 3's default corner radii per the design-language skill. */
internal val TownCrierShapes =
    Shapes(
        extraSmall = RoundedCornerShape(TownCrierRadius.sm),
        medium = RoundedCornerShape(TownCrierRadius.md),
        large = RoundedCornerShape(TownCrierRadius.lg),
    )
