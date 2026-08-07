package uk.towncrierapp.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavHostController
import uk.towncrierapp.presentation.features.map.MapRoute
import uk.towncrierapp.presentation.features.map.MapViewModel

/** The Map tab (GH#776) — 4th bottom-nav destination. Full application detail (#775) is an existing destination, so a "View full details" tap navigates straight there. */
@Composable
internal fun MapTab(
    appGraph: AppGraph,
    navController: NavHostController,
    onSettingsClick: () -> Unit,
) {
    val mapViewModel: MapViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        MapViewModel(
                            appGraph.planningApplicationRepository,
                            appGraph.watchZoneRepository,
                            appGraph.savedApplicationRepository,
                            appGraph.mapPreferencesStore,
                        )
                    }
                },
        )
    LaunchedEffect(mapViewModel) { mapViewModel.load() }
    MapRoute(
        viewModel = mapViewModel,
        onSettingsClick = onSettingsClick,
        onViewFullDetails = { application -> navController.navigate(applicationDetailDestinationFor(application)) },
    )
}
