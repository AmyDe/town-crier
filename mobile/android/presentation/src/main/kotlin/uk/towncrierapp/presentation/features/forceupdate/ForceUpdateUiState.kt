package uk.towncrierapp.presentation.features.forceupdate

/** Pre-login force-update gate state (`GET /v1/version-config` vs the running build). */
public data class ForceUpdateUiState(
    val isChecking: Boolean = false,
    val requiresUpdate: Boolean = false,
)
