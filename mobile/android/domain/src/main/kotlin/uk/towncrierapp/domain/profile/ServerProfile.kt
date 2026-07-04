package uk.towncrierapp.domain.profile

import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/** The server-side profile returned by `POST /v1/me` (ensure-profile). */
public data class ServerProfile(
    val userId: String,
    val pushEnabled: Boolean,
    val tier: SubscriptionTier,
)
