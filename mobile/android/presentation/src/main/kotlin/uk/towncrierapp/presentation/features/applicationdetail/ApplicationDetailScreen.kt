package uk.towncrierapp.presentation.features.applicationdetail

import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.BookmarkBorder
import androidx.compose.material.icons.filled.Share
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import uk.towncrierapp.domain.applications.ApplicationStatus
import uk.towncrierapp.domain.applications.LocalAuthority
import uk.towncrierapp.domain.applications.PlanningApplication
import uk.towncrierapp.domain.applications.PlanningApplicationId
import uk.towncrierapp.domain.applications.StatusEvent
import uk.towncrierapp.presentation.R
import uk.towncrierapp.presentation.designsystem.DateDisplayFormatter
import uk.towncrierapp.presentation.designsystem.TownCrierSpacing
import uk.towncrierapp.presentation.designsystem.TownCrierTheme
import uk.towncrierapp.presentation.designsystem.components.PrimaryButton
import uk.towncrierapp.presentation.designsystem.components.StatusBadge
import uk.towncrierapp.presentation.designsystem.components.StatusTimeline
import uk.towncrierapp.presentation.designsystem.components.statusDisplay
import uk.towncrierapp.presentation.features.applicationlist.applicationErrorMessageRes
import java.time.LocalDate

/** The public share origin the "share" action links to — see `api-go/internal/sharepage`'s `shareOrigin` + `/a/{slug}/{ref}` route. */
internal const val SHARE_ORIGIN: String = "https://share.towncrierapp.uk"

internal fun shareUrlFor(
    authoritySlug: String,
    name: String,
): String = "$SHARE_ORIGIN/a/$authoritySlug/$name"

/**
 * A full navigation destination (Material idiom — not a bottom sheet):
 * status badge, description headline, fields card, [StatusTimeline], a
 * "View on Council Portal" Custom Tab button when present, and toolbar
 * save-toggle + share (share enabled only once the by-id refresh supplies an
 * `authoritySlug`). Port of iOS `ApplicationDetailView` (GH#775).
 */
@Composable
public fun ApplicationDetailRoute(
    viewModel: ApplicationDetailViewModel,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()
    val context = LocalContext.current

    LaunchedEffect(Unit) {
        viewModel.refresh()
        viewModel.checkSavedState()
    }

    ApplicationDetailScreen(
        state = state,
        onBack = onBack,
        onSaveToggleClick = viewModel::toggleSave,
        onPortalClick = { url -> openCouncilPortal(context, url) },
        onShareClick = { url -> shareApplication(context, url) },
        modifier = modifier,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ApplicationDetailScreen(
    state: ApplicationDetailUiState,
    onBack: () -> Unit,
    onSaveToggleClick: () -> Unit,
    onPortalClick: (String) -> Unit,
    onShareClick: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val application = state.application
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = {},
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(imageVector = Icons.AutoMirrored.Filled.ArrowBack, contentDescription = null)
                    }
                },
                actions = {
                    IconButton(onClick = onSaveToggleClick) {
                        Icon(
                            imageVector = if (state.isSaved) Icons.Filled.Bookmark else Icons.Filled.BookmarkBorder,
                            contentDescription =
                                stringResource(
                                    if (state.isSaved) {
                                        R.string.application_detail_unsave_content_description
                                    } else {
                                        R.string.application_detail_save_content_description
                                    },
                                ),
                        )
                    }
                    val authoritySlug = state.authoritySlug
                    if (authoritySlug != null) {
                        IconButton(onClick = { onShareClick(shareUrlFor(authoritySlug, application.id.name)) }) {
                            Icon(
                                imageVector = Icons.Filled.Share,
                                contentDescription =
                                    stringResource(
                                        R.string.application_detail_share_content_description,
                                    ),
                            )
                        }
                    }
                },
            )
        },
    ) { contentPadding ->
        Column(
            modifier =
                Modifier
                    .padding(contentPadding)
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState())
                    .padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.lg),
        ) {
            val display = statusDisplay(application.status)
            StatusBadge(label = display.label, color = display.color, icon = display.icon)
            Text(text = application.description, style = MaterialTheme.typography.titleLarge)
            FieldsCard(application)
            if (application.statusHistory.isNotEmpty()) {
                StatusTimeline(events = application.statusHistory)
            }
            application.portalUrl?.let { url ->
                PrimaryButton(
                    text = stringResource(R.string.application_detail_portal_button),
                    onClick = { onPortalClick(url) },
                )
            }
            state.error?.let { error ->
                Text(
                    text = stringResource(applicationErrorMessageRes(error)),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error,
                )
            }
        }
    }
}

@Composable
private fun FieldsCard(
    application: PlanningApplication,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
    ) {
        Column(
            modifier = Modifier.padding(TownCrierSpacing.md),
            verticalArrangement = Arrangement.spacedBy(TownCrierSpacing.sm),
        ) {
            FieldRow(label = stringResource(R.string.application_detail_address_label), value = application.address)
            FieldRow(label = stringResource(R.string.application_detail_reference_label), value = application.reference)
            FieldRow(
                label = stringResource(R.string.application_detail_authority_label),
                value = application.authority.name,
            )
            FieldRow(
                label = stringResource(R.string.application_detail_received_label),
                value = DateDisplayFormatter.format(application.receivedDate),
            )
        }
    }
}

@Composable
private fun FieldRow(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
) {
    Row(modifier = modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(text = value, style = MaterialTheme.typography.bodyMedium)
    }
}

private fun openCouncilPortal(
    context: Context,
    url: String,
) {
    CustomTabsIntent.Builder().build().launchUrl(context, Uri.parse(url))
}

private fun shareApplication(
    context: Context,
    url: String,
) {
    val sendIntent =
        Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_TEXT, url)
        }
    context.startActivity(Intent.createChooser(sendIntent, null))
}

// Preview-only sample data — cannot reuse :domain's testFixtures from the
// main source set (compose-ui.md).
private val previewApplication =
    PlanningApplication(
        id = PlanningApplicationId("42", "24/0001"),
        reference = "24/0001",
        authority = LocalAuthority(code = "42", name = "Camden", slug = "camden"),
        status = ApplicationStatus.Permitted,
        receivedDate = LocalDate.of(2026, 1, 15),
        description = "Two-storey rear extension with roof lantern",
        address = "1 Example Street, Camden, London",
        portalUrl = "https://planningpublicaccess.camden.gov.uk/example",
        statusHistory =
            listOf(
                StatusEvent(ApplicationStatus.Undecided, LocalDate.of(2026, 1, 15)),
                StatusEvent(ApplicationStatus.Permitted, LocalDate.of(2026, 3, 2)),
            ),
    )

@Preview(name = "light")
@Preview(name = "dark", uiMode = Configuration.UI_MODE_NIGHT_YES)
@Composable
private fun ApplicationDetailScreenPreview() {
    TownCrierTheme {
        ApplicationDetailScreen(
            state =
                ApplicationDetailUiState(
                    application = previewApplication,
                    isSaved = true,
                    authoritySlug = "camden",
                ),
            onBack = {},
            onSaveToggleClick = {},
            onPortalClick = {},
            onShareClick = {},
        )
    }
}

@Preview(name = "before by-id refresh (no slug, no share)")
@Composable
private fun ApplicationDetailScreenBeforeRefreshPreview() {
    TownCrierTheme {
        ApplicationDetailScreen(
            state = ApplicationDetailUiState(application = previewApplication.copy(statusHistory = emptyList())),
            onBack = {},
            onSaveToggleClick = {},
            onPortalClick = {},
            onShareClick = {},
        )
    }
}
