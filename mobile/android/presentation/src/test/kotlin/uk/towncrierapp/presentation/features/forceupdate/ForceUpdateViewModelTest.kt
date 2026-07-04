package uk.towncrierapp.presentation.features.forceupdate

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.versionconfig.AppVersion
import uk.towncrierapp.domain.versionconfig.FakeVersionConfigService
import uk.towncrierapp.presentation.MainDispatcherExtension
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Pre-login force-update gate: `GET /v1/version-config` vs the running
 * build's version. Port of iOS `ForceUpdateViewModelTests` — a check
 * failure must never block the user (fail open).
 */
@ExtendWith(MainDispatcherExtension::class)
class ForceUpdateViewModelTest {
    @Test
    fun `a running version below the minimum blocks with requiresUpdate`() {
        val service = FakeVersionConfigService(fetchMinimumVersionResult = Result.success(AppVersion(1, 1, 0)))
        val viewModel = ForceUpdateViewModel(service, currentVersion = "1.0.0")

        viewModel.checkVersion()

        assertTrue(viewModel.uiState.value.requiresUpdate)
        assertFalse(viewModel.uiState.value.isChecking)
    }

    @Test
    fun `a running version equal to the minimum proceeds normally`() {
        val service = FakeVersionConfigService(fetchMinimumVersionResult = Result.success(AppVersion(1, 1, 0)))
        val viewModel = ForceUpdateViewModel(service, currentVersion = "1.1.0")

        viewModel.checkVersion()

        assertFalse(viewModel.uiState.value.requiresUpdate)
    }

    @Test
    fun `a running version above the minimum proceeds normally`() {
        val service = FakeVersionConfigService(fetchMinimumVersionResult = Result.success(AppVersion(1, 1, 0)))
        val viewModel = ForceUpdateViewModel(service, currentVersion = "2.0.0")

        viewModel.checkVersion()

        assertFalse(viewModel.uiState.value.requiresUpdate)
    }

    @Test
    fun `a version-config failure never blocks the user`() {
        val service = FakeVersionConfigService(fetchMinimumVersionResult = Result.failure(DomainError.NetworkUnavailable))
        val viewModel = ForceUpdateViewModel(service, currentVersion = "1.0.0")

        viewModel.checkVersion()

        assertFalse(viewModel.uiState.value.requiresUpdate)
        assertFalse(viewModel.uiState.value.isChecking)
    }

    @Test
    fun `a malformed running version string never blocks the user`() {
        val service = FakeVersionConfigService(fetchMinimumVersionResult = Result.success(AppVersion(1, 1, 0)))
        val viewModel = ForceUpdateViewModel(service, currentVersion = "not-a-version")

        viewModel.checkVersion()

        assertFalse(viewModel.uiState.value.requiresUpdate)
    }
}
