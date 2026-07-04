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
import java.util.UUID

private val INSTANT_ALERT_ENTITLEMENT = Entitlement.STATUS_CHANGE_ALERTS

/**
 * Drives the four-step onboarding wizard: postcode geocoding, tier-based
 * radius limits, and tier-aware notification-permission copy. Deliberately
 * constructed ONCE per wizard presentation - never re-created per step or on
 * a tier change - so [reconcileTierAfterUpgrade] can push a later tier into
 * the SAME instance's state. That is what lets it survive a future paywall
 * dialog presented over the wizard (#783) without losing the
 * postcode/coordinate/radius already entered: the composition root
 * constructs this ViewModel once for the whole wizard route, not once per
 * step. Port of iOS `OnboardingViewModel`.
 */
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
     * goes through [confirmRadius] instead, since that transition also
     * builds the in-memory zone.
     */
    public fun advance() {
        val next = nextStep(_uiState.value) ?: return
        _uiState.update { it.copy(step = next) }
    }

    /** No-op on [OnboardingStep.Welcome] - the wizard is linear going forward but not one-way. */
    public fun back() {
        val previous = previousStep(_uiState.value.step) ?: return
        _uiState.update { it.copy(step = previous) }
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
     * what preserves postcode/coordinate/radius already entered.
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
            )
        }
    }
}

private fun nextStep(state: OnboardingUiState): OnboardingStep? =
    when (state.step) {
        OnboardingStep.Welcome -> OnboardingStep.Postcode
        OnboardingStep.Postcode -> if (state.geocodedCoordinate != null) OnboardingStep.Radius else null
        OnboardingStep.Radius, OnboardingStep.NotificationPermission -> null
    }

private fun previousStep(step: OnboardingStep): OnboardingStep? =
    when (step) {
        OnboardingStep.Welcome -> null
        OnboardingStep.Postcode -> OnboardingStep.Welcome
        OnboardingStep.Radius -> OnboardingStep.Postcode
        OnboardingStep.NotificationPermission -> OnboardingStep.Radius
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
    )
}

private inline fun log(
    enableDebugLogging: Boolean,
    message: () -> String,
) {
    // Guarded so plain JVM unit tests (enableDebugLogging = false by
    // default) never touch android.util.Log - mirrors ApiClient's `log`.
    if (enableDebugLogging) Log.w("OnboardingViewModel", message())
}
