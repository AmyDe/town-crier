package uk.towncrierapp.presentation.features.onboarding

import android.util.Log
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.onboarding.OnboardingRepository
import uk.towncrierapp.domain.subscriptions.Entitlement
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Postcode
import uk.towncrierapp.domain.watchzones.PostcodeGeocoder
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.domain.watchzones.WatchZoneLimits
import uk.towncrierapp.domain.watchzones.WatchZoneRepository
import uk.towncrierapp.presentation.designsystem.components.LARGE_RADIUS_WARNING_THRESHOLD_METRES
import uk.towncrierapp.presentation.features.watchzones.PolygonDrawingState
import uk.towncrierapp.presentation.features.watchzones.ShapeForSave
import uk.towncrierapp.presentation.features.watchzones.WatchZoneShapeEvent
import uk.towncrierapp.presentation.features.watchzones.WatchZoneShapeMode
import uk.towncrierapp.presentation.features.watchzones.applyShapeEvent
import uk.towncrierapp.presentation.features.watchzones.resolveShapeForSave
import java.util.UUID

private val INSTANT_ALERT_ENTITLEMENT = Entitlement.STATUS_CHANGE_ALERTS

/**
 * Drives the onboarding wizard: postcode geocoding, tier-based radius
 * limits, tier-aware notification-permission copy, and the custom-shape
 * (polygon) drawing step (GH#1072 Phase 5, tc-v6fo0.5). Deliberately
 * constructed ONCE per wizard presentation - never re-created per step or on
 * a tier change - so [reconcileTierAfterUpgrade]/[reconcileTierAfterCustomShapeUpgrade]
 * can push a later tier into the SAME instance's state. That is what lets it
 * survive a future paywall dialog presented over the wizard (#783) without
 * losing the postcode/coordinate/radius already entered: the composition
 * root constructs this ViewModel once for the whole wizard route, not once
 * per step. Port of iOS `OnboardingViewModel`.
 */
@Suppress("TooManyFunctions")
// This wizard genuinely owns 16 independent, single-purpose interactions:
// the original 10 (step nav, postcode entry/lookup, radius edit/confirm,
// notification enable/skip/complete, reconcileTierAfterUpgrade) plus GH#1072
// Phase 5's 6 custom-shape actions (onShapeEvent - itself one dispatch point
// standing in for 5 shape/polygon actions, see WatchZoneShapeEvent -
// selectCustomShape, requestCustomShapeUpgrade, consumeNavigateToPaywall,
// reconcileTierAfterCustomShapeUpgrade, confirmBoundary). Splitting further
// means either merging semantically-distinct methods (breaks the
// cross-ViewModel consumeX()/confirmX() naming convention) or hiving off an
// unrelated class purely to dodge this count - neither improves readability
// over the function-count overage, same rationale as WatchZoneEditorViewModel.
public class OnboardingViewModel(
    private val postcodeGeocoder: PostcodeGeocoder,
    private val watchZoneRepository: WatchZoneRepository,
    private val onboardingRepository: OnboardingRepository,
    tier: SubscriptionTier,
    private val paywallAvailable: Boolean = false,
    private val enableDebugLogging: Boolean = false,
) : ViewModel() {
    private val _uiState = MutableStateFlow(initialState(tier, paywallAvailable))
    public val uiState: StateFlow<OnboardingUiState> = _uiState.asStateFlow()

    /**
     * Advances Welcome -> Postcode unconditionally, or Postcode -> Radius
     * once a postcode has been resolved. Radius -> NotificationPermission
     * goes through [confirmRadius] instead (and [OnboardingStep.BoundaryDrawing]
     * -> NotificationPermission through [confirmBoundary]), since both
     * transitions also build the in-memory zone.
     */
    public fun advance() {
        val next = nextStep(_uiState.value) ?: return
        _uiState.update { it.copy(step = next) }
    }

    /**
     * No-op on [OnboardingStep.Welcome] - the wizard is linear going forward
     * but not one-way. Stepping back out of [OnboardingStep.BoundaryDrawing]
     * discards [OnboardingUiState.shapeMode]/[OnboardingUiState.polygonVertices]
     * and returns to [OnboardingStep.Radius] as a plain circle pick, mirroring
     * the editor's mode toggle; stepping back from
     * [OnboardingStep.NotificationPermission] routes to whichever of Radius
     * or BoundaryDrawing the in-progress zone actually came from, using
     * [OnboardingUiState.shapeMode] as the source of truth (it is only ever
     * [WatchZoneShapeMode.CUSTOM] after a genuine [confirmBoundary] call).
     */
    public fun back() {
        val state = _uiState.value
        val previous = previousStep(state) ?: return
        _uiState.update {
            val withNewStep = it.copy(step = previous)
            if (state.step == OnboardingStep.BoundaryDrawing) withNewStep.discardCustomShapeDrawing() else withNewStep
        }
    }

    public fun updatePostcode(value: String) {
        _uiState.update { it.copy(postcodeInput = value) }
    }

    /** Validates the format locally first - a garbage postcode never spends a network call - before calling [postcodeGeocoder]. */
    public fun lookUpPostcode() {
        val raw = _uiState.value.postcodeInput
        val postcode = Postcode.parse(raw)
        if (postcode == null) {
            _uiState.update { it.copy(postcodeError = DomainError.GeocodingFailed(raw)) }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isLookingUpPostcode = true, postcodeError = null) }
            try {
                val coordinate = postcodeGeocoder.geocode(postcode.value)
                _uiState.update {
                    it.copy(isLookingUpPostcode = false, resolvedPostcode = postcode, geocodedCoordinate = coordinate)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isLookingUpPostcode = false, postcodeError = e) }
            }
        }
    }

    public fun updateRadius(value: Float) {
        _uiState.update {
            it.copy(radiusMetres = value, showsLargeRadiusWarning = value >= LARGE_RADIUS_WARNING_THRESHOLD_METRES)
        }
    }

    /** Builds the [WatchZone] in memory only - nothing is saved until [completeOnboarding] - and advances to the notification-permission step. */
    public fun confirmRadius() {
        val state = _uiState.value
        val coordinate = state.geocodedCoordinate ?: return
        val limits = WatchZoneLimits(state.tier)
        val zone =
            WatchZone(
                id = WatchZoneId(UUID.randomUUID().toString()),
                name = state.resolvedPostcode?.value ?: state.postcodeInput.trim(),
                centre = coordinate,
                radiusMetres = limits.clampRadius(state.radiusMetres.toDouble()),
            )
        _uiState.update { it.copy(pendingZone = zone, step = OnboardingStep.NotificationPermission) }
    }

    // MARK: - Custom-shape boundary drawing (GH#1072 Phase 5, tc-v6fo0.5)

    /**
     * Routes every shape-mode/polygon-drawing interaction on
     * [OnboardingStep.BoundaryDrawing] through one dispatch point, same
     * shape as [uk.towncrierapp.presentation.features.watchzones.WatchZoneEditorViewModel.onShapeEvent]
     * - see [WatchZoneShapeEvent] for why a single method covers five
     * distinct user actions. The actual rules live in
     * [PolygonDrawingState.applyShapeEvent], shared with the editor rather
     * than duplicated.
     */
    public fun onShapeEvent(event: WatchZoneShapeEvent) {
        _uiState.update { state ->
            state.withPolygonDrawing(state.toPolygonDrawingState().applyShapeEvent(event, state.allowsCustomBoundary))
        }
    }

    /**
     * Re-entry point for a user whose tier already allows a custom shape (an
     * already-Personal/Pro user, or one who backed out of drawing and wants
     * back in) - mirrors iOS `selectCustomShape()`. Only takes effect while
     * sitting on [OnboardingStep.Radius] and entitled; a no-op from any other
     * step or tier, since [RadiusPickerStep] only ever shows this affordance
     * there.
     */
    public fun selectCustomShape() {
        _uiState.update { it.enterBoundaryDrawingIfEligible() }
    }

    /** Surfaces the future (#783) in-wizard custom-shape paywall when a free-tier user taps the upsell CTA on the radius step. */
    public fun requestCustomShapeUpgrade() {
        _uiState.update { it.copy(navigateToPaywall = true) }
    }

    /** Resets the one-shot route-to-paywall signal once the Route has acted on it. */
    public fun consumeNavigateToPaywall() {
        _uiState.update { it.copy(navigateToPaywall = false) }
    }

    /**
     * Pushes a tier raised by a (future, #783) purchase from the radius
     * step's custom-shape upsell into this SAME instance, via
     * [reconcileTierAfterUpgrade] - then, mirroring iOS
     * `reconcileTierAfterCustomShapeUpgrade()`, swaps [OnboardingStep.Radius]
     * for [OnboardingStep.BoundaryDrawing] ONLY when the user is actually
     * still sitting on the radius step and the new tier is now entitled. A
     * purchase completing after the user has already moved on (or that
     * somehow didn't unlock custom shapes) leaves the step untouched.
     */
    public fun reconcileTierAfterCustomShapeUpgrade(newTier: SubscriptionTier) {
        reconcileTierAfterUpgrade(newTier)
        _uiState.update { it.enterBoundaryDrawingIfEligible() }
    }

    /**
     * Builds the [WatchZone] from the drawn boundary and advances to the
     * notification-permission step - the [OnboardingStep.BoundaryDrawing]
     * counterpart to [confirmRadius]. Centre/radius are derived from the
     * boundary (centroid + enclosing radius), same as
     * [uk.towncrierapp.presentation.features.watchzones.WatchZoneEditorViewModel.save]'s
     * custom-shape path, so every circle-shaped read path (map centring,
     * list rows) keeps working unchanged. A no-op while the polygon isn't
     * closed yet (matches the disabled confirm button); surfaces
     * [OnboardingUiState.boundaryError] instead of advancing if the vertices
     * somehow fail [uk.towncrierapp.domain.watchzones.WatchZoneBoundary.of]
     * validation at this point.
     */
    public fun confirmBoundary() {
        val state = _uiState.value
        if (state.geocodedCoordinate == null) return
        when (val shape = state.toPolygonDrawingState().resolveShapeForSave()) {
            ShapeForSave.Circle, ShapeForSave.NotReady -> {
                Unit
            }

            ShapeForSave.Invalid -> {
                _uiState.update { it.copy(boundaryError = true) }
            }

            is ShapeForSave.Custom -> {
                val boundary = shape.boundary
                val zone =
                    WatchZone(
                        id = WatchZoneId(UUID.randomUUID().toString()),
                        name = state.resolvedPostcode?.value ?: state.postcodeInput.trim(),
                        centre = boundary.centroid,
                        radiusMetres = boundary.enclosingRadiusMetres,
                        boundary = boundary,
                    )
                _uiState.update { it.copy(pendingZone = zone, step = OnboardingStep.NotificationPermission) }
            }
        }
    }

    /**
     * Called when the user taps "Enable notifications". The actual OS
     * permission request is the Route's job (`rememberLauncherForActivityResult`
     * needs a composable scope) - this function's only job is to complete
     * the wizard regardless of the eventual grant/deny result, which is
     * intentionally never observed here.
     */
    public fun requestNotificationPermission() {
        completeOnboarding()
    }

    /** Free-tier's honest path: no OS permission was ever requested. */
    public fun skipNotifications() {
        completeOnboarding()
    }

    /**
     * Persists the in-memory zone best-effort - a failure is logged, not
     * blocking - sets the device latch, and signals completion. Matches iOS
     * `try?` semantics: the zones tab shows empty and the user can retry
     * from there rather than being trapped in the wizard.
     */
    public fun completeOnboarding() {
        viewModelScope.launch {
            _uiState.update { it.copy(isCompleting = true) }
            _uiState.value.pendingZone?.let { zone ->
                try {
                    watchZoneRepository.create(zone)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: DomainError) {
                    log(enableDebugLogging) { "best-effort watch zone save failed at onboarding completion: $e" }
                }
            }
            onboardingRepository.setOnboardingComplete(true)
            _uiState.update { it.copy(isCompleting = false, isComplete = true) }
        }
    }

    /**
     * Pushes a tier raised by a (future, #783) paywall purchase into this
     * SAME instance - the caller re-resolves the tier after the purchase
     * flow and calls this rather than recreating the ViewModel, which is
     * what preserves postcode/coordinate/radius already entered. Also
     * refreshes [OnboardingUiState.allowsCustomBoundary] - see
     * [reconcileTierAfterCustomShapeUpgrade] for the mid-flow step swap this
     * alone does NOT perform.
     */
    public fun reconcileTierAfterUpgrade(newTier: SubscriptionTier) {
        _uiState.update { state ->
            val limits = WatchZoneLimits(newTier)
            val newMax = limits.maxRadiusMetres.toFloat()
            val clampedRadius = minOf(state.radiusMetres, newMax)
            state.copy(
                tier = newTier,
                maxRadiusMetres = newMax,
                radiusMetres = clampedRadius,
                showsLargeRadiusWarning = clampedRadius >= LARGE_RADIUS_WARNING_THRESHOLD_METRES,
                canUnlockLargerRadius = paywallAvailable && newTier < SubscriptionTier.PRO,
                hasInstantAlertEntitlement = FeatureGate(newTier).hasEntitlement(INSTANT_ALERT_ENTITLEMENT),
                allowsCustomBoundary = limits.allowsCustomBoundary,
            )
        }
    }
}

private fun nextStep(state: OnboardingUiState): OnboardingStep? =
    when (state.step) {
        OnboardingStep.Welcome -> OnboardingStep.Postcode
        OnboardingStep.Postcode -> if (state.geocodedCoordinate != null) OnboardingStep.Radius else null
        OnboardingStep.Radius, OnboardingStep.BoundaryDrawing, OnboardingStep.NotificationPermission -> null
    }

private fun previousStep(state: OnboardingUiState): OnboardingStep? =
    when (state.step) {
        OnboardingStep.Welcome -> {
            null
        }

        OnboardingStep.Postcode -> {
            OnboardingStep.Welcome
        }

        OnboardingStep.Radius -> {
            OnboardingStep.Postcode
        }

        OnboardingStep.BoundaryDrawing -> {
            OnboardingStep.Radius
        }

        OnboardingStep.NotificationPermission -> {
            if (state.shapeMode == WatchZoneShapeMode.CUSTOM) OnboardingStep.BoundaryDrawing else OnboardingStep.Radius
        }
    }

private fun initialState(
    tier: SubscriptionTier,
    paywallAvailable: Boolean,
): OnboardingUiState {
    val limits = WatchZoneLimits(tier)
    return OnboardingUiState(
        tier = tier,
        maxRadiusMetres = limits.maxRadiusMetres.toFloat(),
        canUnlockLargerRadius = paywallAvailable && tier < SubscriptionTier.PRO,
        hasInstantAlertEntitlement = FeatureGate(tier).hasEntitlement(INSTANT_ALERT_ENTITLEMENT),
        allowsCustomBoundary = limits.allowsCustomBoundary,
    )
}

// MARK: - Custom-shape boundary drawing helpers (GH#1072 Phase 5, tc-v6fo0.5)
//
// Top-level, not class members, for the same TooManyFunctions/complexity
// budget reason as WatchZoneEditorViewModel's equivalent private helpers.

private fun OnboardingUiState.toPolygonDrawingState(): PolygonDrawingState =
    PolygonDrawingState(
        shapeMode = shapeMode,
        polygonVertices = polygonVertices,
        isPolygonClosed = isPolygonClosed,
        canClosePolygon = canClosePolygon,
        boundaryError = boundaryError,
    )

private fun OnboardingUiState.withPolygonDrawing(drawing: PolygonDrawingState): OnboardingUiState =
    copy(
        shapeMode = drawing.shapeMode,
        polygonVertices = drawing.polygonVertices,
        isPolygonClosed = drawing.isPolygonClosed,
        canClosePolygon = drawing.canClosePolygon,
        boundaryError = drawing.boundaryError,
    )

/** Discards any in-progress drawing and reverts to [WatchZoneShapeMode.CIRCLE] - what stepping back out of [OnboardingStep.BoundaryDrawing] does. */
private fun OnboardingUiState.discardCustomShapeDrawing(): OnboardingUiState = withPolygonDrawing(PolygonDrawingState())

/**
 * Swaps [OnboardingStep.Radius] for [OnboardingStep.BoundaryDrawing] when -
 * and only when - currently on the radius step and [OnboardingUiState.allowsCustomBoundary]
 * is true. Shared by [OnboardingViewModel.selectCustomShape] and
 * [OnboardingViewModel.reconcileTierAfterCustomShapeUpgrade].
 */
private fun OnboardingUiState.enterBoundaryDrawingIfEligible(): OnboardingUiState =
    if (step == OnboardingStep.Radius && allowsCustomBoundary) {
        copy(shapeMode = WatchZoneShapeMode.CUSTOM, step = OnboardingStep.BoundaryDrawing)
    } else {
        this
    }

private inline fun log(
    enableDebugLogging: Boolean,
    message: () -> String,
) {
    // Guarded so plain JVM unit tests (enableDebugLogging = false by
    // default) never touch android.util.Log - mirrors ApiClient's `log`.
    if (enableDebugLogging) Log.w("OnboardingViewModel", message())
}
