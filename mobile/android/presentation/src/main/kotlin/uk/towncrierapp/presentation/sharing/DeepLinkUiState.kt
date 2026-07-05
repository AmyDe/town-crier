package uk.towncrierapp.presentation.sharing

import uk.towncrierapp.domain.auth.DomainError

/** `DeepLinkViewModel` state — a one-shot [resolution] the NavHost layer consumes via `consumeResolution()`. */
public data class DeepLinkUiState(
    val resolution: DeepLinkResolution? = null,
    val isResolving: Boolean = false,
    val error: DomainError? = null,
)
