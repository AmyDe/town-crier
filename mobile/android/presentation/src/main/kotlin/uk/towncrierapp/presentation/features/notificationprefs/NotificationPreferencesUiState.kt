package uk.towncrierapp.presentation.features.notificationprefs

import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.profile.DigestDay

/**
 * State for [NotificationPreferencesScreen]/[NotificationPreferencesViewModel].
 * [pushEnabled] is round-tripped silently on every PATCH — it has no toggle
 * on this screen (parity with iOS `NotificationPreferencesViewModel`).
 */
public data class NotificationPreferencesUiState(
    val isLoading: Boolean = true,
    val pushEnabled: Boolean = true,
    val savedDecisionPush: Boolean = true,
    val savedDecisionEmail: Boolean = true,
    val emailDigestEnabled: Boolean = true,
    val digestDay: DigestDay = DigestDay.MONDAY,
    val error: DomainError? = null,
)
