package uk.towncrierapp.domain.profile

import uk.towncrierapp.domain.subscriptions.SubscriptionTier

/**
 * The server-side profile returned by `POST`/`GET`/`PATCH /v1/me`. The four
 * preference fields beyond [pushEnabled] default to the server's own
 * `DefaultPreferences` (push-on, Monday digest, every channel on) since
 * `POST /v1/me`'s response body carries only `userId`/`pushEnabled`/`tier` —
 * only `GET`/`PATCH` return the full set.
 */
public data class ServerProfile(
    val userId: String,
    val pushEnabled: Boolean,
    val tier: SubscriptionTier,
    val digestDay: DigestDay = DigestDay.MONDAY,
    val emailDigestEnabled: Boolean = true,
    val savedDecisionPush: Boolean = true,
    val savedDecisionEmail: Boolean = true,
)
