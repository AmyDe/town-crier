package uk.towncrierapp.presentation.features.map

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.PlanningApplicationRepository
import uk.towncrierapp.domain.applications.SavedApplicationRepository
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.map.MapCluster
import uk.towncrierapp.domain.map.MapPreferencesStore
import uk.towncrierapp.domain.map.MapViewport
import uk.towncrierapp.domain.map.zoomedInto
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneRepository

/**
 * Drives the Map tab: server-side cluster rendering for the current
 * viewport, debounced camera-idle refetch, zone/status filtering, and tap
 * routing (single pin -> summary, splittable bubble -> client zoom-in,
 * stacked cell -> disambiguation list). Port of iOS `MapViewModel` (GH#776).
 *
 * The point-read helpers behind [onClusterTapped] ([pointReadOrNull],
 * [pointReadAllOrNull]) and the zone-resolution helper behind [load]
 * ([resolveInitialZone]) are top-level functions in this file rather than
 * members — they need only the values already in scope at their one call
 * site, and keeping them out of the class body is what keeps this class
 * under detekt's member-count budget without duplicating logic per caller.
 */
public class MapViewModel(
    private val repository: PlanningApplicationRepository,
    private val watchZoneRepository: WatchZoneRepository,
    private val savedApplicationRepository: SavedApplicationRepository,
    private val mapPreferencesStore: MapPreferencesStore,
    private val debounceMillis: Long = DEBOUNCE_MILLIS,
) : ViewModel() {
    private val _uiState = MutableStateFlow(MapUiState())
    public val uiState: StateFlow<MapUiState> = _uiState.asStateFlow()

    /** The viewport/zoom of the most recent cluster fetch, so a status-chip change or a bubble zoom-in can reuse the same visible rect. */
    private var lastViewport: MapViewport? = null
    private var lastZoom: Int? = null

    /**
     * The in-flight (or pending) cluster fetch — [onCameraIdle], [selectZone], and
     * [applyStatusFilter] each cancel this before launching their own replacement via
     * [launchFetch]. That keeps exactly one fetch alive at a time, so the last
     * *request* always wins the map, never whichever happens to be the last to
     * *complete* (e.g. a debounced pan-fetch for the previous zone landing after an
     * immediate zone-switch fetch, or two rapid zone switches racing each other).
     */
    private var fetchJob: Job? = null

    public fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val zones = watchZoneRepository.zones()
                val selected = resolveInitialZone(mapPreferencesStore, zones)
                _uiState.update { it.copy(zones = zones, selectedZone = selected) }
                if (selected != null) {
                    val (viewport, zoom) = MapViewport.initial(selected.centre, selected.radiusMetres)
                    lastViewport = viewport
                    lastZoom = zoom
                    fetchClustersAndUpdateState(repository, _uiState, viewport, zoom)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(error = e) }
            }
            _uiState.update { it.copy(isLoading = false, hasLoaded = true) }
        }
    }

    /** Camera-idle events drive a ~250ms debounced refetch (coalescing to the latest viewport), never a per-frame fetch. */
    public fun onCameraIdle(
        viewport: MapViewport,
        rawZoom: Double,
    ) {
        val zoom = MapViewport.clampZoom(rawZoom)
        fetchJob?.cancel()
        fetchJob =
            launchFetch(
                scope = viewModelScope,
                debounceMillis = debounceMillis,
                debounce = true,
                onViewportSettled = {
                    lastViewport = viewport
                    lastZoom = zoom
                },
            ) {
                fetchClustersAndUpdateState(repository, _uiState, viewport, zoom)
            }
    }

    /** Switches the active zone: persists the choice, clears the status filter (server/iOS parity), and refetches immediately — never debounced. */
    public fun selectZone(zone: WatchZone) {
        _uiState.update { it.copy(selectedZone = zone, selectedStatusFilter = null, clusters = emptyList()) }
        // Best-effort, independent of the fetch below: cancelling fetchJob (here or from
        // a later call) must never take this write down with it, and a slow/failed write
        // must never delay or block the map from refetching.
        viewModelScope.launch { mapPreferencesStore.writeLastSelectedZoneId(zone.id) }
        val (viewport, zoom) = MapViewport.initial(zone.centre, zone.radiusMetres)
        _uiState.update { it.copy(pendingCameraTarget = viewport) }
        fetchJob?.cancel()
        fetchJob =
            launchFetch(
                scope = viewModelScope,
                debounceMillis = debounceMillis,
                debounce = false,
                onViewportSettled = {
                    lastViewport = viewport
                    lastZoom = zoom
                },
            ) {
                fetchClustersAndUpdateState(repository, _uiState, viewport, zoom)
            }
    }

    /** Applies a status filter chip by refetching the current viewport's clusters server-side (`status=`) — never by filtering a held set. */
    public fun applyStatusFilter(status: ApplicationStatus?) {
        _uiState.update { it.copy(selectedStatusFilter = status) }
        val viewport = lastViewport ?: return
        val zoom = lastZoom ?: return
        fetchJob?.cancel()
        fetchJob =
            launchFetch(
                scope = viewModelScope,
                debounceMillis = debounceMillis,
                debounce = false,
                onViewportSettled = {
                    lastViewport = viewport
                    lastZoom = zoom
                },
            ) {
                fetchClustersAndUpdateState(repository, _uiState, viewport, zoom)
            }
    }

    /** Clears the one-shot [MapUiState.pendingCameraTarget] once the Screen has moved the camera. */
    public fun consumeCameraTarget() {
        _uiState.update { it.copy(pendingCameraTarget = null) }
    }

    /**
     * Routes a cluster tap: a stacked (unsplittable) cell opens the
     * disambiguation list (concurrent all-or-nothing point-reads); a
     * splittable bubble zooms in client-side with no server round-trip
     * (halves the current viewport's span, floored, matching iOS's exact
     * zoom-in math); a single-member cell point-reads and opens the summary
     * sheet.
     */
    public fun onClusterTapped(cluster: MapCluster) {
        when {
            cluster.isStacked -> {
                viewModelScope.launch {
                    pointReadAllOrNull(repository, cluster.members)?.let { applications ->
                        _uiState.update { it.copy(stackedApplications = applications) }
                    }
                }
            }

            cluster.isSingleMember -> {
                val member = cluster.member ?: return
                viewModelScope.launch {
                    pointReadOrNull(repository, member)?.let { application ->
                        _uiState.update { it.copy(selectedApplication = application) }
                        val isSaved = isApplicationSaved(savedApplicationRepository, application.id)
                        _uiState.update { it.copy(isSelectedApplicationSaved = isSaved) }
                    }
                }
            }

            else -> {
                val current = lastViewport ?: return
                _uiState.update { it.copy(pendingCameraTarget = current.zoomedInto(cluster.coordinate)) }
            }
        }
    }

    /** Handles a disambiguation-list row tap: closes the list and opens that application's summary. */
    public fun selectFromStack(application: PlanningApplication) {
        _uiState.update { it.copy(selectedApplication = application, stackedApplications = null) }
        viewModelScope.launch {
            val isSaved = isApplicationSaved(savedApplicationRepository, application.id)
            _uiState.update { it.copy(isSelectedApplicationSaved = isSaved) }
        }
    }

    /** Dismisses the disambiguation list without selecting a row. */
    public fun clearStack() {
        _uiState.update { it.copy(stackedApplications = null) }
    }

    /** Dismisses the summary sheet. */
    public fun clearSelection() {
        _uiState.update { it.copy(selectedApplication = null) }
    }

    /** Optimistically flips the selected application's saved state, reverting on failure — same pattern as `ApplicationDetailViewModel.toggleSave`. */
    @Suppress("SwallowedException")
    // Preserve current state on failure — the reverted isSelectedApplicationSaved IS the response to the error.
    public fun toggleSaveSelectedApplication() {
        val application = _uiState.value.selectedApplication ?: return
        val wasSaved = _uiState.value.isSelectedApplicationSaved
        _uiState.update { it.copy(isSelectedApplicationSaved = !wasSaved) }
        viewModelScope.launch {
            try {
                if (wasSaved) {
                    savedApplicationRepository.unsave(
                        application.id,
                    )
                } else {
                    savedApplicationRepository.save(application.id)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isSelectedApplicationSaved = wasSaved) }
            }
        }
    }

    private companion object {
        const val DEBOUNCE_MILLIS = 250L
    }
}

/**
 * Cancels-and-replaces are the caller's job (see [MapViewModel.fetchJob]) —
 * this just launches one candidate fetch and, once it actually starts running
 * (after the optional debounce, so a fetch cancelled mid-delay never touches
 * either), reports [onViewportSettled] before running [fetch]. Top-level (not
 * a [MapViewModel] member, see the class doc) since callers already hold
 * everything it needs.
 */
private fun launchFetch(
    scope: CoroutineScope,
    debounceMillis: Long,
    debounce: Boolean,
    onViewportSettled: () -> Unit,
    fetch: suspend () -> Unit,
): Job =
    scope.launch {
        if (debounce) delay(debounceMillis)
        onViewportSettled()
        fetch()
    }

/**
 * Fetches the cluster aggregates for a viewport at a zoom and publishes them
 * into [uiState]. A transient refetch failure keeps the last good clusters
 * rather than blanking the map; a screen-level error is surfaced only when
 * there is nothing to show yet — mirrors iOS `loadClusters(viewport:zoom:)`.
 * Top-level (not a [MapViewModel] member, see the class doc) — callers set
 * `lastViewport`/`lastZoom` themselves immediately before calling this, since
 * those two fields stay private `var`s on the ViewModel.
 */
private suspend fun fetchClustersAndUpdateState(
    repository: PlanningApplicationRepository,
    uiState: MutableStateFlow<MapUiState>,
    viewport: MapViewport,
    zoom: Int,
) {
    val zoneId = uiState.value.selectedZone?.id ?: return
    try {
        val clusters = repository.fetchClusters(zoneId, viewport, zoom, uiState.value.selectedStatusFilter)
        uiState.update { it.copy(clusters = clusters, error = null) }
    } catch (e: CancellationException) {
        throw e
    } catch (e: DomainError) {
        if (uiState.value.clusters.isEmpty()) {
            uiState.update { it.copy(error = e) }
        }
    }
}

private suspend fun resolveInitialZone(
    mapPreferencesStore: MapPreferencesStore,
    zones: List<WatchZone>,
): WatchZone? {
    val savedId = mapPreferencesStore.readLastSelectedZoneId()
    return zones.firstOrNull { it.id == savedId } ?: zones.firstOrNull()
}

/** A transient point-read failure leaves the map untouched (`null`); the user can tap the pin again. */
@Suppress("SwallowedException")
private suspend fun pointReadOrNull(
    repository: PlanningApplicationRepository,
    member: PlanningApplicationId,
): PlanningApplication? =
    try {
        repository.detail(member.authority, member.name)
    } catch (e: CancellationException) {
        throw e
    } catch (e: DomainError) {
        null
    }

/**
 * Point-reads every carried member concurrently via [coroutineScope] +
 * [async]/[awaitAll]: structured concurrency means one failure cancels the
 * siblings still in flight and rethrows, which is exactly the all-or-nothing
 * contract this needs — any member failing returns `null` (no partial list
 * is ever published).
 */
@Suppress("SwallowedException")
private suspend fun pointReadAllOrNull(
    repository: PlanningApplicationRepository,
    members: List<PlanningApplicationId>,
): List<PlanningApplication>? =
    try {
        coroutineScope {
            members.map { member -> async { repository.detail(member.authority, member.name) } }.awaitAll()
        }
    } catch (e: CancellationException) {
        throw e
    } catch (e: DomainError) {
        null
    }

@Suppress("SwallowedException")
private suspend fun isApplicationSaved(
    savedApplicationRepository: SavedApplicationRepository,
    applicationId: PlanningApplicationId,
): Boolean =
    try {
        savedApplicationRepository.savedApplications().any { it.applicationUid == applicationId }
    } catch (e: CancellationException) {
        throw e
    } catch (e: DomainError) {
        false
    }
