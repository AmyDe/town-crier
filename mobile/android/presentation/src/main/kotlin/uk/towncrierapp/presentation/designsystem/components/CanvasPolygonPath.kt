package uk.towncrierapp.presentation.designsystem.components

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Path

/**
 * Builds a closed [Path] through [vertices], mirroring iOS
 * `CustomShapePolygon.path(in:)`. Shared by every Canvas-drawn polygon in
 * the design system — [CustomShapeUpsellGraphic]'s decorative "blob" tile
 * and [uk.towncrierapp.presentation.features.watchzones.ZoneBoundaryThumbnail]'s
 * real zone-boundary thumbnail both build their outline through this same
 * mechanical moveTo/lineTo/close sequence, so it lives here once rather than
 * as two drifting private copies.
 */
internal fun polygonPath(vertices: List<Offset>): Path =
    Path().apply {
        val first = vertices.firstOrNull() ?: return@apply
        moveTo(first.x, first.y)
        vertices.drop(1).forEach { vertex -> lineTo(vertex.x, vertex.y) }
        close()
    }
