package uk.towncrierapp.presentation.features.watchzones

import uk.towncrierapp.domain.watchzones.Coordinate

/**
 * Every shape-mode/polygon-drawing interaction the editor screen can raise,
 * routed through the single [WatchZoneEditorViewModel.onShapeEvent] entry
 * point (GH#1072 Phase 3). One dispatch method rather than five separate
 * ones keeps the ViewModel under detekt's per-class function budget without
 * losing call-site clarity — each event still names exactly what happened.
 */
public sealed interface WatchZoneShapeEvent {
    /** The user switched between [WatchZoneShapeMode.CIRCLE] and [WatchZoneShapeMode.CUSTOM]. */
    public data class ModeSelected(
        public val mode: WatchZoneShapeMode,
    ) : WatchZoneShapeEvent

    /** A map tap landed away from the first vertex — add it as the polygon's next point. */
    public data class VertexAdded(
        public val coordinate: Coordinate,
    ) : WatchZoneShapeEvent

    /** A drag on vertex [index] ended at [coordinate]. */
    public data class VertexMoved(
        public val index: Int,
        public val coordinate: Coordinate,
    ) : WatchZoneShapeEvent

    /** The first vertex was tapped with enough vertices down to close the ring. */
    public data object PolygonClosed : WatchZoneShapeEvent

    /** The user asked to undo their last drawing action. */
    public data object UndoRequested : WatchZoneShapeEvent
}
