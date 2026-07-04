// ApplicationNavGraph.kt is the bead-mandated file name for every
// applications-browsing nav destination + composable (mirrors
// WatchZoneNavGraph.kt) — ApplicationDetailDestination below is only ONE of
// several top-level declarations this file thematically groups, not its sole
// declaration in spirit.
@file:Suppress("MatchingDeclarationName")

package uk.towncrierapp.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.toRoute
import kotlinx.serialization.Serializable
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.LocalAuthority
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.StatusEvent
import uk.towncrierapp.domain.applications.isDecided
import uk.towncrierapp.domain.applications.wireValue
import uk.towncrierapp.domain.reviewprompt.ReviewSignal
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.presentation.features.applicationdetail.ApplicationDetailRoute
import uk.towncrierapp.presentation.features.applicationdetail.ApplicationDetailViewModel
import uk.towncrierapp.presentation.features.applicationlist.ApplicationListRoute
import uk.towncrierapp.presentation.features.applicationlist.ApplicationListViewModel
import uk.towncrierapp.presentation.features.saved.SavedListRoute
import uk.towncrierapp.presentation.features.saved.SavedListViewModel
import java.time.LocalDate

/**
 * The application-detail destination: every field flattened to a primitive
 * nav arg — type-safe Navigation routes only support serializable
 * primitives, and the domain `PlanningApplication` deliberately isn't
 * `@Serializable` (see android-coding-standards skill, data-access.md on
 * domain/DTO separation; same pattern as `WatchZoneEditorDestination`). This
 * is what makes stale-while-revalidate possible: the tapped row's full data
 * travels with the navigation, so [ApplicationDetailViewModel] can render it
 * before its background by-id refresh completes.
 */
@Serializable
internal data class ApplicationDetailDestination(
    val authority: String,
    val name: String,
    val authorityName: String,
    val status: String,
    val receivedDate: String,
    val description: String,
    val address: String,
    val latitude: Double? = null,
    val longitude: Double? = null,
    val portalUrl: String? = null,
    val decidedDate: String? = null,
)

internal fun applicationDetailDestinationFor(application: PlanningApplication): ApplicationDetailDestination =
    ApplicationDetailDestination(
        authority = application.id.authority,
        name = application.id.name,
        authorityName = application.authority.name,
        status = application.status.wireValue,
        receivedDate = application.receivedDate.toString(),
        description = application.description,
        address = application.address,
        latitude = application.location?.latitude,
        longitude = application.location?.longitude,
        portalUrl = application.portalUrl,
        // At most 2 statusHistory points by construction (GH#775): index 0 is
        // always the Undecided/submitted event, index 1 (if present) is the
        // decided one.
        decidedDate =
            application.statusHistory
                .getOrNull(1)
                ?.date
                ?.toString(),
    )

private fun ApplicationDetailDestination.toInitialApplication(): PlanningApplication {
    val resolvedStatus = ApplicationStatus.fromWireValue(status)
    val received = LocalDate.parse(receivedDate)
    val history =
        buildList {
            add(StatusEvent(ApplicationStatus.Undecided, received))
            if (resolvedStatus.isDecided) {
                decidedDate?.let { add(StatusEvent(resolvedStatus, LocalDate.parse(it))) }
            }
        }
    return PlanningApplication(
        id = PlanningApplicationId(authority, name),
        reference = name,
        authority = LocalAuthority(code = authority, name = authorityName),
        status = resolvedStatus,
        receivedDate = received,
        description = description,
        address = address,
        location = if (latitude != null && longitude != null) Coordinate(latitude, longitude) else null,
        portalUrl = portalUrl,
        statusHistory = history,
    )
}

/** The Applications tab (GH#775) — first bottom-nav destination, replacing the #771 Home placeholder. */
@Composable
internal fun ApplicationsTab(
    appGraph: AppGraph,
    navController: NavHostController,
    onSettingsClick: () -> Unit,
) {
    val listViewModel: ApplicationListViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        ApplicationListViewModel(
                            appGraph.planningApplicationRepository,
                            appGraph.watchZoneRepository,
                            appGraph.notificationStateRepository,
                            appGraph.applicationCacheStore,
                            appGraph.applicationListPreferencesStore,
                        )
                    }
                },
        )
    LaunchedEffect(listViewModel) { listViewModel.load() }
    ApplicationListRoute(
        viewModel = listViewModel,
        onApplicationSelected = { application -> navController.navigate(applicationDetailDestinationFor(application)) },
        onAddZoneClick = { navController.navigate(WatchZoneEditorDestination()) },
        onSettingsClick = onSettingsClick,
    )
}

/** The Saved tab (GH#775). */
@Composable
internal fun SavedTab(
    appGraph: AppGraph,
    navController: NavHostController,
    onSettingsClick: () -> Unit,
) {
    val savedViewModel: SavedListViewModel =
        viewModel(
            factory = viewModelFactory { initializer { SavedListViewModel(appGraph.savedApplicationRepository) } },
        )
    LaunchedEffect(savedViewModel) { savedViewModel.load() }
    SavedListRoute(
        viewModel = savedViewModel,
        onApplicationSelected = { application -> navController.navigate(applicationDetailDestinationFor(application)) },
        onSettingsClick = onSettingsClick,
    )
}

/** Application detail — a full navigation destination reachable from any tab (GH#775 pre-resolved decision: not a bottom sheet). */
@Composable
internal fun ApplicationDetailDestinationContent(
    entry: NavBackStackEntry,
    appGraph: AppGraph,
    navController: NavHostController,
) {
    val route = entry.toRoute<ApplicationDetailDestination>()
    val detailViewModel: ApplicationDetailViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        ApplicationDetailViewModel(
                            appGraph.planningApplicationRepository,
                            appGraph.savedApplicationRepository,
                            route.toInitialApplication(),
                            // The review-prompt savedApplication signal call site
                            // (GH #628 / tc-4jjw): fires only on a successful
                            // false-to-true save, see ApplicationDetailViewModel.toggleSave.
                            onSaved = { appGraph.reviewPromptTracker.recordSignal(ReviewSignal.SavedApplication) },
                        )
                    }
                },
        )
    ApplicationDetailRoute(viewModel = detailViewModel, onBack = navController::popBackStack)
}
