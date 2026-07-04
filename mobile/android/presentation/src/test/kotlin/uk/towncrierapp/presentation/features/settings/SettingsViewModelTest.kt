package uk.towncrierapp.presentation.features.settings

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.AuthMethod
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.auth.FakeAuthenticationService
import uk.towncrierapp.domain.auth.UserProfile
import uk.towncrierapp.domain.auth.anAuthSession
import uk.towncrierapp.domain.devicetoken.FakeDeviceTokenRepository
import uk.towncrierapp.domain.profile.FakeUserProfileRepository
import uk.towncrierapp.domain.settings.FakeAppearanceStore
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.MainDispatcherExtension
import uk.towncrierapp.presentation.appearance.AppearanceCoordinator
import uk.towncrierapp.presentation.designsystem.Appearance
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Port of iOS `SettingsViewModelTests`/`SettingsViewModelAccountDeletionTests`/
 * `SettingsViewModelDataExportTests`, adapted to the Android ports (tc-4jjw).
 */
@ExtendWith(MainDispatcherExtension::class)
class SettingsViewModelTest {
    private fun makeSut(
        authService: FakeAuthenticationService = FakeAuthenticationService(),
        userProfileRepository: FakeUserProfileRepository = FakeUserProfileRepository(),
        deviceTokenRepository: FakeDeviceTokenRepository? = FakeDeviceTokenRepository(),
        tier: SubscriptionTier = SubscriptionTier.FREE,
        onSignedOut: () -> Unit = {},
    ) = SettingsViewModel(
        authenticationService = authService,
        userProfileRepository = userProfileRepository,
        appearanceCoordinator = AppearanceCoordinator(FakeAppearanceStore()),
        tier = tier,
        appVersion = "1.2.3",
        deviceTokenRepository = deviceTokenRepository,
        onSignedOut = onSignedOut,
    )

    // region Account

    @Test
    fun `load populates email name and authMethod from the current session`() {
        val authService =
            FakeAuthenticationService(
                currentSessionResult =
                    anAuthSession(
                        userProfile = UserProfile(userId = "auth0|1", email = "resident@example.test", name = "Res"),
                    ),
            )
        val viewModel = makeSut(authService = authService)

        viewModel.load()

        val state = viewModel.uiState.value
        assertEquals("resident@example.test", state.email)
        assertEquals("Res", state.name)
        assertEquals(AuthMethod.EMAIL_PASSWORD, state.authMethod)
        assertFalse(state.isLoading)
    }

    @Test
    fun `a blank SIWA email renders gracefully rather than as an error`() {
        val authService =
            FakeAuthenticationService(
                currentSessionResult =
                    anAuthSession(
                        userProfile = UserProfile(userId = "apple|1", email = "", name = null),
                    ),
            )
        val viewModel = makeSut(authService = authService)

        viewModel.load()

        assertEquals("", viewModel.uiState.value.email)
        assertEquals(AuthMethod.APPLE, viewModel.uiState.value.authMethod)
    }

    @Test
    fun `the constructor-supplied tier and app version are exposed immediately`() {
        val viewModel = makeSut(tier = SubscriptionTier.PRO)

        assertEquals(SubscriptionTier.PRO, viewModel.uiState.value.subscriptionTier)
        assertEquals("1.2.3", viewModel.uiState.value.appVersion)
    }

    // endregion

    // region Appearance

    @Test
    fun `setAppearance updates the uiState via the shared AppearanceCoordinator`() {
        val coordinator = AppearanceCoordinator(FakeAppearanceStore())
        val viewModel =
            SettingsViewModel(
                authenticationService = FakeAuthenticationService(),
                userProfileRepository = FakeUserProfileRepository(),
                appearanceCoordinator = coordinator,
                tier = SubscriptionTier.FREE,
                appVersion = "1.0",
            )

        viewModel.setAppearance(Appearance.OledDark)

        assertEquals(Appearance.OledDark, viewModel.uiState.value.appearance)
        assertEquals(Appearance.OledDark, coordinator.appearance.value)
    }

    // endregion

    // region Account deletion (GDPR Art. 17 ordering)

    @Test
    fun `requestAccountDeletion shows the confirmation, cancelAccountDeletion dismisses it`() {
        val viewModel = makeSut()

        viewModel.requestAccountDeletion()
        assertTrue(viewModel.uiState.value.isShowingDeleteConfirmation)

        viewModel.cancelAccountDeletion()
        assertFalse(viewModel.uiState.value.isShowingDeleteConfirmation)
    }

    @Test
    fun `a server deletion failure keeps the user signed in with a retryable error`() {
        val authService = FakeAuthenticationService()
        val userProfileRepository =
            FakeUserProfileRepository(deleteAccountResult = Result.failure(DomainError.NetworkUnavailable))
        var signedOutCallCount = 0
        val viewModel =
            makeSut(authService = authService, userProfileRepository = userProfileRepository, onSignedOut = {
                signedOutCallCount++
            })

        viewModel.confirmDeleteAccount()

        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.deletionError)
        assertFalse(viewModel.uiState.value.isDeletingAccount)
        assertTrue(authService.logoutCalls.isEmpty())
        assertEquals(0, signedOutCallCount)
    }

    @Test
    fun `a successful deletion attempts a best-effort device-token removal then wipes the local session`() {
        val authService = FakeAuthenticationService()
        val deviceTokenRepository = FakeDeviceTokenRepository()
        var signedOutCallCount = 0
        val viewModel =
            makeSut(authService = authService, deviceTokenRepository = deviceTokenRepository, onSignedOut = {
                signedOutCallCount++
            })

        viewModel.confirmDeleteAccount()

        assertEquals(1, deviceTokenRepository.removeDeviceTokenCalls.size)
        assertEquals(1, authService.logoutCalls.size)
        assertEquals(1, signedOutCallCount)
        assertNull(viewModel.uiState.value.deletionError)
    }

    @Test
    fun `deletion succeeds even when the device-token removal fails (best-effort)`() {
        val authService = FakeAuthenticationService()
        val deviceTokenRepository =
            FakeDeviceTokenRepository(removeDeviceTokenResult = Result.failure(DomainError.NetworkUnavailable))
        var signedOutCallCount = 0
        val viewModel =
            makeSut(authService = authService, deviceTokenRepository = deviceTokenRepository, onSignedOut = {
                signedOutCallCount++
            })

        viewModel.confirmDeleteAccount()

        assertEquals(1, authService.logoutCalls.size)
        assertEquals(1, signedOutCallCount)
    }

    @Test
    fun `deletion succeeds when no device-token repository has been wired yet (777 not landed)`() {
        val authService = FakeAuthenticationService()
        var signedOutCallCount = 0
        val viewModel =
            makeSut(authService = authService, deviceTokenRepository = null, onSignedOut = { signedOutCallCount++ })

        viewModel.confirmDeleteAccount()

        assertEquals(1, authService.logoutCalls.size)
        assertEquals(1, signedOutCallCount)
    }

    @Test
    fun `a failed deletion is retryable — calling confirmDeleteAccount again re-attempts the server DELETE`() {
        val userProfileRepository =
            FakeUserProfileRepository(deleteAccountResult = Result.failure(DomainError.NetworkUnavailable))
        val viewModel = makeSut(userProfileRepository = userProfileRepository)

        viewModel.confirmDeleteAccount()
        assertEquals(1, userProfileRepository.deleteAccountCalls.size)

        userProfileRepository.deleteAccountResult = Result.success(Unit)
        viewModel.confirmDeleteAccount()

        assertEquals(2, userProfileRepository.deleteAccountCalls.size)
        assertNull(viewModel.uiState.value.deletionError)
    }

    // endregion

    // region Sign out

    @Test
    fun `signOut attempts a best-effort device-token removal then wipes the local session`() {
        val authService = FakeAuthenticationService()
        val deviceTokenRepository = FakeDeviceTokenRepository()
        var signedOutCallCount = 0
        val viewModel =
            makeSut(authService = authService, deviceTokenRepository = deviceTokenRepository, onSignedOut = {
                signedOutCallCount++
            })

        viewModel.signOut()

        assertEquals(1, deviceTokenRepository.removeDeviceTokenCalls.size)
        assertEquals(1, authService.logoutCalls.size)
        assertEquals(1, signedOutCallCount)
    }

    // endregion

    // region Data export

    @Test
    fun `exportData fetches the server bytes byte-for-byte unmodified`() {
        val scriptedBytes = """{"profile":{"userId":"auth0|1"}}""".toByteArray(Charsets.UTF_8)
        val userProfileRepository = FakeUserProfileRepository(exportDataResult = Result.success(scriptedBytes))
        val viewModel = makeSut(userProfileRepository = userProfileRepository)

        viewModel.exportData()

        val exported = viewModel.uiState.value.exportedData
        assertContentEquals(scriptedBytes, exported?.bytes)
        assertFalse(viewModel.uiState.value.isExporting)
    }

    @Test
    fun `exportData failure surfaces an error and produces no artifact`() {
        val userProfileRepository =
            FakeUserProfileRepository(exportDataResult = Result.failure(DomainError.NetworkUnavailable))
        val viewModel = makeSut(userProfileRepository = userProfileRepository)

        viewModel.exportData()

        assertNull(viewModel.uiState.value.exportedData)
        assertEquals(DomainError.NetworkUnavailable, viewModel.uiState.value.exportError)
        assertFalse(viewModel.uiState.value.isExporting)
    }

    @Test
    fun `dismissExportShare clears the exported artifact`() {
        val viewModel = makeSut()
        viewModel.exportData()

        viewModel.dismissExportShare()

        assertNull(viewModel.uiState.value.exportedData)
    }

    @Test
    fun `dismissExportError clears the export error`() {
        val userProfileRepository =
            FakeUserProfileRepository(exportDataResult = Result.failure(DomainError.NetworkUnavailable))
        val viewModel = makeSut(userProfileRepository = userProfileRepository)
        viewModel.exportData()

        viewModel.dismissExportError()

        assertNull(viewModel.uiState.value.exportError)
    }

    // endregion
}
