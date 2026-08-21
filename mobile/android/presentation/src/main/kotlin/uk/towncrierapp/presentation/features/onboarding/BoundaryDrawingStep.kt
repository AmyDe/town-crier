package uk.towncrierapp.presentation.features.onboarding

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.domain.watchzones.Coordinate
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton
import uk.towncrierapp.presentation.features.watchzones.PolygonDrawingSection
import uk.towncrierapp.presentation.features.watchzones.WatchZoneShapeEvent

/**
 * Step reached only via [OnboardingViewModel.selectCustomShape] or
 * [OnboardingViewModel.reconcileTierAfterCustomShapeUpgrade] (GH#1072 Phase
 * 5, tc-v6fo0.5) - the [OnboardingStep.BoundaryDrawing] counterpart to
 * [RadiusPickerStep]. Reuses the editor's [PolygonDrawingSection] drawing
 * surface directly (it takes the vertex/closed/error fields rather than a
 * whole `WatchZoneEditorUiState`, precisely so this step can share it without
 * an adapter type). [onConfirmClick] is only enabled once the polygon is
 * closed - a custom shape has a genuine "not ready yet" state a circle
 * doesn't.
 */
@Composable
internal fun BoundaryDrawingStep(
    state: OnboardingUiState,
    onShapeEvent: (WatchZoneShapeEvent) -> Unit,
    onConfirmClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val coordinate = state.geocodedCoordinate
    Column(
        modifier = modifier.fillMaxSize().padding(TownCrierSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.md),
    ) {
        Text(
            text = stringResource(R.string.onboarding_boundary_drawing_title),
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground,
        )
        Text(
            text = stringResource(R.string.onboarding_boundary_drawing_subtitle),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (coordinate != null) {
            PolygonDrawingSection(
                vertices = state.polygonVertices,
                isPolygonClosed = state.isPolygonClosed,
                boundaryError = state.boundaryError,
                centre = coordinate,
                onShapeEvent = onShapeEvent,
                modifier = Modifier.weight(1f),
            )
        }
        PrimaryButton(
            text = stringResource(R.string.onboarding_boundary_drawing_confirm_button),
            onClick = onConfirmClick,
            enabled = state.isPolygonClosed,
        )
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun BoundaryDrawingStepPreview() {
    TownCrierTheme {
        BoundaryDrawingStep(
            state =
                OnboardingUiState(
                    geocodedCoordinate = Coordinate(51.5074, -0.1278),
                    polygonVertices =
                        listOf(
                            Coordinate(51.5074, -0.1278),
                            Coordinate(51.5090, -0.1260),
                            Coordinate(51.5074, -0.1240),
                        ),
                    isPolygonClosed = true,
                ),
            onShapeEvent = {},
            onConfirmClick = {},
        )
    }
}
