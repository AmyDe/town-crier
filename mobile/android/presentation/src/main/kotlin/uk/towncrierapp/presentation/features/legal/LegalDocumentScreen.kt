package uk.towncrierapp.presentation.features.legal

import android.content.res.Configuration
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme

/** Renders a single bundled legal document: title, formatted date, and section list. Port of iOS `LegalDocumentView`. */
@Composable
public fun LegalDocumentRoute(
    viewModel: LegalDocumentViewModel,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) { viewModel.load() }

    LegalDocumentScreen(state = state, onBack = onBack, modifier = modifier)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun LegalDocumentScreen(
    state: LegalDocumentUiState,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { Text(state.title) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = stringResource(R.string.legal_back_content_description),
                        )
                    }
                },
            )
        },
    ) { contentPadding ->
        if (state.isLoading) {
            Box(modifier = Modifier.padding(contentPadding).fillMaxSize(), contentAlignment = Alignment.Center) {
                CircularProgressIndicator()
            }
            return@Scaffold
        }
        Column(
            modifier =
                Modifier
                    .padding(contentPadding)
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.lg),
        ) {
            Text(
                text = state.formattedLastUpdated,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            state.sections.forEach { section ->
                Column(verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.xs)) {
                    Text(text = section.heading, style = MaterialTheme.typography.titleMedium)
                    Text(text = section.body, style = MaterialTheme.typography.bodyLarge)
                }
            }
        }
    }
}

private val previewState =
    LegalDocumentUiState(
        isLoading = false,
        title = "Privacy Policy",
        formattedLastUpdated = "1 July 2026",
        sections =
            listOf(
                LegalDocumentSectionUi("Who We Are", "Town Crier is operated by Ivo and the Bea Ltd."),
                LegalDocumentSectionUi("What We Collect", "We keep the collection small."),
            ),
    )

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun LegalDocumentScreenPreview() {
    TownCrierTheme {
        LegalDocumentScreen(state = previewState, onBack = {})
    }
}

@Preview(name = "loading")
@Composable
private fun LegalDocumentScreenLoadingPreview() {
    TownCrierTheme {
        LegalDocumentScreen(state = LegalDocumentUiState(), onBack = {})
    }
}
