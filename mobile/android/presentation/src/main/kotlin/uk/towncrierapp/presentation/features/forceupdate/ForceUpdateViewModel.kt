package uk.towncrierapp.presentation.features.forceupdate

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.versionconfig.AppVersion
import uk.towncrierapp.domain.versionconfig.VersionConfigService
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/**
 * Blocking pre-login version gate: unauthenticated `GET /v1/version-config`
 * versus [currentVersion] (`BuildConfig.VERSION_NAME`, injected from `:app`
 * since `:presentation` cannot see another module's generated BuildConfig).
 * Re-checked on every auth-state transition (epic #770). A failed check
 * never blocks the user — port of iOS `ForceUpdateViewModel`.
 */
public class ForceUpdateViewModel(
    private val versionConfigService: VersionConfigService,
    private val currentVersion: String,
) : ViewModel() {
    private val _uiState = MutableStateFlow(ForceUpdateUiState())
    public val uiState: StateFlow<ForceUpdateUiState> = _uiState.asStateFlow()

    public fun checkVersion() {
        val runningVersion = AppVersion.parse(currentVersion) ?: return
        viewModelScope.launch {
            _uiState.update { it.copy(isChecking = true) }
            try {
                val minimumVersion = versionConfigService.fetchMinimumVersion()
                _uiState.update { it.copy(isChecking = false, requiresUpdate = runningVersion < minimumVersion) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: DomainError) {
                // Fail open — a version-config outage must never lock users out.
                _uiState.update { it.copy(isChecking = false) }
            }
        }
    }
}
