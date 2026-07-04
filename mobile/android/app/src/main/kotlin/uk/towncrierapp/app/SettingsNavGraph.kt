// SettingsNavGraph.kt is the bead-mandated file name for every
// settings-area nav destination + composable (mirrors WatchZoneNavGraph.kt /
// ApplicationNavGraph.kt) — several top-level declarations below, not a
// single one, same pattern as its siblings.
@file:Suppress("MatchingDeclarationName")

package uk.towncrierapp.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.toRoute
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import uk.towncrierapp.domain.legal.LegalDocumentType
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.features.legal.LegalDocumentRoute
import uk.towncrierapp.presentation.features.legal.LegalDocumentViewModel
import uk.towncrierapp.presentation.features.login.LoginViewModel
import uk.towncrierapp.presentation.features.notificationprefs.NotificationPreferencesRoute
import uk.towncrierapp.presentation.features.notificationprefs.NotificationPreferencesViewModel
import uk.towncrierapp.presentation.features.settings.SettingsRoute
import uk.towncrierapp.presentation.features.settings.SettingsSignOutSupport
import uk.towncrierapp.presentation.features.settings.SettingsViewModel

/** The Settings destination — reachable from the settings icon on every bottom-nav-hosting screen. */
@Serializable
internal data object SettingsDestination

/** The in-app notification-preferences destination, reached from a Settings row. */
@Serializable
internal data object NotificationPreferencesDestination

/**
 * A bundled legal document (privacy policy or terms of service), reached
 * from a Settings row. [documentType] carries [LegalDocumentType]'s enum
 * name — type-safe Navigation routes only support serializable primitives
 * (same pattern as `ApplicationDetailDestination`'s flattened fields).
 */
@Serializable
internal data class LegalDocumentDestination(
    val documentType: String,
)

private fun LegalDocumentType.toDestination() = LegalDocumentDestination(documentType = name)

private fun LegalDocumentDestination.toType(): LegalDocumentType = LegalDocumentType.valueOf(documentType)

/** The Settings screen: account, appearance, notification prefs entry, subscription, legal, export, sign-out/deletion, about. */
@Composable
internal fun SettingsDestinationContent(
    appGraph: AppGraph,
    subscriptionTier: SubscriptionTier,
    loginViewModel: LoginViewModel,
    navController: NavHostController,
) {
    val context = LocalContext.current
    val coroutineScope = rememberCoroutineScope()
    val settingsViewModel: SettingsViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        SettingsViewModel(
                            authenticationService = appGraph.authenticationService,
                            userProfileRepository = appGraph.userProfileRepository,
                            appearanceCoordinator = appGraph.appearanceCoordinator,
                            tier = subscriptionTier,
                            appVersion = appGraph.currentVersion,
                            signOutSupport =
                                SettingsSignOutSupport(
                                    deviceTokenRepository = appGraph.deviceTokenRepository,
                                    // Resets the shell's own auth state WITHOUT a second
                                    // AuthenticationService.logout() call — see
                                    // LoginViewModel.markSignedOut's doc (tc-4jjw).
                                    onSignedOut = loginViewModel::markSignedOut,
                                ),
                        )
                    }
                },
        )
    SettingsRoute(
        viewModel = settingsViewModel,
        onBack = navController::popBackStack,
        onNotificationPreferencesClick = { navController.navigate(NotificationPreferencesDestination) },
        onPrivacyPolicyClick = { navController.navigate(LegalDocumentType.PRIVACY_POLICY.toDestination()) },
        onTermsOfServiceClick = { navController.navigate(LegalDocumentType.TERMS_OF_SERVICE.toDestination()) },
        onRateAppClick = {
            coroutineScope.launch { requestReviewOrOpenStoreListing(context, appGraph.activityProvider) }
        },
    )
}

@Composable
internal fun NotificationPreferencesDestinationContent(
    appGraph: AppGraph,
    navController: NavHostController,
) {
    val viewModel: NotificationPreferencesViewModel =
        viewModel(
            factory =
                viewModelFactory { initializer { NotificationPreferencesViewModel(appGraph.userProfileRepository) } },
        )
    LaunchedEffect(viewModel) { viewModel.load() }
    NotificationPreferencesRoute(viewModel = viewModel, onBack = navController::popBackStack)
}

@Composable
internal fun LegalDocumentDestinationContent(
    entry: NavBackStackEntry,
    appGraph: AppGraph,
    navController: NavHostController,
) {
    val route = entry.toRoute<LegalDocumentDestination>()
    val viewModel: LegalDocumentViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer { LegalDocumentViewModel(appGraph.legalDocumentRepository, route.toType()) }
                },
        )
    LegalDocumentRoute(viewModel = viewModel, onBack = navController::popBackStack)
}
