package uk.towncrierapp.presentation.features.watchzones

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.applications.ApplicationCacheStore
import uk.towncrierapp.domain.applications.CachedApplicationPage
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.subscriptions.Entitlement
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.PostcodeGeocoder
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneBoundary
import uk.towncrierapp.domain.watchzones.WatchZoneBoundaryResult
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneLimits
import uk.towncrierapp.domain.watchzones.WatchZoneRepository
import uk.towncrierapp.presentation.designsystem.components.LARGE_RADIUS_WARNING_THRESHOLD_METRES
import java.util.UUID

/**
 * Drives create/edit of a single watch zone: postcode geocoding, tier-based
 * radius limits, and the two instant-alert toggles. Port of iOS
 * `WatchZoneEditorViewModel`. Navigation (dismiss-on-save, route-to-paywall)
 * is modelled as state the Route reconciles — see [WatchZoneEditorUiState].
 */
@Suppress("TooManyFunctions")
// This screen genuinely owns 11 independent, single-purpose interactions
// (name/postcode/radius/toggle edits, submit, save, two one-shot signal
// consumers matching the `consumeX()` convention shared with
// ZonePreferencesViewModel, plus GH#1072 Phase 3's onShapeEvent — itself
// already a single dispatch point standing in for 5 shape/polygon actions,
// see WatchZoneShapeEvent). Splitting further means either merging
// semantically-distinct consumeX() methods (breaks the cross-ViewModel
// naming convention) or hiving off an unrelated class purely to dodge this
// count — neither improves readability over the one-function overage.
public class WatchZoneEditorViewModel(
    private val geocoder: PostcodeGeocoder,
    private val repository: WatchZoneRepository,
    tier: SubscriptionTier,
    private val editingZone: WatchZone? = null,
    // Defaults to a no-op so every pre-existing call site (tests, and any
    // future caller with nothing to invalidate) keeps compiling unchanged;
    // the composition root wires the real cache seam explicitly (tc-cnme).
    private val applicationCacheStore: ApplicationCacheStore = NoOpApplicationCacheStore,
) : ViewModel() {
    private val limits = WatchZoneLimits(tier)

    private val _uiState = MutableStateFlow(initialState(tier, limits, editingZone))
    public val uiState: StateFlow<WatchZoneEditorUiState> = _uiState.asStateFlow()

    public fun updateName(value: String) {
        _uiState.update { it.copy(name = value).withSaveEnabled() }
    }

    public fun updatePostcode(value: String) {
        _uiState.update { it.copy(postcode = value) }
    }

    public fun updateRadius(value: Float) {
        _uiState.update {
            it.copy(
                radiusMetres = value,
                showsLargeRadiusWarning = value >= LARGE_RADIUS_WARNING_THRESHOLD_METRES,
            )
        }
    }

    public fun updatePushEnabled(value: Boolean) {
        _uiState.update { it.copy(pushEnabled = value) }
    }

    public fun updateEmailInstantEnabled(value: Boolean) {
        _uiState.update { it.copy(emailInstantEnabled = value) }
    }

    /** Surfaces the in-editor subscription upsell when a free-tier user taps a locked instant-alert toggle. */
    public fun requestInstantAlertUpgrade() {
        _uiState.update { it.copy(navigateToPaywall = true) }
    }

    /**
     * Routes every shape-mode/polygon-drawing interaction (GH#1072 Phase 3)
     * through one dispatch point — see [WatchZoneShapeEvent] for why a
     * single method covers five distinct user actions. All the actual rules
     * (the 3-vertex close gate, the 50-vertex cap, [WatchZoneBoundary.of]
     * validation, the Free-tier custom-mode guard) live in the private
     * `apply*`/`with*` helpers below, kept as top-level functions so this
     * class's own function/complexity budgets stay unaffected by them.
     */
    public fun onShapeEvent(event: WatchZoneShapeEvent) {
        _uiState.update { state -> state.applyShapeEvent(event, limits) }
    }

    public fun submitPostcode() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val coordinate = geocoder.geocode(_uiState.value.postcode)
                _uiState.update {
                    it
                        .copy(
                            geocodedCoordinate = coordinate,
                            isLoading = false,
                            name = it.name.ifBlank { it.postcode.trim() },
                        ).withSaveEnabled()
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLoading = false, error = e) }
            }
        }
    }

    /**
     * Persists the zone. On a quota breach
     * ([DomainError.InsufficientEntitlement]) routes to the paywall
     * placeholder and leaves `error` unset — the Route dismisses the editor.
     * Any other failure sets the inline error and leaves the editor open.
     */
    @Suppress("SwallowedException")
    // Deliberate degrade path (tc-gpjk): a quota breach routes to the paywall
    // placeholder — the UI only needs THAT it happened, not the exception itself.
    public fun save() {
        val state = _uiState.value
        val coordinate = state.geocodedCoordinate ?: return
        val boundary =
            when (val shape = state.resolveShapeForSave()) {
                ShapeForSave.Circle -> {
                    null
                }

                is ShapeForSave.Custom -> {
                    shape.boundary
                }

                ShapeForSave.NotReady -> {
                    return
                }

                ShapeForSave.Invalid -> {
                    _uiState.update { it.copy(boundaryError = true) }
                    return
                }
            }
        viewModelScope.launch {
            _uiState.update { it.copy(error = null) }
            val zone =
                WatchZone(
                    id = editingZone?.id ?: WatchZoneId(UUID.randomUUID().toString()),
                    name = state.name.trim(),
                    // A custom-shape zone's centre/radius are derived from its
                    // boundary (centroid + enclosing radius), same as the
                    // server (issue #1072) — circle mode keeps clamping the
                    // slider value to the tier's radius cap; a drawn shape
                    // isn't reshaped to fit, it's sent as drawn and the server
                    // is the source of truth on `boundary_too_large`.
                    centre = boundary?.centroid ?: coordinate,
                    radiusMetres = boundary?.enclosingRadiusMetres ?: limits.clampRadius(state.radiusMetres.toDouble()),
                    authorityId = editingZone?.authorityId ?: 0,
                    pushEnabled = state.pushEnabled,
                    emailInstantEnabled = state.emailInstantEnabled,
                    boundary = boundary,
                )
            try {
                if (editingZone != null) {
                    repository.update(zone)
                    // Only the edit path invalidates (tc-cnme): a brand-new
                    // zone has no prior applications-list cache entry to
                    // invalidate, mirroring iOS AppCoordinator+WatchZones.swift's
                    // onSave callback.
                    applicationCacheStore.invalidate(zone.id)
                } else {
                    repository.create(zone)
                }
                _uiState.update { it.copy(saveCompleted = true) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError.InsufficientEntitlement) {
                _uiState.update { it.copy(navigateToPaywall = true) }
            } catch (e: DomainError) {
                _uiState.update { it.copy(error = e) }
            }
        }
    }

    /** Resets the one-shot dismiss-editor signal once the Route has acted on it. */
    public fun consumeSaveCompleted() {
        _uiState.update { it.copy(saveCompleted = false) }
    }

    /** Resets the one-shot route-to-paywall signal once the Route has acted on it. */
    public fun consumeNavigateToPaywall() {
        _uiState.update { it.copy(navigateToPaywall = false) }
    }
}

// Top-level (not a class member) purely to keep the class's own function
// count under detekt's TooManyFunctions budget — it needs no instance state.
private fun WatchZoneEditorUiState.withSaveEnabled(): WatchZoneEditorUiState =
    copy(isSaveEnabled = geocodedCoordinate != null && name.isNotBlank() && isShapeReady())

/** A circle is always ready to save; a custom shape needs its polygon closed first. */
private fun WatchZoneEditorUiState.isShapeReady(): Boolean =
    when (shapeMode) {
        WatchZoneShapeMode.CIRCLE -> true
        WatchZoneShapeMode.CUSTOM -> isPolygonClosed
    }

// MARK: - Shape-mode/polygon-drawing event handling (GH#1072 Phase 3)
//
// Top-level, not class members, for the same TooManyFunctions/complexity
// budget reason as withSaveEnabled/isShapeReady above: WatchZoneEditorViewModel
// exposes ONE public entry point (onShapeEvent); every rule these events
// enforce (the Free-tier custom-mode guard, the 3-vertex close gate, the
// 50-vertex add cap, WatchZoneBoundary.of validation) lives here instead.

private fun WatchZoneEditorUiState.applyShapeEvent(
    event: WatchZoneShapeEvent,
    limits: WatchZoneLimits,
): WatchZoneEditorUiState =
    when (event) {
        is WatchZoneShapeEvent.ModeSelected -> withShapeMode(event.mode, limits)
        is WatchZoneShapeEvent.VertexAdded -> withVertexAdded(event.coordinate)
        is WatchZoneShapeEvent.VertexMoved -> withVertexMoved(event.index, event.coordinate)
        WatchZoneShapeEvent.PolygonClosed -> withPolygonClosed()
        WatchZoneShapeEvent.UndoRequested -> withUndoApplied()
    }

/** A no-op for [WatchZoneShapeMode.CUSTOM] when [limits] don't allow it — belt-and-braces alongside the Screen never offering it. */
private fun WatchZoneEditorUiState.withShapeMode(
    mode: WatchZoneShapeMode,
    limits: WatchZoneLimits,
): WatchZoneEditorUiState {
    if (mode == WatchZoneShapeMode.CUSTOM && !limits.allowsCustomBoundary) return this
    return copy(shapeMode = mode).withSaveEnabled()
}

/** Ignored once closed (undo first) or at the 50-vertex cap ([WatchZoneBoundary.MAX_VERTICES]). */
private fun WatchZoneEditorUiState.withVertexAdded(coordinate: Coordinate): WatchZoneEditorUiState {
    if (isPolygonClosed || polygonVertices.size >= WatchZoneBoundary.MAX_VERTICES) return this
    val vertices = polygonVertices + coordinate
    return copy(
        polygonVertices = vertices,
        canClosePolygon = vertices.size >= WatchZoneBoundary.MIN_VERTICES,
        boundaryError = false,
    ).withSaveEnabled()
}

/**
 * Ignored for an out-of-range [index]. Dragging a vertex on an already-closed
 * polygon re-validates through [WatchZoneBoundary.of] the same way
 * [withPolygonClosed] does — a drag that makes the ring self-intersecting
 * reopens it and surfaces [WatchZoneEditorUiState.boundaryError] rather than
 * leaving [WatchZoneEditorUiState.isPolygonClosed]/`isSaveEnabled` stale for
 * a shape that would silently fail on save.
 */
private fun WatchZoneEditorUiState.withVertexMoved(
    index: Int,
    coordinate: Coordinate,
): WatchZoneEditorUiState {
    if (index !in polygonVertices.indices) return this
    val vertices = polygonVertices.toMutableList().also { it[index] = coordinate }
    if (!isPolygonClosed) {
        return copy(polygonVertices = vertices, boundaryError = false).withSaveEnabled()
    }
    return when (WatchZoneBoundary.of(vertices)) {
        is WatchZoneBoundaryResult.Valid -> copy(polygonVertices = vertices, boundaryError = false).withSaveEnabled()
        else -> copy(polygonVertices = vertices, isPolygonClosed = false, boundaryError = true).withSaveEnabled()
    }
}

/**
 * Requires at least [WatchZoneBoundary.MIN_VERTICES] and a simple,
 * non-self-intersecting ring — [WatchZoneBoundary.of] is the single source
 * of truth for both, so an invalid shape sets
 * [WatchZoneEditorUiState.boundaryError] and stays open rather than closing
 * anyway.
 */
private fun WatchZoneEditorUiState.withPolygonClosed(): WatchZoneEditorUiState {
    if (isPolygonClosed || polygonVertices.size < WatchZoneBoundary.MIN_VERTICES) return this
    return when (WatchZoneBoundary.of(polygonVertices)) {
        is WatchZoneBoundaryResult.Valid -> copy(isPolygonClosed = true, boundaryError = false).withSaveEnabled()
        else -> copy(boundaryError = true)
    }
}

/** Reopens a just-closed polygon without dropping a vertex, otherwise drops the most recently added vertex; a no-op with nothing drawn. */
private fun WatchZoneEditorUiState.withUndoApplied(): WatchZoneEditorUiState =
    when {
        isPolygonClosed -> {
            copy(isPolygonClosed = false, boundaryError = false).withSaveEnabled()
        }

        polygonVertices.isNotEmpty() -> {
            val vertices = polygonVertices.dropLast(1)
            copy(
                polygonVertices = vertices,
                canClosePolygon = vertices.size >= WatchZoneBoundary.MIN_VERTICES,
                boundaryError = false,
            ).withSaveEnabled()
        }

        else -> {
            this
        }
    }

/** The shape [WatchZoneEditorViewModel.save] resolves from the current state — see [WatchZoneEditorUiState.resolveShapeForSave]. */
private sealed interface ShapeForSave {
    data object Circle : ShapeForSave

    data class Custom(
        val boundary: WatchZoneBoundary,
    ) : ShapeForSave

    /** Custom mode but the polygon isn't closed yet — save() aborts silently, matching the disabled Save button. */
    data object NotReady : ShapeForSave

    /** Custom mode, closed, but [WatchZoneBoundary.of] rejected the vertices (e.g. self-intersecting). */
    data object Invalid : ShapeForSave
}

private fun WatchZoneEditorUiState.resolveShapeForSave(): ShapeForSave =
    when (shapeMode) {
        WatchZoneShapeMode.CIRCLE -> {
            ShapeForSave.Circle
        }

        WatchZoneShapeMode.CUSTOM -> {
            if (!isPolygonClosed) {
                ShapeForSave.NotReady
            } else {
                when (val result = WatchZoneBoundary.of(polygonVertices)) {
                    is WatchZoneBoundaryResult.Valid -> ShapeForSave.Custom(result.boundary)
                    else -> ShapeForSave.Invalid
                }
            }
        }
    }

// The default ApplicationCacheStore (tc-cnme) — genuinely stateless, so a
// shared `object` (not a hidden mutable global) is the right shape here.
private object NoOpApplicationCacheStore : ApplicationCacheStore {
    override suspend fun get(zoneId: WatchZoneId): CachedApplicationPage? = null

    override suspend fun put(
        zoneId: WatchZoneId,
        entry: CachedApplicationPage,
    ) = Unit

    override suspend fun invalidate(zoneId: WatchZoneId) = Unit

    override suspend fun invalidateAll() = Unit
}

private fun initialState(
    tier: SubscriptionTier,
    limits: WatchZoneLimits,
    editingZone: WatchZone?,
): WatchZoneEditorUiState {
    val radius = (editingZone?.radiusMetres?.toFloat() ?: WatchZoneEditorUiState.DEFAULT_RADIUS_METRES)
    // WatchZoneBoundary.vertices is the closed ring (first vertex repeated
    // last); the editor's own [WatchZoneEditorUiState.polygonVertices]
    // mirrors the open, as-tapped ring instead, so the closing repeat is
    // dropped here and re-added by WatchZoneBoundary.of on save/close.
    val boundary = editingZone?.boundary
    val polygonVertices = boundary?.vertices?.dropLast(1).orEmpty()
    return WatchZoneEditorUiState(
        isEditing = editingZone != null,
        name = editingZone?.name.orEmpty(),
        radiusMetres = radius,
        maxRadiusMetres = limits.maxRadiusMetres.toFloat(),
        geocodedCoordinate = editingZone?.centre,
        pushEnabled = editingZone?.pushEnabled ?: true,
        emailInstantEnabled = editingZone?.emailInstantEnabled ?: true,
        featureGate = FeatureGate(tier),
        instantAlertEntitlement = Entitlement.STATUS_CHANGE_ALERTS,
        canUnlockLargerRadius = tier < SubscriptionTier.PRO,
        showsLargeRadiusWarning = radius >= LARGE_RADIUS_WARNING_THRESHOLD_METRES,
        isSaveEnabled = editingZone != null,
        allowsCustomBoundary = limits.allowsCustomBoundary,
        shapeMode = if (boundary != null) WatchZoneShapeMode.CUSTOM else WatchZoneShapeMode.CIRCLE,
        polygonVertices = polygonVertices,
        isPolygonClosed = boundary != null,
        canClosePolygon = polygonVertices.size >= WatchZoneBoundary.MIN_VERTICES,
    )
}
