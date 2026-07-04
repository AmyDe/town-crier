package uk.towncrierapp.presentation.features.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import uk.towncrierapp.domain.auth.AuthenticationService
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.devicetoken.DeviceTokenRepository
import uk.towncrierapp.domain.profile.UserProfileRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.appearance.AppearanceCoordinator
import uk.towncrierapp.presentation.designsystem.Appearance

/**
 * Bundles the two sign-out-adjacent dependencies so [SettingsViewModel]'s own
 * constructor parameter list stays under detekt's threshold (same rationale
 * as `AppGraphOptions`/`SettingsLeaves` in `:app`). [deviceTokenRepository]
 * defaults to `null` since #777 hasn't landed a real device-token
 * registration yet — every removal attempt is a true no-op until it does.
 * [onSignedOut] defaults to a no-op so every pre-existing call site (tests)
 * keeps compiling unchanged; the composition root wires it to
 * `LoginViewModel.markSignedOut()` so the shell routes back to Login.
 */
public class SettingsSignOutSupport(
    public val deviceTokenRepository: DeviceTokenRepository? = null,
    public val onSignedOut: () -> Unit = {},
)

/**
 * Drives the Settings screen: account display, the four-way appearance
 * picker, sign-out, and account deletion (UK GDPR Art. 17 ordering — see
 * [confirmDeleteAccount]). Notification preferences, legal documents, and
 * data-attribution rows are static or delegate to their own screens; this
 * ViewModel owns only what genuinely needs server/session state. Port of
 * iOS `SettingsViewModel`.
 */
public class SettingsViewModel(
    private val authenticationService: AuthenticationService,
    private val userProfileRepository: UserProfileRepository,
    private val appearanceCoordinator: AppearanceCoordinator,
    tier: SubscriptionTier,
    appVersion: String,
    signOutSupport: SettingsSignOutSupport = SettingsSignOutSupport(),
) : ViewModel() {
    private val deviceTokenRepository = signOutSupport.deviceTokenRepository
    private val onSignedOut = signOutSupport.onSignedOut

    private val _uiState =
        MutableStateFlow(SettingsUiState(subscriptionTier = tier, appVersion = appVersion))
    public val uiState: StateFlow<SettingsUiState> = _uiState.asStateFlow()

    init {
        appearanceCoordinator.appearance
            .onEach { appearance -> _uiState.update { it.copy(appearance = appearance) } }
            .launchIn(viewModelScope)
    }

    /** Loads account display fields from the current session. [uk.towncrierapp.domain.auth.UserProfile.email] may legitimately be blank (SIWA). */
    public fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true) }
            val session = authenticationService.currentSession()
            _uiState.update {
                it.copy(
                    isLoading = false,
                    email = session?.userProfile?.email,
                    name = session?.userProfile?.name,
                    authMethod = session?.userProfile?.authMethod,
                )
            }
        }
    }

    /** Persists the choice via [AppearanceCoordinator] and restyles the whole app immediately — see [uiState]'s [SettingsUiState.appearance]. */
    public fun setAppearance(value: Appearance) {
        viewModelScope.launch { appearanceCoordinator.setAppearance(value) }
    }

    public fun requestAccountDeletion() {
        _uiState.update { it.copy(isShowingDeleteConfirmation = true) }
    }

    public fun cancelAccountDeletion() {
        _uiState.update { it.copy(isShowingDeleteConfirmation = false, deletionError = null) }
    }

    /**
     * UK GDPR Art. 17 ordering (non-negotiable):
     * 1. `DELETE /v1/me` must succeed FIRST — on failure, an inline retryable
     *    error is shown and the user stays signed in.
     * 2. Only on success: a best-effort device-token removal (never blocks
     *    on failure).
     * 3. Local credential/session wipe via [AuthenticationService.logout].
     * 4. [onSignedOut] fires so the shell can route to Login — the client
     *    never calls Auth0's management API directly; the server cascades it.
     */
    public fun confirmDeleteAccount() {
        _uiState.update { it.copy(isShowingDeleteConfirmation = false, isDeletingAccount = true, deletionError = null) }
        viewModelScope.launch {
            try {
                userProfileRepository.deleteAccount()
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isDeletingAccount = false, deletionError = e) }
                return@launch
            }
            removeDeviceTokenBestEffort()
            authenticationService.logout()
            _uiState.update { it.copy(isDeletingAccount = false) }
            onSignedOut()
        }
    }

    /** Best-effort device-token removal, then the local session wipe — no server erasure involved (contrast [confirmDeleteAccount]). */
    public fun signOut() {
        viewModelScope.launch {
            removeDeviceTokenBestEffort()
            authenticationService.logout()
            onSignedOut()
        }
    }

    /**
     * Fetches the full GDPR data export (`GET /v1/me/data`) and publishes the
     * raw server bytes, unmodified, for the Route to hand to the share
     * sheet. A second call while one is already in flight is ignored.
     */
    public fun exportData() {
        if (_uiState.value.isExporting) return
        _uiState.update { it.copy(isExporting = true, exportError = null, exportedData = null) }
        viewModelScope.launch {
            try {
                val bytes = userProfileRepository.exportData()
                _uiState.update { it.copy(isExporting = false, exportedData = ExportedData(bytes)) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                _uiState.update { it.copy(isExporting = false, exportError = e) }
            }
        }
    }

    /** Clears the shareable artifact once the share sheet has been dismissed. */
    public fun dismissExportShare() {
        _uiState.update { it.copy(exportedData = null) }
    }

    /** Clears the export error once its message has been shown. */
    public fun dismissExportError() {
        _uiState.update { it.copy(exportError = null) }
    }

    @Suppress("SwallowedException", "TooGenericExceptionCaught")
    // Best-effort by design (epic #770 / #778): a missing or failing
    // device-token removal must never block sign-out or account deletion.
    private suspend fun removeDeviceTokenBestEffort() {
        try {
            deviceTokenRepository?.removeDeviceToken()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // Swallowed: see the KDoc above.
        }
    }
}
