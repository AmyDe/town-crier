package uk.towncrierapp.presentation.designsystem.components

import android.content.res.Configuration
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/**
 * Town Crier's primary call-to-action: amber fill, full-width, 40% opacity
 * when disabled. There is no FAB variant in this design system — every
 * primary action is this button (design-language skill, Buttons).
 */
@Composable
public fun PrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    Button(
        onClick = onClick,
        modifier =
            modifier
                .fillMaxWidth()
                .heightIn(min = 44.dp),
        // minimum tap target, design-language skill
        enabled = enabled,
        shape = MaterialTheme.shapes.medium,
        colors =
            ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary,
                disabledContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.4f),
                disabledContentColor = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.4f),
            ),
        contentPadding = PaddingValues(horizontal = TownCrierSpacing.md, vertical = TownCrierSpacing.sm),
    ) {
        Text(text = text, style = TownCrierTheme.bodyEmphasis)
    }
}

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun PrimaryButtonEnabledPreview() {
    TownCrierTheme {
        PrimaryButton(text = "Save watch zone", onClick = {})
    }
}

@Preview(name = "disabled")
@Composable
private fun PrimaryButtonDisabledPreview() {
    TownCrierTheme {
        PrimaryButton(text = "Save watch zone", onClick = {}, enabled = false)
    }
}
