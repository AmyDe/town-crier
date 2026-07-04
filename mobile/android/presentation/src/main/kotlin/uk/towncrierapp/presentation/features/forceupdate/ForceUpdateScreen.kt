package uk.towncrierapp.presentation.features.forceupdate

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton

/** The Play Store listing to deep-link to — the real package id, never the `.dev`-suffixed one (don't repeat iOS's placeholder-store-id bug). */
public const val PLAY_STORE_URL: String = "https://play.google.com/store/apps/details?id=uk.towncrierapp.mobile"

/**
 * Full-screen, non-dismissable blocking update prompt. There is no back
 * button or skip action here by design — the app is unusable below the
 * server's minimum version.
 */
@Composable
public fun ForceUpdateScreen(
    onUpdateClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(modifier = modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier =
                Modifier
                    .fillMaxSize()
                    .padding(PaddingValues(horizontal = TownCrierSpacing.lg, vertical = TownCrierSpacing.xxl)),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = stringResource(R.string.force_update_title),
                style = MaterialTheme.typography.titleLarge,
                color = MaterialTheme.colorScheme.onBackground,
            )
            Text(
                text = stringResource(R.string.force_update_message),
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = TownCrierSpacing.sm, bottom = TownCrierSpacing.lg),
            )
            PrimaryButton(text = stringResource(R.string.force_update_button), onClick = onUpdateClick)
        }
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun ForceUpdateScreenPreview() {
    TownCrierTheme {
        ForceUpdateScreen(onUpdateClick = {})
    }
}
