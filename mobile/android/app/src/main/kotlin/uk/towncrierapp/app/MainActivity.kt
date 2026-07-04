package uk.towncrierapp.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.features.forceupdate.ForceUpdateScreen
import uk.towncrierapp.presentation.features.forceupdate.ForceUpdateViewModel
import uk.towncrierapp.presentation.features.forceupdate.PLAY_STORE_URL
import uk.towncrierapp.presentation.features.login.LoginRoute
import uk.towncrierapp.presentation.features.login.LoginViewModel
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
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
import androidx.navigation.compose.rememberNavController
import kotlinx.serialization.Serializable
import uk.towncrierapp.mobile.R

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

/** The one placeholder destination this scaffold ships — real screens land in later phases of epic #770. */
@Serializable
private data object Home

/**
 * Root routing: pre-login force-update gate, then Login or the authed
 * shell — port of iOS `TownCrierApp.body`'s `Group { if requiresUpdate ...
 * else if isAuthenticated ... else Login }`. Re-checks the version gate and
 * re-runs the signed-in sequence (ensure profile → resolve tier, #549 /
 * tc-f2il) on every auth-state transition, keyed off `isAuthenticated` so a
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
    val context = LocalContext.current

    LaunchedEffect(loginState.isAuthenticated) {
        forceUpdateViewModel.checkVersion()
        if (loginState.isAuthenticated) {
            appGraph.authCoordinator.onSignedIn()
        }
    }

    Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        when {
            forceUpdateState.requiresUpdate ->
                ForceUpdateScreen(
                    onUpdateClick = { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(PLAY_STORE_URL))) },
                )
            loginState.isAuthenticated ->
                NavHost(navController = navController, startDestination = Home) {
                    composable<Home> { HomeRoute() }
                }
            else -> LoginRoute(viewModel = loginViewModel)
        }
    }
}

@Composable
private fun HomeRoute(modifier: Modifier = Modifier) {
    HomeScreen(modifier = modifier)
}

@Composable
internal fun HomeScreen(modifier: Modifier = Modifier) {
    Box(modifier = modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Text(
            text = stringResource(R.string.app_name),
            style = MaterialTheme.typography.headlineLarge,
            color = MaterialTheme.colorScheme.onBackground,
        )
    }
}
