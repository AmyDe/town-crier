package uk.towncrierapp.domain.applications

import java.time.OffsetDateTime

/**
 * The most recent event a user hasn't yet acknowledged for a
 * [PlanningApplication] — its presence (non-`null` on the row) is what marks
 * a row "unread" throughout `:presentation` (the client-derived unread count
 * and the unread dot both key off this, never `totalUnreadCount`). [type] is
 * kept as the raw server string (`"NewApplication"` / `"DecisionUpdate"`) —
 * deliberately not modelled as a closed set, matching iOS. Port of iOS
 * `LatestUnreadEvent` (GH#775).
 */
public data class LatestUnreadEvent(
    public val type: String,
    public val decision: String? = null,
    public val createdAt: OffsetDateTime,
)
