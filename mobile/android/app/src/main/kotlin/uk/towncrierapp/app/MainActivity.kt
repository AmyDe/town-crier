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
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.presentation.auth.OnboardingPresentation
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.features.forceupdate.ForceUpdateScreen
import uk.towncrierapp.presentation.features.forceupdate.ForceUpdateViewModel
import uk.towncrierapp.presentation.features.forceupdate.PLAY_STORE_URL
import uk.towncrierapp.presentation.features.login.LoginRoute
import uk.towncrierapp.presentation.features.login.LoginViewModel
import uk.towncrierapp.presentation.R as PresentationR

/** Town Crier's single activity — see android-coding-standards skill: single-activity Compose, no Fragments. */
public class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val appGraph = (application as TownCrierApplication).appGraph
        setContent {
            TownCrierTheme {
                TownCrierApp(appGraph = appGraph)
            }
        }
    }
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
            AuthedShell(appGraph = appGraph, navController = navController)
        }
    }
}

/**
 * The signed-in shell: bottom navigation (Applications, Zones, Saved)
 * wrapping the NavHost. The watch-zone destinations' content lives in
 * `WatchZoneNavGraph.kt`, and the applications-browsing destinations' in
 * `ApplicationNavGraph.kt`, to keep every file under detekt's per-file
 * function-count budget.
 */
@Composable
private fun AuthedShell(
    appGraph: AppGraph,
    navController: NavHostController,
    modifier: Modifier = Modifier,
) {
    val subscriptionTier by appGraph.authCoordinator.subscriptionTier.collectAsStateWithLifecycle()
    val backStackEntry by navController.currentBackStackEntryAsState()

    Scaffold(
        modifier = modifier,
        bottomBar = {
            AuthedBottomBar(currentRoute = backStackEntry?.destination?.route, navController = navController)
        },
    ) { contentPadding ->
        NavHost(
            navController = navController,
            startDestination = Applications,
            modifier = Modifier.padding(contentPadding),
        ) {
            composable<Applications> { ApplicationsTab(appGraph = appGraph, navController = navController) }
            composable<WatchZones> {
                WatchZonesTab(appGraph = appGraph, subscriptionTier = subscriptionTier, navController = navController)
            }
            composable<Saved> { SavedTab(appGraph = appGraph, navController = navController) }
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
