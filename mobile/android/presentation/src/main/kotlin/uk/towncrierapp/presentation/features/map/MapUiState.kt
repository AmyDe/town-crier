package uk.towncrierapp.presentation.features.map

import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.map.MapCluster
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.watchzones.WatchZone

/**
 * `MapScreen` state. Port of iOS `MapViewModel`'s published state (GH#776).
 *
 * [stackedApplications] is non-null only while the disambiguation list for an
 * unsplittable cluster (GH#722) is showing — the summary sheet
 * ([selectedApplication]) and the list are never both open (setting one
 * clears the other, see [MapViewModel]). [pendingCameraTarget] is a one-shot
 * effect the Screen reconciles by moving the `GoogleMap` camera and calling
 * [MapViewModel.consumeCameraTarget] — set by a zone switch (seeded
 * whole-zone framing) or a splittable-bubble tap (client-side zoom-in, no
 * server round-trip).
 */
public data class MapUiState(
    val zones: List<WatchZone> = emptyList(),
    val selectedZone: WatchZone? = null,
    val selectedStatusFilter: ApplicationStatus? = null,
    val clusters: List<MapCluster> = emptyList(),
    val isLoading: Boolean = false,
    val hasLoaded: Boolean = false,
    val error: DomainError? = null,
    val selectedApplication: PlanningApplication? = null,
    val isSelectedApplicationSaved: Boolean = false,
    val stackedApplications: List<PlanningApplication>? = null,
    val pendingCameraTarget: MapViewport? = null,
) {
    /** Only once a zone has loaded, so a filter that returns zero clusters doesn't make the chips vanish. */
    public val showStatusFilters: Boolean get() = hasLoaded && error == null && selectedZone != null

    public val showZonePicker: Boolean get() = zones.size > 1

    public val isEmpty: Boolean get() = hasLoaded && clusters.isEmpty() && error == null && !isLoading
}
