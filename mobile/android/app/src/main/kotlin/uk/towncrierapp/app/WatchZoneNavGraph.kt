package uk.towncrierapp.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.key
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.toRoute
import kotlinx.serialization.Serializable
import uk.towncrierapp.domain.subscriptions.FeatureGate
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.presentation.features.watchzones.WatchZoneEditorRoute
import uk.towncrierapp.presentation.features.watchzones.WatchZoneEditorUiState
import uk.towncrierapp.presentation.features.watchzones.WatchZoneEditorViewModel
import uk.towncrierapp.presentation.features.watchzones.WatchZoneListViewModel
import uk.towncrierapp.presentation.features.watchzones.WatchZonesRoute
import uk.towncrierapp.presentation.features.watchzones.ZonePreferencesRoute
import uk.towncrierapp.presentation.features.watchzones.ZonePreferencesViewModel

/**
 * The watch-zone editor destination. `zoneId == null` means "create"; a
 * non-null [zoneId] carries the rest of the zone's fields as plain nav args
 * (type-safe Navigation routes only support serializable primitives, and the
 * domain `WatchZone` deliberately isn't `@Serializable` — see
 * android-coding-standards skill, data-access.md on domain/DTO separation).
 */
@Serializable
internal data class WatchZoneEditorDestination(
    val zoneId: String? = null,
    val name: String? = null,
    val latitude: Double? = null,
    val longitude: Double? = null,
    val radiusMetres: Double? = null,
    val authorityId: Int? = null,
    val pushEnabled: Boolean? = null,
    val emailInstantEnabled: Boolean? = null,
)

/** The per-zone notification-preferences destination, reached from a watch-zone row. */
@Serializable
internal data class ZonePreferencesDestination(
    val zoneId: String,
    val zoneName: String,
)

internal fun watchZoneEditorDestinationFor(zone: WatchZone) =
    WatchZoneEditorDestination(
        zoneId = zone.id.value,
        name = zone.name,
        latitude = zone.centre.latitude,
        longitude = zone.centre.longitude,
        radiusMetres = zone.radiusMetres,
        authorityId = zone.authorityId,
        pushEnabled = zone.pushEnabled,
        emailInstantEnabled = zone.emailInstantEnabled,
    )

private fun WatchZoneEditorDestination.toEditingZone(): WatchZone? {
    val id = zoneId ?: return null
    return WatchZone(
        id = WatchZoneId(id),
        name = name.orEmpty(),
        centre = Coordinate(latitude ?: 0.0, longitude ?: 0.0),
        radiusMetres = radiusMetres ?: WatchZoneEditorUiState.DEFAULT_RADIUS_METRES.toDouble(),
        authorityId = authorityId ?: 0,
        pushEnabled = pushEnabled ?: true,
        emailInstantEnabled = emailInstantEnabled ?: true,
    )
}

/**
 * The watch-zones tab: rebuilds [WatchZoneListViewModel] whenever
 * [subscriptionTier] changes (`key(subscriptionTier)`) so the upgrade
 * badge/upsell state never goes stale after a tier change (tc-ujct parity).
 */
@Composable
internal fun WatchZonesTab(
    appGraph: AppGraph,
    subscriptionTier: SubscriptionTier,
    navController: NavHostController,
) {
    val listViewModel: WatchZoneListViewModel =
        key(subscriptionTier) {
            viewModel(
                factory =
                    viewModelFactory {
                        initializer {
                            WatchZoneListViewModel(
                                appGraph.watchZoneRepository,
                                FeatureGate(subscriptionTier),
                            )
                        }
                    },
            )
        }
    LaunchedEffect(listViewModel) { listViewModel.load() }
    WatchZonesRoute(
        viewModel = listViewModel,
        onZoneSelected = { zone -> navController.navigate(watchZoneEditorDestinationFor(zone)) },
        onZonePreferencesSelected = { zone ->
            navController.navigate(ZonePreferencesDestination(zoneId = zone.id.value, zoneName = zone.name))
        },
        onAddZone = { navController.navigate(WatchZoneEditorDestination()) },
    )
}

/** Create/edit a watch zone. Save-403 / dismiss-on-save routing is state the ViewModel exposes; see `WatchZoneEditorRoute`. */
@Composable
internal fun WatchZoneEditorDestinationContent(
    entry: NavBackStackEntry,
    appGraph: AppGraph,
    subscriptionTier: SubscriptionTier,
    navController: NavHostController,
) {
    val route = entry.toRoute<WatchZoneEditorDestination>()
    val editingZone = route.toEditingZone()
    val editorViewModel: WatchZoneEditorViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        WatchZoneEditorViewModel(
                            appGraph.postcodeGeocoder,
                            appGraph.watchZoneRepository,
                            subscriptionTier,
                            editingZone,
                        )
                    }
                },
        )
    WatchZoneEditorRoute(
        viewModel = editorViewModel,
        onDismiss = navController::popBackStack,
        // #783 hasn't shipped the paywall yet — dismiss the editor (matching
        // the "quota breach" UX) and no-op the rest.
        onUpgradeRequired = { navController.popBackStack() },
    )
}

/** Per-zone notification preferences, reached from a watch-zone row. */
@Composable
internal fun ZonePreferencesDestinationContent(
    entry: NavBackStackEntry,
    appGraph: AppGraph,
    subscriptionTier: SubscriptionTier,
    navController: NavHostController,
) {
    val route = entry.toRoute<ZonePreferencesDestination>()
    val preferencesViewModel: ZonePreferencesViewModel =
        viewModel(
            factory =
                viewModelFactory {
                    initializer {
                        ZonePreferencesViewModel(
                            appGraph.zonePreferencesRepository,
                            WatchZoneId(route.zoneId),
                            route.zoneName,
                            subscriptionTier,
                        )
                    }
                },
        )
    ZonePreferencesRoute(viewModel = preferencesViewModel, onDismiss = navController::popBackStack)
}
