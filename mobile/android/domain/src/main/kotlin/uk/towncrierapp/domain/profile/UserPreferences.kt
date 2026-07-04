package uk.towncrierapp.domain.profile

/**
 * The full notification-preferences set sent as the `PATCH /v1/me` body.
 * The server treats these five fields as a set — every write carries all
 * five, never a partial update (epic #770 pre-resolved decision), so a
 * setter that changes one field still round-trips the other four unchanged.
 */
public data class UserPreferences(
    val pushEnabled: Boolean,
    val digestDay: DigestDay,
    val emailDigestEnabled: Boolean,
    val savedDecisionPush: Boolean,
    val savedDecisionEmail: Boolean,
)

/** The [UserPreferences] currently in effect for this profile — the starting point for a full-body PATCH that changes only one field. */
public val ServerProfile.preferences: UserPreferences
    get() = UserPreferences(pushEnabled, digestDay, emailDigestEnabled, savedDecisionPush, savedDecisionEmail)
