package uk.towncrierapp.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.Place
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import kotlinx.serialization.Serializable
import uk.towncrierapp.domain.reviewprompt.ReviewSignal
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.auth.OnboardingPresentation
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.features.forceupdate.ForceUpdateScreen
import uk.towncrierapp.presentation.features.forceupdate.ForceUpdateViewModel
import uk.towncrierapp.presentation.features.forceupdate.PLAY_STORE_URL
import uk.towncrierapp.presentation.features.login.LoginRoute
import uk.towncrierapp.presentation.features.login.LoginViewModel
import uk.towncrierapp.presentation.sharing.AppLinkParser
import uk.towncrierapp.presentation.sharing.DeepLinkResolution
import uk.towncrierapp.presentation.sharing.DeepLinkViewModel
import uk.towncrierapp.presentation.R as PresentationR

/**
 * Town Crier's single activity — see android-coding-standards skill:
 * single-activity Compose, no Fragments. `launchMode="singleTask"`
 * (AndroidManifest.xml) keeps App Link taps routed to this one instance via
 * [onNewIntent] rather than spawning a second activity on top (GH#782).
 */
public class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val appGraph = (application as TownCrierApplication).appGraph
        handleAppLinkIntent(appGraph, intent)
        setContent {
            // The four-way appearance picker (tc-4jjw / #778): restyles the
            // whole app the instant `SettingsViewModel.setAppearance` writes
            // a new value, since this StateFlow IS what feeds TownCrierTheme.
            val appearance by appGraph.appearanceCoordinator.appearance.collectAsStateWithLifecycle()
            LaunchedEffect(Unit) { appGraph.appearanceCoordinator.load() }
            TownCrierTheme(appearance = appearance) {
                TownCrierApp(appGraph = appGraph)
            }
        }
    }

    /** Re-delivery to the already-running singleTask instance (a link tapped while the app is open, GH#782). */
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleAppLinkIntent((application as TownCrierApplication).appGraph, intent)
    }
}

/**
 * Parses [intent]'s data (if any) as an App Link and, if recognised, hands it
 * to [AppGraph.pendingLinkHolder] — held there until the user is
 * authenticated, then dispatched by [TownCrierApp] (GH#782). Not every
 * inbound intent carries link data (e.g. the plain launcher intent), and not
 * every URL this activity could theoretically receive matches one of the
 * three recognised shapes — both are silently ignored, matching iOS
 * `OpenURLRoute.resolve`'s "no match falls through" contract.
 */
private fun handleAppLinkIntent(
    appGraph: AppGraph,
    intent: Intent,
) {
    val data = intent.dataString ?: return
    AppLinkParser.parse(data)?.let(appGraph.pendingLinkHolder::linkReceived)
}

/**
 * The Applications tab (GH#775) — first bottom-nav destination, replacing the
 * #771 Home placeholder now that there's a real daily-use surface to land on.
 */
@Serializable
internal data object Applications

/** The watch-zones tab (tc-z95t) — 2nd bottom-nav destination. */
@Serializable
internal data object WatchZones

/** The Saved tab (GH#775) — 3rd bottom-nav destination. */
@Serializable
internal data object Saved

// A no-arg `@Serializable data object` route's KSerializer serialName is its
// fully qualified class name — comparing against it is a simpler, equally
// reliable way to tell "is this bottom-nav tab currently selected" than the
// generic `NavDestination.hasRoute<T>()` extension.
private val APPLICATIONS_ROUTE = Applications::class.qualifiedName
private val WATCH_ZONES_ROUTE = WatchZones::class.qualifiedName
private val SAVED_ROUTE = Saved::class.qualifiedName

/**
 * Root routing: pre-login force-update gate, then Login, then - once
 * authenticated - the onboarding gate (tc-7ttz: `Undetermined` shows a
 * loading screen, never a flash of the wrong state) before the authed shell.
 * Port of iOS `TownCrierApp.body`'s `Group { if requiresUpdate ... else if
 * isAuthenticated ... else Login }` extended by `AppCoordinator+Onboarding`.
 * Re-checks the version gate and re-runs the signed-in sequence (ensure
 * profile → resolve tier → resolve the onboarding gate, #549 / tc-f2il /
 * tc-7ttz) on every auth-state transition, keyed off `isAuthenticated` so a
 * fresh sign-in re-triggers it.
 */
@Composable
public fun TownCrierApp(
    appGraph: AppGraph,
    navController: NavHostController = rememberNavController(),
) {
    val loginViewModel: LoginViewModel =
        viewModel(factory = viewModelFactory { initializer { LoginViewModel(appGraph.authenticationService) } })
    val forceUpdateViewModel: ForceUpdateViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer { ForceUpdateViewModel(appGraph.versionConfigService, appGraph.currentVersion) }
                },
        )

    val loginState by loginViewModel.uiState.collectAsStateWithLifecycle()
    val forceUpdateState by forceUpdateViewModel.uiState.collectAsStateWithLifecycle()
    val onboardingPresentation by appGraph.authCoordinator.onboardingPresentation.collectAsStateWithLifecycle()
    val subscriptionTier by appGraph.authCoordinator.subscriptionTier.collectAsStateWithLifecycle()
    val context = LocalContext.current

    LaunchedEffect(loginState.isAuthenticated) {
        forceUpdateViewModel.checkVersion()
        if (loginState.isAuthenticated) {
            appGraph.authCoordinator.onSignedIn()
        }
        // Auth-gated App Link dispatch (GH#782): a link tapped signed-out is
        // held, never resolved, until this transitions to true — see
        // PendingLinkHolder's doc for the "no signed-out detail view day-1" rationale.
        appGraph.pendingLinkHolder.onAuthenticationChanged(loginState.isAuthenticated)
    }

    Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        when {
            forceUpdateState.requiresUpdate -> {
                ForceUpdateScreen(
                    onUpdateClick = { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(PLAY_STORE_URL))) },
                )
            }

            loginState.isAuthenticated -> {
                AuthedContent(
                    appGraph = appGraph,
                    navController = navController,
                    onboardingPresentation = onboardingPresentation,
                    subscriptionTier = subscriptionTier,
                    loginViewModel = loginViewModel,
                )
            }

            else -> {
                LoginRoute(viewModel = loginViewModel)
            }
        }
    }
}

/** Onboarding gate, evaluated only once signed in - see [TownCrierApp]'s doc for the full ordering contract. */
@Composable
private fun AuthedContent(
    appGraph: AppGraph,
    navController: NavHostController,
    onboardingPresentation: OnboardingPresentation,
    subscriptionTier: SubscriptionTier,
    loginViewModel: LoginViewModel,
) {
    when (onboardingPresentation) {
        OnboardingPresentation.Undetermined -> {
            OnboardingLoadingScreen()
        }

        OnboardingPresentation.Required -> {
            OnboardingGate(
                appGraph = appGraph,
                subscriptionTier = subscriptionTier,
                onOnboardingComplete = appGraph.authCoordinator::onOnboardingCompleted,
            )
        }

        OnboardingPresentation.NotRequired -> {
            AuthedShell(appGraph = appGraph, navController = navController, loginViewModel = loginViewModel)
        }
    }
}

/**
 * The signed-in shell: bottom navigation (Applications, Zones, Saved)
 * wrapping the NavHost. Each of the three bottom-nav tabs' own existing top
 * app bar gets a settings action (epic #770's Material-native-idiom
 * chapter) via [onSettingsClick] — not a second, wrapping top bar, which
 * would double up with `ApplicationListScreen`/`WatchZoneListScreen`/
 * `SavedListScreen`'s already-present `TopAppBar`s. The watch-zone
 * destinations' content lives in `WatchZoneNavGraph.kt`, the
 * applications-browsing destinations' in `ApplicationNavGraph.kt`, and the
 * settings-area destinations' in `SettingsNavGraph.kt`, to keep every file
 * under detekt's per-file function-count budget.
 */
@Composable
private fun AuthedShell(
    appGraph: AppGraph,
    navController: NavHostController,
    loginViewModel: LoginViewModel,
    modifier: Modifier = Modifier,
) {
    val subscriptionTier by appGraph.authCoordinator.subscriptionTier.collectAsStateWithLifecycle()
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route

    DeepLinkDispatcher(appGraph = appGraph, navController = navController)

    Scaffold(
        modifier = modifier,
        bottomBar = { AuthedBottomBar(currentRoute = currentRoute, navController = navController) },
    ) { contentPadding ->
        AuthedNavHost(
            appGraph = appGraph,
            navController = navController,
            loginViewModel = loginViewModel,
            subscriptionTier = subscriptionTier,
            modifier = Modifier.padding(contentPadding),
        )
    }
}

/**
 * Observes [AppGraph.pendingLinkHolder]'s ready link (only ever non-null once
 * signed in — see `PendingLinkHolder`'s doc), resolves it via
 * [DeepLinkViewModel], and navigates the moment a resolution lands. A no-UI
 * composable (GH#782) — kept separate from [AuthedShell] so its two
 * `LaunchedEffect`s don't crowd that function's own state reads. A failed
 * resolution (e.g. an unknown/expired share link) is dropped silently —
 * best-effort, matching `ApplicationDetailViewModel.checkSavedState`'s
 * precedent for a background fetch the user didn't directly initiate.
 */
@Composable
private fun DeepLinkDispatcher(
    appGraph: AppGraph,
    navController: NavHostController,
) {
    val deepLinkViewModel: DeepLinkViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        DeepLinkViewModel(
                            appGraph.planningApplicationRepository,
                            // openedAlert fires only for the by-id path (push taps and
                            // legacy /applications/{uid} links) — never for a public
                            // share link (GH#782, iOS AppCoordinator.handleDeepLink parity).
                            onOpenedAlert = { appGraph.reviewPromptTracker.recordSignal(ReviewSignal.OpenedAlert) },
                        )
                    }
                },
        )
    val readyLink by appGraph.pendingLinkHolder.readyLink.collectAsStateWithLifecycle()
    val deepLinkState by deepLinkViewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(readyLink) {
        readyLink?.let {
            deepLinkViewModel.resolve(it)
            appGraph.pendingLinkHolder.consume()
        }
    }

    LaunchedEffect(deepLinkState.resolution) {
        val resolution = deepLinkState.resolution ?: return@LaunchedEffect
        when (resolution) {
            is DeepLinkResolution.ShowApplication -> {
                navController.navigateToTab(Applications)
                navController.navigate(applicationDetailDestinationFor(resolution.application))
            }
            DeepLinkResolution.ShowApplicationsList -> navController.navigateToTab(Applications)
        }
        deepLinkViewModel.consumeResolution()
    }
}

@Composable
private fun AuthedNavHost(
    appGraph: AppGraph,
    navController: NavHostController,
    loginViewModel: LoginViewModel,
    subscriptionTier: SubscriptionTier,
    modifier: Modifier = Modifier,
) {
    val onSettingsClick: () -> Unit = { navController.navigate(SettingsDestination) }
    NavHost(navController = navController, startDestination = Applications, modifier = modifier) {
        composable<Applications> {
            ApplicationsTab(appGraph = appGraph, navController = navController, onSettingsClick = onSettingsClick)
        }
        composable<WatchZones> { backStackEntry ->
            WatchZonesTab(
                appGraph = appGraph,
                subscriptionTier = subscriptionTier,
                navController = navController,
                onSettingsClick = onSettingsClick,
                backStackEntry = backStackEntry,
            )
        }
        composable<Saved> {
            SavedTab(appGraph = appGraph, navController = navController, onSettingsClick = onSettingsClick)
        }
        composable<WatchZoneEditorDestination> { entry ->
            WatchZoneEditorDestinationContent(
                entry = entry,
                appGraph = appGraph,
                subscriptionTier = subscriptionTier,
                navController = navController,
            )
        }
        composable<ZonePreferencesDestination> { entry ->
            ZonePreferencesDestinationContent(
                entry = entry,
                appGraph = appGraph,
                subscriptionTier = subscriptionTier,
                navController = navController,
            )
        }
        composable<ApplicationDetailDestination> { entry ->
            ApplicationDetailDestinationContent(entry = entry, appGraph = appGraph, navController = navController)
        }
        composable<SettingsDestination> {
            SettingsDestinationContent(
                appGraph = appGraph,
                subscriptionTier = subscriptionTier,
                loginViewModel = loginViewModel,
                navController = navController,
            )
        }
        composable<NotificationPreferencesDestination> {
            NotificationPreferencesDestinationContent(appGraph = appGraph, navController = navController)
        }
        composable<LegalDocumentDestination> { entry ->
            LegalDocumentDestinationContent(entry = entry, appGraph = appGraph, navController = navController)
        }
    }
}

@Composable
private fun AuthedBottomBar(
    currentRoute: String?,
    navController: NavHostController,
) {
    NavigationBar {
        NavigationBarItem(
            selected = currentRoute == APPLICATIONS_ROUTE,
            onClick = { navController.navigateToTab(Applications) },
            icon = { Icon(imageVector = Icons.AutoMirrored.Filled.List, contentDescription = null) },
            label = { Text(stringResource(PresentationR.string.bottom_nav_applications)) },
        )
        NavigationBarItem(
            selected = currentRoute == WATCH_ZONES_ROUTE,
            onClick = { navController.navigateToTab(WatchZones) },
            icon = { Icon(imageVector = Icons.Filled.Place, contentDescription = null) },
            label = { Text(stringResource(PresentationR.string.bottom_nav_zones)) },
        )
        NavigationBarItem(
            selected = currentRoute == SAVED_ROUTE,
            onClick = { navController.navigateToTab(Saved) },
            icon = { Icon(imageVector = Icons.Filled.Star, contentDescription = null) },
            label = { Text(stringResource(PresentationR.string.bottom_nav_saved)) },
        )
    }
}

/** Standard bottom-nav tab switch: single top-of-stack instance per tab, restoring saved state. */
internal fun NavHostController.navigateToTab(route: Any) {
    navigate(route) {
        popUpTo(graph.startDestinationId) { saveState = true }
        launchSingleTop = true
        restoreState = true
    }
}
