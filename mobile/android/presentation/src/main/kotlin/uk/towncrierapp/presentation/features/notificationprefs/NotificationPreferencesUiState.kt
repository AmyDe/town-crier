package uk.towncrierapp.presentation.features.notificationprefs

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.profile.DigestDay

/** [pushEnabled] has no toggle on this screen; it is round-tripped unchanged on every PATCH. */
public data class NotificationPreferencesUiState(
    val isLoading: Boolean = true,
    val pushEnabled: Boolean = true,
    val savedDecisionPush: Boolean = true,
    val savedDecisionEmail: Boolean = true,
    val emailDigestEnabled: Boolean = true,
    val digestDay: DigestDay = DigestDay.MONDAY,
    val error: DomainError? = null,
)
