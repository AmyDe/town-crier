package uk.towncrierapp.presentation.features.watchzones

import android.content.res.Configuration
import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.domain.watchzones.WatchZone
import uk.towncrierapp.domain.watchzones.WatchZoneId
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

// WatchZoneListScreen's previews, split out to keep WatchZoneListScreen.kt
// under detekt's per-file function-count budget (same pattern as
// WatchZoneEditorScreen.kt / WatchZoneEditorSections.kt).

// Preview-only sample data — cannot reuse :domain's testFixtures from the
// main source set (compose-ui.md: previews can't see test/testFixtures
// source sets), so a small duplicate lives here instead.
private val previewHomeZone =
    WatchZone(
        id = WatchZoneId("preview-home"),
        name = "Home",
        centre = Coordinate(51.5074, -0.1278),
        radiusMetres = 500.0,
    )
private val previewOfficeZone =
    WatchZone(
        id = WatchZoneId("preview-office"),
        name = "Office",
        centre = Coordinate(51.5155, -0.0922),
        radiusMetres = 1_500.0,
    )
private val previewParentsZone =
    WatchZone(
        id = WatchZoneId("preview-parents"),
        name = "Parents",
        centre = Coordinate(52.2053, 0.1218),
        radiusMetres = 2_000.0,
    )

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun WatchZoneListScreenPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state = WatchZoneListUiState(zones = listOf(previewHomeZone, previewOfficeZone)),
            onZoneSelected = {},
            onZonePreferencesSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
            onSettingsClick = {},
        )
    }
}

@Preview(name = "empty")
@Composable
private fun WatchZoneListScreenEmptyPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state = WatchZoneListUiState(),
            onZoneSelected = {},
            onZonePreferencesSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
            onSettingsClick = {},
        )
    }
}

@Preview(name = "free tier at cap — badge + inline upsell")
@Composable
private fun WatchZoneListScreenFreeAtCapPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state =
                WatchZoneListUiState(
                    zones = listOf(previewHomeZone),
                    canAddZone = false,
                    showUpgradeBadge = true,
                    showsFreeTierUpsell = true,
                ),
            onZoneSelected = {},
            onZonePreferencesSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
            onSettingsClick = {},
        )
    }
}

@Preview(name = "personal tier at cap — badge only")
@Composable
private fun WatchZoneListScreenPersonalAtCapPreview() {
    TownCrierTheme {
        WatchZoneListScreen(
            state =
                WatchZoneListUiState(
                    zones = listOf(previewHomeZone, previewOfficeZone, previewParentsZone),
                    canAddZone = false,
                    showUpgradeBadge = true,
                    showsFreeTierUpsell = false,
                ),
            onZoneSelected = {},
            onZonePreferencesSelected = {},
            onDeleteZone = {},
            onAddZoneClick = {},
            onViewPlansClick = {},
            onSettingsClick = {},
        )
    }
}
