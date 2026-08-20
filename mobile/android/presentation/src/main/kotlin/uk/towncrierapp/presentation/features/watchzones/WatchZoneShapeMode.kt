package uk.towncrierapp.presentation.features.watchzones

/**
 * Which shape the watch-zone editor is currently drawing: a plain circle
 * (radius slider, the original behaviour) or a hand-drawn polygon
 * (GH#1072 Phase 3, tc-v6fo0.3). [CUSTOM] is only reachable when
 * [WatchZoneEditorUiState.allowsCustomBoundary] is true — see
 * [WatchZoneEditorViewModel.onShapeEvent]. Presentation-only: the
 * `:domain` boundary type ([uk.towncrierapp.domain.watchzones.WatchZoneBoundary])
 * has no notion of "mode", only "present or absent".
 */
public enum class WatchZoneShapeMode {
    CIRCLE,
    CUSTOM,
}
