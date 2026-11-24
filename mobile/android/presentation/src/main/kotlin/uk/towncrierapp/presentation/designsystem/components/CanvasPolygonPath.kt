package uk.towncrierapp.presentation.designsystem.components

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Path

/** Builds a closed [Path] through [vertices]. */
internal fun polygonPath(vertices: List<Offset>): Path =
    Path().apply {
        val first = vertices.firstOrNull() ?: return@apply
        moveTo(first.x, first.y)
        vertices.drop(1).forEach { vertex -> lineTo(vertex.x, vertex.y) }
        close()
    }
